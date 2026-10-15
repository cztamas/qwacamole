#!/usr/bin/env python3
"""Validate an approved main-source candidate; publish only on explicit --publish."""
import argparse
import hashlib
import importlib.util
import json
from pathlib import Path
import re
import subprocess
from urllib.parse import urlparse

ROOT = Path(__file__).resolve().parent.parent
REPO = "cztamas/qwacamole"
spec = importlib.util.spec_from_file_location("release_agent", ROOT / "scripts/release-agent.py")
packages = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packages)


def run(*args):
    return subprocess.check_output([str(a) for a in args], cwd=ROOT, text=True).strip()


def api(path):
    return json.loads(run("gh", "api", "repos/" + REPO + "/" + path))


def require(value, message):
    if not value:
        raise ValueError(message)


def validate_tag_map(version, source, root_source, refs):
    packages.validate_identity(version, source)
    require(packages.SHA.fullmatch(root_source), "root source must be a full SHA")
    expected = {"v" + version: root_source, "agent/v" + version: source, "caddy/v" + version: source}
    require(refs == expected, "tag namespace or source identity mismatch")


def validate_environment(environment, branches):
    require(any(rule.get("type") == "required_reviewers" and rule.get("reviewers")
                for rule in environment.get("protection_rules", [])), "publication environment requires maintainer reviewers")
    policy = environment.get("deployment_branch_policy") or {}
    require(policy.get("custom_branch_policies") is True and policy.get("protected_branches") is False,
            "publication environment must select main explicitly")
    require([(b.get("name"), b.get("type")) for b in branches.get("branch_policies", [])] == [("main", "branch")],
            "publication environment must allow only main, no tags or wildcards")


def validate_portable_checks(checks, source):
    # PR runs may checkout GitHub's synthetic merge commit, even when the run
    # metadata names the PR head. Publication needs the manual main run that
    # actually checks out the selected source.
    require(any(c.get("path") == ".github/workflows/go.yml" and
                c.get("event") == "workflow_dispatch" and c.get("head_branch") == "main" and
                c.get("head_sha") == source and c.get("conclusion") == "success"
                for c in checks), "successful manual main four-module/integration run at exact source required")


def render_notes(template, version, root_source, source, manifest_sha256, candidate_run):
    # The source cannot contain its own final commit hash. Resolve identity in
    # the release body from the verified manifest/tags, never by editing the
    # clean selected source after building or retagging it.
    return template.rstrip() + ("\n\n## Verified release identity\n\n"
        f"Version: `{version}`. Core source: `{root_source}`. Agent/Caddy source: `{source}`.\n\n"
        "GitHub's automatic source archives follow the root/core tag. Use the "
        "separate agent/Caddy module tags for their source; agent binaries use "
        "the agent/Caddy source above.\n\n"
        f"Reviewed manifest SHA256: `{manifest_sha256}`. Candidate run: "
        f"https://github.com/{REPO}/actions/runs/{candidate_run}.\n")


