#!/usr/bin/env python3
"""Independent source/command and extracted production-binary security checks.

JSON-mode govulncheck does not fail on findings. Inspect its structured results,
reject every reachable finding and require exact dispositions for module-only
findings. Never allow a disposition to suppress a reachable symbol.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
SCANNER_VERSION = "v1.9.0"


def messages(text):
    decoder = json.JSONDecoder()
    while text.strip():
        value, end = decoder.raw_decode(text.lstrip())
        yield value
        text = text.lstrip()[end:]


def findings(text):
    config, inventory, reachable = None, {}, set()
    saw_sbom = False
    for message in messages(text):
        if "config" in message:
            config = message["config"]
        if "SBOM" in message:
            saw_sbom = bool(message["SBOM"].get("modules"))
        if "finding" not in message:
            continue
        finding = message["finding"]
        frame = finding["trace"][0]
        key = (finding["osv"], frame["module"], frame.get("version", ""))
        record = inventory.setdefault(key, {"id": key[0], "module": key[1], "version": key[2],
                          "fixed_version": finding.get("fixed_version"), "affected_packages": []})
        if frame.get("package") and frame["package"] not in record["affected_packages"]:
            record["affected_packages"].append(frame["package"])
        if frame.get("function"):
            reachable.add(key)
    if not config or config.get("scanner_version") != SCANNER_VERSION or not config.get("db_last_modified") or not saw_sbom:
        raise ValueError("missing pinned scanner/database metadata")
    return config, inventory, reachable


def assess(text, dispositions):
    config, inventory, reachable = findings(text)
    failures = []
    for key, finding in inventory.items():
        finding["reachable"] = key in reachable
        if key in reachable:
            failures.append("reachable vulnerability: " + " ".join(key))
        elif key not in dispositions:
            failures.append("module finding needs an exact disposition: " + " ".join(key))
        elif any(package == prefix or package.startswith(prefix + "/")
                 for package in finding["affected_packages"]
                 for prefix in dispositions[key].get("excluded_packages", [])):
            failures.append("reviewed excluded package is now present: " + " ".join(key))
        else:
            finding["disposition"] = dispositions[key]
    return config, list(inventory.values()), failures


def load_dispositions(file):
    entries = json.loads(file.read_text())["unreachable"]
    result = {}
    for entry in entries:
        key = (entry["id"], entry["module"], entry["version"])
        if key in result or not entry.get("reason") or not entry.get("evidence") or not all(
                url.startswith("https://") for url in entry["evidence"]):
            raise ValueError("dispositions require unique exact identity, reason and evidence")
        result[key] = {k: entry[k] for k in ("reason", "evidence", "excluded_packages") if k in entry}
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", default=str(ROOT / ".tools/go/bin/go"))
    parser.add_argument("--scanner", default=str(ROOT / ".tools/bin/govulncheck"))
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--candidate", type=Path)
    parser.add_argument("--dispositions", type=Path, default=ROOT / "release/vulnerability-dispositions.json")
    args = parser.parse_args()
    go = Path(shutil.which(args.go) or args.go).resolve()
    scanner = Path(shutil.which(args.scanner) or args.scanner).resolve()
    version = subprocess.check_output([scanner, "-version"], text=True)
    if "Scanner: govulncheck@" + SCANNER_VERSION not in version.splitlines():
        raise ValueError("require govulncheck@" + SCANNER_VERSION)
    dispositions = load_dispositions(args.dispositions)
    args.output.mkdir(parents=True, exist_ok=False)
    env = dict(os.environ, GOWORK="off", GOEXPERIMENT="jsonv2", GOTOOLCHAIN="local",
               PATH=str(go.parent) + os.pathsep + os.environ.get("PATH", ""))
    env["GOPRIVATE"] = "github.com/cztamas/qwacamole" + ("," + env["GOPRIVATE"] if env.get("GOPRIVATE") else "")
    with urllib.request.urlopen(urllib.request.Request("https://vuln.go.dev/index/db.json",
            headers={"Cache-Control": "no-cache"}), timeout=30) as response:
        database = response.read()
    (args.output / "database.json").write_bytes(database)
    report = {"scanner_version": SCANNER_VERSION, "database": "https://vuln.go.dev",
              "database_retrieved_at": datetime.now(timezone.utc).isoformat(),
              "database_metadata_sha256": hashlib.sha256(database).hexdigest(), "scans": []}
    failures = []

    def scan(name, directory, arguments):
        started = datetime.now(timezone.utc).isoformat()
        process = subprocess.run([scanner, "-json", *arguments], cwd=directory, env=env,
                                 text=True, capture_output=True)
        (args.output / (name + ".json")).write_text(process.stdout)
        (args.output / (name + ".stderr")).write_text(process.stderr)
        if process.returncode:
            raise RuntimeError(f"{name}: scanner execution failed ({process.returncode}); see saved stderr")
        config, inventory, errors = assess(process.stdout, dispositions)
        if "-mode=binary" not in arguments:
            imports = subprocess.check_output([go, "list", "-deps", "-f", "{{.ImportPath}}", *arguments],
                                              cwd=directory, env=env, text=True)
            (args.output / (name + ".packages.txt")).write_text(imports)
            for finding in inventory:
                key = (finding["id"], finding["module"], finding["version"])
                for prefix in dispositions.get(key, {}).get("excluded_packages", []):
                    if any(package == prefix or package.startswith(prefix + "/") for package in imports.splitlines()):
                        errors.append("excluded package is imported: " + prefix)
        report["scans"].append({"name": name, "started_at": started,
                               "config": config, "findings": inventory, "passed": not errors})
        failures.extend(name + ": " + error for error in errors)
        print(("FAIL" if errors else "PASS"), name, len(inventory), "module advisories", flush=True)

    for directory, name in ((ROOT, "core"), (ROOT / "agent", "agent"),
                            (ROOT / "caddy", "caddy"), (ROOT / "devtools", "devtools")):
        scan(name, directory, ["./..."])
    for module, command in (("agent", "qwacamole-agent"), ("caddy", "caddy-qwac")):
        scan(module + "-command", ROOT / module, ["./cmd/" + command])
    if args.candidate:
        spec = importlib.util.spec_from_file_location("release_agent", ROOT / "scripts/release-agent.py")
        packages = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(packages)
        manifest = packages.verify(args.candidate)
        report["candidate_source"] = manifest["source_commit"]
        for archive in manifest["archives"]:
            component = archive["component"]
            binary = "caddy-qwac" if component == "caddy" else "qwacamole-agent"
            with tempfile.TemporaryDirectory(prefix="qwacamole-vuln-") as scratch:
                with tarfile.open(args.candidate / archive["name"]) as tar:
                    tar.extractall(scratch, filter="data")
                executable = Path(scratch) / archive["name"][:-7] / binary
                scan(f"binary-{component}-{archive['goos']}-{archive['goarch']}", ROOT,
                     ["-mode=binary", str(executable)])
    report["passed"] = not failures
    (args.output / "summary.json").write_text(json.dumps(report, indent=2) + "\n")
    if failures:
        raise SystemExit("\n".join(failures))
    print("PASS independent source and production commands" + (" and all extracted release binaries" if args.candidate else ""))


if __name__ == "__main__":
    main()
