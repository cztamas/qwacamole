from contextlib import redirect_stdout
import hashlib
import importlib.util
import io
import json
import subprocess
import tempfile
from pathlib import Path
import unittest
from types import SimpleNamespace
from unittest.mock import patch


def load(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


publish = load("publish_alpha", "publish-alpha.py")
public = load("check_public_install", "check-public-install.py")


class Publication(unittest.TestCase):
    def test_explicit_reporting_channel_is_one_documented_destination(self):
        good = "mailto:tamas.czank@gmail.com"
        publish.validate_reporting_channel(good, f"[Security contact]({good}).")
        publish.validate_reporting_channel("https://example.test/security", "<https://example.test/security>")
        for value in ("mailto:", "mailto:a@example.test,b@example.test", "mailto:a@example.test?subject=secret",
                      "mailto:a@example.test#fragment", "mailto:a%0d%0a@example.test", "mailto:a@localhost",
                      "mailto:Name <a@example.test>", "mailto:.a@example.test", "mailto:a..b@example.test",
                      "mailto:a@example..test", "mailto:a@-example.test", "mailto:a@example.test\n",
                      "mailto://a@example.test", "http://example.test", "https://user:password@example.test"):
            with self.subTest(value=value), self.assertRaises(ValueError):
                publish.validate_reporting_channel(value, f"<{value}>")
        for text in ("No address", good + ".evil.test", "[Wrong](" + good + ".evil.test)"):
            with self.subTest(text=text), self.assertRaisesRegex(ValueError, "missing from SECURITY"):
                publish.validate_reporting_channel(good, text)

    def test_pr_merge_checks_cannot_authorize_selected_source_publication(self):
        source = "a" * 40
        good = {"path": ".github/workflows/go.yml", "event": "workflow_dispatch",
                "head_branch": "main", "head_sha": source, "conclusion": "success"}
        publish.validate_portable_checks([good], source)
        for field, value in (("event", "pull_request"), ("head_sha", "b" * 40),
                             ("head_branch", "feature"), ("conclusion", "failure"),
                             ("path", ".github/workflows/other.yml")):
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, "manual main"):
                publish.validate_portable_checks([dict(good, **{field: value})], source)

    def test_release_notes_resolve_identity_without_changing_source(self):
        template = "# Approved alpha\n\nOperator behavior and limits.\n"
        result = publish.render_notes(template, "0.1.0-alpha.1", "b" * 40, "a" * 40, "c" * 64, 123)
        self.assertIn(template.rstrip(), result)
        for value in ("b" * 40, "a" * 40, "c" * 64, "/actions/runs/123"):
            self.assertIn(value, result)
        self.assertEqual(template, "# Approved alpha\n\nOperator behavior and limits.\n")

    def test_publication_environment_cannot_be_only_a_name(self):
        environment = {"protection_rules": [{"type": "required_reviewers", "reviewers": [{"type": "User"}]}],
                       "deployment_branch_policy": {"custom_branch_policies": True, "protected_branches": False}}
        branches = {"branch_policies": [{"name": "main", "type": "branch"}]}
        publish.validate_environment(environment, branches)
        for env, br in [({}, branches), (dict(environment, protection_rules=[]), branches),
                        (environment, {"branch_policies": [{"name": "*", "type": "branch"}]}),
                        (environment, {"branch_policies": [{"name": "main", "type": "tag"}]}),
                        (dict(environment, deployment_branch_policy=None), branches)]:
            with self.subTest(environment=env, branches=br), self.assertRaises(ValueError):
                publish.validate_environment(env, br)

    def test_unapproved_policy_cannot_reach_network_or_publication(self):
        source = "a" * 40
        with patch.object(publish, "run", side_effect=[source, ""]), patch.object(publish.subprocess, "check_call"), patch.object(publish.json, "loads", return_value={"approved": False}), patch.object(publish, "api") as api:
            with self.assertRaisesRegex(ValueError, "owner publication approval missing"):
                publish.gate(SimpleNamespace(source=source, version="0.1.0-alpha.1"))
            api.assert_not_called()

    def test_wrong_source_rejected_before_network_or_publication(self):
        with patch.object(publish, "run", return_value="b" * 40), patch.object(publish, "api") as api:
            with self.assertRaisesRegex(ValueError, "checkout must match"):
                publish.gate(SimpleNamespace(source="a" * 40, version="0.1.0-alpha.1"))
            api.assert_not_called()

    def test_exact_tag_namespace_and_component_sources(self):
        version, nested, root = "0.1.0-alpha.1", "a" * 40, "b" * 40
        good = {"v" + version: root, "agent/v" + version: nested, "caddy/v" + version: nested}
        publish.validate_tag_map(version, nested, root, good)
        for bad in ({"v" + version: nested}, dict(good, **{"agent/v" + version: root}), {k.replace("caddy/", "plugin/"): v for k, v in good.items()}):
            with self.subTest(refs=bad), self.assertRaisesRegex(ValueError, "namespace or source"):
                publish.validate_tag_map(version, nested, root, bad)

    def test_anonymous_install_drops_all_inherited_credentials(self):
        with patch.dict("os.environ", {"GH_TOKEN": "synthetic", "GITHUB_TOKEN": "synthetic", "SSH_AUTH_SOCK": "synthetic", "GIT_CONFIG_COUNT": "1", "GOPRIVATE": "private.example", "GOENV": "sensitive"}):
            env = public.anonymous_environment(Path("/tmp/anonymous"), "/tools/go/bin/go")
        self.assertFalse(any(k in env for k in ("GH_TOKEN", "GITHUB_TOKEN", "SSH_AUTH_SOCK", "GIT_CONFIG_COUNT")))
        self.assertEqual(env["GOPROXY"], "https://proxy.golang.org")
        self.assertEqual(env["GIT_CONFIG_GLOBAL"], "/dev/null")
        self.assertEqual(env["GOENV"], "off")
        self.assertEqual(env["GOWORK"], "off")
        for key in ("GOPRIVATE", "GONOSUMDB", "GONOPROXY"):
            self.assertEqual(env[key], "")


