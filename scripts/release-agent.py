#!/usr/bin/env python3
"""Deterministic agent candidates and strict archive verification (stdlib only)."""
import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import platform
import re
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parent.parent
TARGETS = (("linux", "amd64"), ("linux", "arm64"), ("darwin", "arm64"))
VERSION = re.compile(r"0\.1\.0-alpha\.[1-9][0-9]*\Z")
SHA = re.compile(r"[0-9a-f]{40}\Z")


def run(args, **kwargs):
    return subprocess.check_output([str(a) for a in args], text=True, **kwargs).strip()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def validate_identity(version, source):
    if not VERSION.fullmatch(version) or not SHA.fullmatch(source):
        raise ValueError("require version 0.1.0-alpha.N and a full lowercase source SHA")


def legal_files(source, approved):
    if not approved:
        return {"LICENSE-PROPOSAL.txt": (source / "release/LICENSE-PROPOSAL.txt").read_bytes(),
                "NOTICE-PROPOSAL.txt": (source / "release/NOTICE-PROPOSAL.txt").read_bytes()}
    decision = json.loads((source / "release/publication.json").read_text())
    files = {name: (source / name).read_bytes() for name in ("LICENSE", "NOTICE")}
    if decision.get("approved") is not True or not str(decision.get("owner_decision_url") or "").startswith("https://"):
        raise ValueError("owner licensing/publication approval missing")
    for name, data in files.items():
        if digest(data) != decision.get(name.lower() + "_sha256"):
            raise ValueError("approved legal-file hash mismatch: " + name)
    return files


def archive_bytes(prefix, files):
    raw = io.BytesIO()
    with gzip.GzipFile(fileobj=raw, mode="wb", filename="", mtime=0) as gz:
        with tarfile.open(fileobj=gz, mode="w", format=tarfile.USTAR_FORMAT) as tar:
            for name in sorted(files):
                data, mode = files[name]
                item = tarfile.TarInfo(prefix + "/" + name)
                item.size, item.mode, item.mtime = len(data), mode, 0
                item.uid = item.gid = 0
                item.uname = item.gname = ""
                tar.addfile(item, io.BytesIO(data))
    return raw.getvalue()


