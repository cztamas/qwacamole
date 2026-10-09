import importlib.util
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


if __name__ == "__main__":
    unittest.main()