class ApprovedPublication(unittest.TestCase):
    """Exercise the real gate/CLI; mock only external Git, GitHub and builds."""

    def setUp(self):
        scratch = tempfile.TemporaryDirectory()
        self.addCleanup(scratch.cleanup)
        self.root = Path(scratch.name) / "source"
        self.root.mkdir()
        self.artifacts = Path(scratch.name) / "artifacts"
        self.artifacts.mkdir()
        self.source, self.core, self.tag = "a" * 40, "b" * 40, "c" * 40
        self.version = "0.1.0-alpha.1"
        self.releases, self.failed_endpoint, self.rebuild_sums = [[]], None, None
        self.release_writes = []
        self.args = SimpleNamespace(source=self.source, version=self.version,
            candidate_run=123, directory=str(self.artifacts), go="mock-go")
        policy = {"approved": True, "version": self.version,
                  "owner_decision_url": "https://example.test/owner-decision"}
        for name in ("LICENSE", "NOTICE"):
            data = (publish.ROOT / name).read_bytes()
            (self.root / name).write_bytes(data)
            policy[name.lower() + "_sha256"] = hashlib.sha256(data).hexdigest()
            for module in ("agent", "caddy"):
                (self.root / module).mkdir(exist_ok=True)
                (self.root / module / name).write_bytes(data)
        for module in ("agent", "caddy"):
            (self.root / module / "go.mod").write_text(
                "require github.com/cztamas/qwacamole v" + self.version + "\n")
        (self.root / "release").mkdir()
        (self.root / "release/publication.json").write_text(json.dumps(policy))
        (self.root / "docs").mkdir()
        self.notes = "# Approved synthetic test release\n"
        (self.root / "docs/release-notes-alpha.md").write_text(self.notes)
        self.manifest = {"schema": 2, "core_source_commit": self.core, "caddy_version":"v2.11.7", "plugin_source_commit":self.source, "caddy_dependencies":[{"path":"github.com/caddyserver/caddy/v2","version":"v2.11.7","sum":"synthetic"},{"path":"github.com/caddyserver/certmagic","version":"v0.25.6","sum":"synthetic"}], "version": self.version,
            "source_commit": self.source, "core_version": "v" + self.version,
            "go_toolchain": "go1.27.2", "go_version": "go1.27.2",
            "go_experiment": "jsonv2", "cgo_enabled": False,
            "candidate": False, "archives": []}
        for goos, arch in publish.packages.TARGETS:
            prefix = f"qwacamole-agent-{self.version}-{goos}-{arch}"
            files = {n: ((self.root / n).read_bytes(), 0o644) for n in ("LICENSE", "NOTICE")}
            files.update({"GO-NOTICES.txt": (b"Synthetic Go notices", 0o644),
                          "INSTALL.txt": (b"Synthetic installation instructions", 0o644),
                          "agent.json.example":(b"synthetic template",0o644), "install.sh":(b"synthetic installer",0o755), "qwacamole-agent": (b"Mock built executable", 0o755)})
            data = publish.packages.archive_bytes(prefix, files)
            (self.artifacts / (prefix + ".tar.gz")).write_bytes(data)
            self.manifest["archives"].append({"name": prefix + ".tar.gz",
                "goos": goos, "goarch": arch, "size": len(data),
                "sha256": hashlib.sha256(data).hexdigest(), "files": {
                    n: {"sha256": hashlib.sha256(d).hexdigest(), "mode": oct(m)}
                    for n, (d, m) in files.items()}})
        prefix=f"qwacamole-caddy-{self.version}-linux-amd64"
        files={n:(n.encode(),0o644) for n in ("LICENSE","NOTICE","GO-NOTICES.txt","DEPENDENCY-NOTICES.txt","BUILD.txt","INSTALL.txt","Caddyfile.example")}
        files.update({"caddy-qwac":(b"synthetic executable",0o755),"install.sh":(b"synthetic installer",0o755)})
        data=publish.packages.archive_bytes(prefix,files)
        (self.artifacts/(prefix+".tar.gz")).write_bytes(data)
        self.manifest["archives"].append({"component":"caddy","name":prefix+".tar.gz","goos":"linux","goarch":"amd64","size":len(data),"sha256":hashlib.sha256(data).hexdigest(),"files":{n:{"sha256":hashlib.sha256(d).hexdigest(),"mode":oct(m)} for n,(d,m) in files.items()}})
        self.seal()
        self.responses = {
            "repos/" + publish.REPO: {"private": False, "default_branch": "main"},
            "environments/public-alpha-release": {
                "protection_rules": [{"type": "required_reviewers", "reviewers": [{"type": "User"}]}],
                "deployment_branch_policy": {"custom_branch_policies": True, "protected_branches": False}},
            "environments/public-alpha-release/deployment-branch-policies": {
                "branch_policies": [{"name": "main", "type": "branch"}]},
            "private-vulnerability-reporting": {"enabled": True},
            "git/ref/tags/v" + self.version: {"object": {"type": "tag", "sha": self.tag}},
            "git/tags/" + self.tag: {"object": {"type": "commit", "sha": self.core}},
            "git/ref/tags/agent/v" + self.version: {"object": {"type": "commit", "sha": self.source}},
            "git/ref/tags/caddy/v" + self.version: {"object": {"type": "commit", "sha": self.source}},
            "actions/runs/123": {"event": "workflow_dispatch", "head_branch": "main",
                "head_sha": self.source, "conclusion": "success", "path": ".github/workflows/agent-candidate.yml"},
            "actions/runs?head_sha=" + self.source + "&per_page=100": {"workflow_runs": [{
                "event": "workflow_dispatch", "head_branch": "main", "head_sha": self.source,
                "conclusion": "success", "path": ".github/workflows/go.yml"}]}}
        self.enterContext(patch.object(publish, "ROOT", self.root))
        self.reads = self.enterContext(patch.object(publish, "run", side_effect=self.external_read))
        self.commands = self.enterContext(patch.object(publish.subprocess, "check_call", side_effect=self.external_command))
        self.enterContext(patch.object(publish.packages, "run", side_effect=self.native_version))

    def seal(self):
        path = self.artifacts / "manifest.json"
        path.write_text(json.dumps(self.manifest))
        self.args.manifest_sha256 = hashlib.sha256(path.read_bytes()).hexdigest()
        (self.artifacts / "SHA256SUMS").write_text("".join(
            f"{hashlib.sha256(p.read_bytes()).hexdigest()}  {p.name}\n"
            for p in sorted(self.artifacts.iterdir()) if p.name != "SHA256SUMS"))

    def external_read(self, *argv):
        if argv == ("git", "rev-parse", "HEAD"):
            return self.source
        if argv == ("git", "status", "--porcelain", "--untracked-files=no"):
            return ""
        if argv[:2] == ("gh", "api"):
            path = argv[-1]
            endpoint = path.removeprefix("repos/" + publish.REPO + "/")
            if endpoint == self.failed_endpoint:
                raise subprocess.CalledProcessError(1, argv, stderr=f"Synthetic HTTP {self.api_failure_status}")
            if argv[2:4] == ("--paginate", "--slurp"):
                self.assertEqual(endpoint, "releases?per_page=100")
                return json.dumps(self.releases)
            return json.dumps(self.responses[endpoint])
        self.fail("unexpected external read: " + repr(argv))

    def external_command(self, argv, **kwargs):
        if argv[:3] == ["git", "merge-base", "--is-ancestor"]:
            self.assertIn(argv[3:], ([self.source, "origin/main"], [self.core, self.source]))
        elif argv[0] == "python3":
            self.assertEqual(Path(argv[1]), self.root / "scripts/release-agent.py")
            self.assertEqual(argv[2:5], ["build", "--source", self.source])
            self.assertIn("--approved", argv)
            self.assertEqual(argv[argv.index("--version") + 1], self.version)
            self.assertEqual(argv[argv.index("--go") + 1], self.args.go)
            output = Path(argv[argv.index("--output") + 1])
            output.mkdir()
            data = self.rebuild_sums if self.rebuild_sums is not None else (self.artifacts / "SHA256SUMS").read_bytes()
            (output / "SHA256SUMS").write_bytes(data)
        elif argv[:3] == ["gh", "release", "create"]:
            self.release_writes.append((argv, Path(argv[argv.index("--notes-file") + 1]).read_text()))
        else:
            self.fail("unexpected external command: " + repr(argv))

    def native_version(self, argv):
        if "caddy-qwac" in str(argv[0]):
            if argv[1:]==["list-modules","--packages"]:return "tls.issuance.qwac http.handlers.qwac"
            self.assertEqual(argv[1:],["qwac-version","--json"])
            identity={k:self.manifest[k] for k in ("version","source_commit","core_version","go_version","caddy_version")}
            return json.dumps(dict(identity,goos="linux",goarch="amd64"))
        self.assertEqual(argv[1:], ["version", "-json"])
        goos, arch = Path(argv[0]).parent.name.rsplit("-", 2)[-2:]
        identity = {k: self.manifest[k] for k in ("version", "source_commit", "core_version", "go_version")}
        return json.dumps(dict(identity, goos=goos, goarch=arch, modified=False))

    def invoke(self, publish_release=False, gate_only=False):
        with redirect_stdout(io.StringIO()) as output:
            if gate_only:
                result = publish.gate(self.args)
            else:
                argv = ["publish-alpha.py", "--source", self.source, "--version", self.version,
                    "--candidate-run", "123", "--manifest-sha256", self.args.manifest_sha256,
                    "--directory", str(self.artifacts), "--go", self.args.go]
                if publish_release:
                    argv.append("--publish")
                with patch("sys.argv", argv):
                    result = publish.main()
        return result, output.getvalue()

    def test_complete_approved_gate_checks_real_artifacts(self):
        (manifest, core), _ = self.invoke(gate_only=True)
        self.assertEqual(manifest, self.manifest)
        self.assertEqual(core, self.core)
        self.assertTrue(any(c.args[0][0] == "python3" for c in self.commands.call_args_list))
        self.assertEqual(self.release_writes, [])

    def test_approved_mailto_runs_complete_gate_without_claiming_github_reporting(self):
        policy_path = self.root / "release/publication.json"
        policy = json.loads(policy_path.read_text())
        channel = "mailto:tamas.czank@gmail.com"
        policy["confidential_reporting_url"] = channel
        policy_path.write_text(json.dumps(policy))
        (self.root / "SECURITY.md").write_text(f"Primary contact: [email]({channel}). Support is best-effort only.")
        self.failed_endpoint, self.api_failure_status = "private-vulnerability-reporting", 404
        self.invoke(gate_only=True)
        self.assertFalse(any(call.args[-1].endswith("/private-vulnerability-reporting")
                             for call in self.reads.call_args_list))
        self.assertEqual(self.release_writes, [])
        policy["confidential_reporting_url"] += "?cc=other@example.test"
        policy_path.write_text(json.dumps(policy))
        with self.assertRaisesRegex(ValueError, "without headers"):
            self.invoke(publish_release=True)
        self.assertEqual(self.release_writes, [])

    def test_preflight_mode_renders_identity_without_publication(self):
        _, output = self.invoke()
        for identity in (self.source, self.core, self.args.manifest_sha256, "/actions/runs/123"):
            self.assertIn(identity, output)
        self.assertEqual((self.root / "docs/release-notes-alpha.md").read_text(), self.notes)
        self.assertEqual(self.release_writes, [])

    def test_approved_publish_creates_once_with_verified_tag_and_notes(self):
        self.invoke(publish_release=True)
        self.assertEqual(len(self.release_writes), 1)
        argv, notes = self.release_writes[0]
        self.assertEqual(argv[:4], ["gh", "release", "create", "v" + self.version])
        self.assertIn("--verify-tag", argv)
        self.assertIn("--prerelease", argv)
        self.assertEqual(argv[argv.index("--repo") + 1], publish.REPO)
        self.assertNotIn("--clobber", argv)
        self.assertEqual(set(argv[-len(list(self.artifacts.iterdir())):]), {str(p) for p in self.artifacts.iterdir()})
        self.assertIn(self.args.manifest_sha256, notes)
        self.assertIn(self.core, notes)
        self.assertIn(self.source, notes)

    def test_reviewed_manifest_mismatch_prevents_publication(self):
        self.args.manifest_sha256 = "0" * 64
        with self.assertRaisesRegex(ValueError, "manifest digest mismatch"):
            self.invoke(publish_release=True)
        self.assertFalse(any(c.args[0][0] == "python3" for c in self.commands.call_args_list))
        self.assertEqual(self.release_writes, [])

    def test_corrupted_artifact_prevents_publication(self):
        archive = self.artifacts / self.manifest["archives"][0]["name"]
        archive.write_bytes(archive.read_bytes() + b"corruption")
        with self.assertRaisesRegex(ValueError, "checksum set mismatch"):
            self.invoke(publish_release=True)
        self.assertEqual(self.release_writes, [])

    def test_independent_rebuild_mismatch_prevents_publication(self):
        self.rebuild_sums = b"different rebuilt checksum set"
        with self.assertRaisesRegex(ValueError, "differs from reproducible source rebuild"):
            self.invoke(publish_release=True)
        self.assertEqual(self.release_writes, [])

    def test_existing_draft_in_later_release_page_prevents_publication(self):
        self.releases = [[{"tag_name": "v0.1.0-alpha.2"}],
                         [{"tag_name": "v" + self.version, "draft": True}]]
        with self.assertRaisesRegex(ValueError, "release version already exists"):
            self.invoke(publish_release=True)
        self.assertEqual(self.release_writes, [])

    def test_api_denial_or_outage_never_reaches_publication(self):
        endpoints = ["environments/public-alpha-release",
                     "environments/public-alpha-release/deployment-branch-policies",
                     "private-vulnerability-reporting", "git/tags/" + self.tag,
                     "actions/runs/123", "releases?per_page=100"]
        for endpoint in endpoints:
            for status in (403, 500):
                with self.subTest(endpoint=endpoint, status=status):
                    self.failed_endpoint, self.api_failure_status = endpoint, status
                    before = len(self.reads.call_args_list)
                    with self.assertRaises(subprocess.CalledProcessError):
                        self.invoke(publish_release=True)
                    calls = self.reads.call_args_list[before:]
                    self.assertEqual(sum(c.args[-1].endswith(endpoint) for c in calls), 1)
                    self.assertEqual(self.release_writes, [])


if __name__ == "__main__":
    unittest.main()