def build(args):
    validate_identity(args.version, args.source)
    if run(["git", "rev-parse", args.source + "^{commit}"], cwd=ROOT) != args.source:
        raise ValueError("source does not resolve to the exact commit")
    # Source comes from Git, never untracked files, profiles, credentials or a
    # dirty working tree. The source commit must already contain this tooling.
    with tempfile.TemporaryDirectory(prefix="qwacamole-package-") as scratch:
        source = Path(scratch) / "source"
        source.mkdir()
        packed = subprocess.check_output(["git", "archive", args.source], cwd=ROOT)
        with tarfile.open(fileobj=io.BytesIO(packed)) as tar:
            # git archive yields repository-controlled paths; filter rejects any
            # unsafe link if a future source tree accidentally introduces one.
            tar.extractall(source, filter="data")
        legal = legal_files(source, args.approved)
        if args.approved and json.loads((source / "release/publication.json").read_text()).get("version") != args.version:
            raise ValueError("approved publication version mismatch")
        go = Path(args.go).resolve() if "/" in args.go else args.go
        env = dict(os.environ, GOWORK="off", CGO_ENABLED="0", GOEXPERIMENT="nojsonv2",
                   GOENV="off", GOFLAGS="", GOTOOLCHAIN="local", GOAMD64="v1", GOARM64="v8.0")
        toolchain = run([go, "env", "GOVERSION"], env=env)
        goroot = Path(run([go, "env", "GOROOT"], env=env))
        go_notices = []
        for path in [goroot / "LICENSE", goroot / "PATENTS", *sorted((goroot / "src/vendor").rglob("LICENSE"))]:
            go_notices.append("--- " + path.relative_to(goroot).as_posix() + " ---\n" + path.read_text())
        core = run([go, "list", "-m", "-f", "{{.Version}}", "github.com/cztamas/qwacamole"], cwd=source / "agent", env=env)
        if not core or core == "(devel)":
            raise ValueError("agent requires a real root module version")
        manifest = {"schema": 1, "version": args.version, "source_commit": args.source,
                    "core_version": core, "go_toolchain": toolchain, "go_version": None, "go_experiment": "nojsonv2",
                    "cgo_enabled": False, "candidate": not args.approved, "archives": []}
        output = Path(args.output).resolve()
        output.mkdir(parents=True, exist_ok=False)
        for goos, goarch in TARGETS:
            binary = Path(scratch) / (goos + "-" + goarch)
            target_env = dict(env, GOOS=goos, GOARCH=goarch)
            run([go, "build", "-trimpath", "-buildvcs=false", "-ldflags",
                 "-buildid= -X main.releaseVersion=" + args.version + " -X main.releaseSource=" + args.source,
                 "-o", binary, "./cmd/qwacamole-agent"], cwd=source / "agent", env=target_env)
            metadata = run([go, "version", "-m", binary], env=env)
            runtime_version = metadata.splitlines()[0].split(": ", 1)[1]
            if manifest["go_version"] not in (None, runtime_version):
                raise ValueError("target build toolchains disagree")
            manifest["go_version"] = runtime_version
            if "=>" in metadata or any(s in metadata for s in ("qwacamole/caddy", "qwacamole/devtools", "caddyserver", "chromedp", "k8s.io")):
                raise ValueError("production package dependency boundary violated")
            prefix = f"qwacamole-agent-{args.version}-{goos}-{goarch}"
            install = (f"qwacamole-agent {args.version}\nSource: {args.source}\nTarget: {goos}/{goarch}\n"
                       + ("PRIVATE REVIEW CANDIDATE: licensing/publication pending.\n" if not args.approved else "Owner-approved release artifact.\n")
                       + "Verify SHA256SUMS and manifest.json before installing.\n"
                       "Install only the matching target executable into your managed PATH; run\n"
                       "qwacamole-agent version -json before configuring credentials or deployment.\n"
                       "Nginx and Certbot are separately installed external dependencies. Caddy is\n"
                       "a separate xcaddy module distribution. See docs/agent-nginx.md in the exact\n"
                       "source commit for permissions, setup, deploy hooks and update/rollback.\n"
                       "Stop and preserve complete state before an incompatible migration.\n")
            files = {name: (data, 0o644) for name, data in legal.items()}
            files.update({"qwacamole-agent": (binary.read_bytes(), 0o755),
                          "GO-NOTICES.txt": ("\n\n".join(go_notices).encode(), 0o644),
                          "INSTALL.txt": (install.encode(), 0o644)})
            data = archive_bytes(prefix, files)
            name = prefix + ".tar.gz"
            (output / name).write_bytes(data)
            manifest["archives"].append({"name": name, "goos": goos, "goarch": goarch,
                "sha256": digest(data), "size": len(data), "files": {
                    n: {"sha256": digest(d), "mode": oct(m)} for n, (d, m) in sorted(files.items())}})
        (output / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
        sums = "".join(f"{digest(p.read_bytes())}  {p.name}\n" for p in sorted(output.iterdir()))
        (output / "SHA256SUMS").write_text(sums)
        verify(output, smoke=True)


def verify(output, smoke=False, require_native=False):
    output = Path(output)
    manifest = json.loads((output / "manifest.json").read_text())
    validate_identity(manifest["version"], manifest["source_commit"])
    if manifest["schema"] != 1 or type(manifest["candidate"]) is not bool:
        raise ValueError("invalid manifest schema or candidate marker")
    if manifest["go_experiment"] != "nojsonv2" or manifest["cgo_enabled"] is not False or manifest["go_toolchain"] != "go1.27.1" or manifest["go_version"] != "go1.27.1-X:nojsonv2":
        raise ValueError("unexpected build toolchain/settings")
    names = [a["name"] for a in manifest["archives"]]
    if len(names) != 3 or len(set(names)) != 3 or {(a["goos"], a["goarch"]) for a in manifest["archives"]} != set(TARGETS):
        raise ValueError("exactly the three supported targets required")
    safe_names = {f"qwacamole-agent-{manifest['version']}-{osname}-{arch}.tar.gz" for osname, arch in TARGETS}
    if set(names) != safe_names:
        raise ValueError("archive version/target name mismatch")
    expected_names = safe_names | {"manifest.json", "SHA256SUMS"}
    if {p.name for p in output.iterdir()} != expected_names or any(p.is_symlink() or not p.is_file() for p in output.iterdir()):
        raise ValueError("unexpected artifact, directory or symlink")
    expected_sums = "".join(f"{digest((output / n).read_bytes())}  {n}\n" for n in sorted(expected_names - {"SHA256SUMS"}))
    if (output / "SHA256SUMS").read_text() != expected_sums:
        raise ValueError("checksum set mismatch")
    native = ("darwin" if platform.system() == "Darwin" else "linux", {"aarch64": "arm64", "arm64": "arm64", "x86_64": "amd64"}.get(platform.machine()))
    legal = {"LICENSE-PROPOSAL.txt", "NOTICE-PROPOSAL.txt"} if manifest["candidate"] else {"LICENSE", "NOTICE"}
    expected_files = legal | {"qwacamole-agent", "GO-NOTICES.txt", "INSTALL.txt"}
    ran_native = False
    for a in manifest["archives"]:
        prefix = f"qwacamole-agent-{manifest['version']}-{a['goos']}-{a['goarch']}"
        if a["name"] != prefix + ".tar.gz":
            raise ValueError("archive version/target name mismatch")
        data = (output / a["name"]).read_bytes()
        if digest(data) != a["sha256"] or len(data) != a["size"]:
            raise ValueError("archive hash/size mismatch")
        if set(a["files"]) != expected_files:
            raise ValueError("unexpected manifest contents")
        with tarfile.open(fileobj=io.BytesIO(data)) as tar:
            members = tar.getmembers()
            if len(members) != len(expected_files) or {m.name for m in members} != {prefix + "/" + n for n in expected_files}:
                raise ValueError("unexpected archive member")
            for m in members:
                name = m.name[len(prefix) + 1:]
                mode = 0o755 if name == "qwacamole-agent" else 0o644
                if not m.isfile() or m.mode != mode or m.uid or m.gid or m.mtime or m.uname or m.gname:
                    raise ValueError("unsafe member type, permissions or metadata")
                content = tar.extractfile(m).read()
                if digest(content) != a["files"][name]["sha256"] or oct(mode) != a["files"][name]["mode"]:
                    raise ValueError("member hash/mode mismatch")
            if smoke and (a["goos"], a["goarch"]) == native:
                with tempfile.TemporaryDirectory(prefix="qwacamole-extracted-") as scratch:
                    tar.extractall(scratch, filter="data")
                    actual = json.loads(run([Path(scratch) / prefix / "qwacamole-agent", "version", "-json"]))
                    expected = {k: manifest[k] for k in ("version", "source_commit", "core_version", "go_version")}
                    expected.update(goos=a["goos"], goarch=a["goarch"], modified=False)
                    if actual != expected:
                        raise ValueError("extracted binary identity disagrees with manifest")
                    ran_native = True
                    print("PASS extracted native package version:", json.dumps(actual, sort_keys=True))
    if require_native and not ran_native:
        raise ValueError("no matching target was run; native package verification required")
    print("PASS package contents, modes, hashes and source/target identity")
    return manifest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    b = sub.add_parser("build")
    b.add_argument("--source", required=True)
    b.add_argument("--version", required=True)
    b.add_argument("--output", required=True)
    b.add_argument("--go", default="go")
    b.add_argument("--approved", action="store_true", help="require approved root LICENSE/NOTICE hashes")
    v = sub.add_parser("verify")
    v.add_argument("directory")
    v.add_argument("--smoke", action="store_true", help="run version from the extracted native package")
    v.add_argument("--require-native", action="store_true", help="fail if no extracted native target ran")
    args = parser.parse_args()
    if args.command == "build":
        build(args)
    else:
        verify(args.directory, args.smoke, args.require_native)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError, tarfile.TarError) as exc:
        raise SystemExit(str(exc))