def gate(args):
    packages.validate_identity(args.version, args.source)
    require(run("git", "rev-parse", "HEAD") == args.source, "checkout must match selected source exactly")
    require(not run("git", "status", "--porcelain", "--untracked-files=no"), "tracked checkout is dirty")
    subprocess.check_call(["git", "merge-base", "--is-ancestor", args.source, "origin/main"], cwd=ROOT)
    policy = json.loads((ROOT / "release/publication.json").read_text())
    require(policy.get("approved") is True and policy.get("version") == args.version, "owner publication approval missing/version mismatch")
    url = urlparse(policy.get("owner_decision_url") or "")
    require(url.scheme == "https" and bool(url.netloc), "owner decision must have a real review URL")
    packages.legal_files(ROOT, approved=True)
    for module in ("agent", "caddy"):
        for name in ("LICENSE", "NOTICE"):
            require((ROOT / module / name).read_bytes() == (ROOT / name).read_bytes(), "nested module legal-file mismatch: " + module + "/" + name)
    info = json.loads(run("gh", "api", "repos/" + REPO))
    require(info.get("private") is False and info.get("default_branch") == "main", "public main repository required")
    validate_environment(api("environments/public-alpha-release"),
                         api("environments/public-alpha-release/deployment-branch-policies"))
    channel = urlparse(policy.get("confidential_reporting_url") or "")
    if channel.scheme == "https" and channel.netloc:
        require(channel.geturl() in (ROOT / "SECURITY.md").read_text(), "approved confidential channel missing from SECURITY.md")
    else:
        require(api("private-vulnerability-reporting").get("enabled") is True, "verified private vulnerability reporting or approved confidential channel required")
    refs = {}
    for tag in ("v" + args.version, "agent/v" + args.version, "caddy/v" + args.version):
        obj = api("git/ref/tags/" + tag)["object"]
        while obj["type"] == "tag":
            obj = api("git/tags/" + obj["sha"])["object"]
        require(obj["type"] == "commit", "release tag must reference a commit")
        refs[tag] = obj["sha"]
    root_source = refs["v" + args.version]
    validate_tag_map(args.version, args.source, root_source, refs)
    subprocess.check_call(["git", "merge-base", "--is-ancestor", root_source, args.source], cwd=ROOT)
    core_version = "v" + args.version
    for module in ("agent", "caddy"):
        require(re.search(r"github\.com/cztamas/qwacamole\s+" + re.escape(core_version) + r"(?:\s|$)", (ROOT / module / "go.mod").read_text()), "nested module must require published root " + core_version)
    candidate_run = api("actions/runs/" + str(args.candidate_run))
    require(candidate_run.get("event") == "workflow_dispatch" and candidate_run.get("head_branch") == "main" and candidate_run.get("head_sha") == args.source and candidate_run.get("conclusion") == "success" and candidate_run.get("path") == ".github/workflows/agent-candidate.yml", "successful manual main candidate run at exact source required")
    checks = api("actions/runs?head_sha=" + args.source + "&per_page=100")["workflow_runs"]
    validate_portable_checks(checks, args.source)
    manifest_path = Path(args.directory) / "manifest.json"
    require(hashlib.sha256(manifest_path.read_bytes()).hexdigest() == args.manifest_sha256, "owner-reviewed manifest digest mismatch")
    manifest = packages.verify(args.directory, smoke=True)
    require(manifest.get("schema")==2 and manifest["candidate"] is False and manifest["source_commit"] == args.source and manifest["version"] == args.version and manifest["core_version"] == core_version and manifest.get("core_source_commit")==root_source, "release/source/core metadata mismatch or unapproved candidate")
    # Compare downloaded artifacts with an independent build of the selected
    # approved source. A modified manifest/checksum set cannot authorize a binary.
    import tempfile
    with tempfile.TemporaryDirectory(prefix="qwacamole-publish-rebuild-") as scratch:
        output = Path(scratch) / "packages"
        subprocess.check_call(["python3", ROOT / "scripts/release-agent.py", "build", "--source", args.source,
                               "--version", args.version, "--output", output, "--go", args.go, "--approved"], cwd=ROOT)
        require((output / "SHA256SUMS").read_bytes() == (Path(args.directory) / "SHA256SUMS").read_bytes(), "candidate differs from reproducible source rebuild")
    # gh release list includes draft releases; a hidden draft must not be replaced.
    releases = json.loads(run("gh", "api", "--paginate", "--slurp", "repos/" + REPO + "/releases?per_page=100"))
    require(not any(r["tag_name"] == "v" + args.version for page in releases for r in page), "release version already exists; never overwrite")
    print("PASS approved source, tags, licensing, security, CI and exact artifact checks")
    return manifest, root_source


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("--source", required=True)
    p.add_argument("--version", required=True)
    p.add_argument("--candidate-run", required=True, type=int)
    p.add_argument("--manifest-sha256", required=True)
    p.add_argument("--directory", required=True)
    p.add_argument("--go", default="go")
    p.add_argument("--publish", action="store_true")
    args = p.parse_args()
    require(re.fullmatch(r"[0-9a-f]{64}", args.manifest_sha256), "reviewed manifest SHA256 required")
    manifest, root_source = gate(args)
    notes = render_notes((ROOT / "docs/release-notes-alpha.md").read_text(), args.version,
                         root_source, args.source, args.manifest_sha256, args.candidate_run)
    if not args.publish:
        print(notes)
    if args.publish:
        files = [str(p) for p in sorted(Path(args.directory).iterdir())]
        # --verify-tag forbids accidental tag creation. No --clobber, tag push,
        # release editing or retry on an uncertain write. Inspect before retry.
        import tempfile
        with tempfile.NamedTemporaryFile(mode="w", suffix=".md") as notes_file:
            notes_file.write(notes)
            notes_file.flush()
            subprocess.check_call(["gh", "release", "create", "v" + args.version, "--repo", REPO, "--verify-tag", "--prerelease",
                "--title", "qwacamole " + args.version, "--notes-file", notes_file.name, *files], cwd=ROOT)
        print("Published approved prerelease; core source", root_source, "agent/Caddy source", args.source)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as exc:
        raise SystemExit(str(exc))
