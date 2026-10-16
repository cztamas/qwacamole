#!/usr/bin/env python3
"""Check Caddy's escaped-path case denial, independently of the QWAC gate."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import socket
import subprocess
import tarfile
import tempfile
import time
import urllib.error
import urllib.request


def check(binary, output):
    binary = binary.resolve()
    with tempfile.TemporaryDirectory(prefix="qwacamole-routing-") as scratch:
        scratch = Path(scratch)
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        config = scratch / "Caddyfile"
        config.write_text("{\n admin off\n auto_https off\n}\n"
                          f"http://127.0.0.1:{port} {{\n"
                          ' @deny path /admin%2Fpanel\n respond @deny "DENY" 403\n'
                          ' respond "ALLOW" 200\n}\n')
        env = dict(os.environ, XDG_CONFIG_HOME=str(scratch / "config"), XDG_DATA_HOME=str(scratch / "data"))
        version = subprocess.check_output([binary, "version"], text=True).strip()
        result = {"binary_sha256": hashlib.sha256(binary.read_bytes()).hexdigest(),
                  "caddy_version": version, "binding_gate_configured": False, "responses": {}}
        with (scratch / "caddy.log").open("w+") as log:
            process = subprocess.Popen([binary, "run", "--config", config, "--adapter", "caddyfile"],
                                       env=env, stdout=log, stderr=subprocess.STDOUT)
            try:
                def request(path):
                    try:
                        response = urllib.request.urlopen(f"http://127.0.0.1:{port}" + path, timeout=1)
                    except urllib.error.HTTPError as response:
                        return response.code, response.read().decode()
                    with response:
                        return response.status, response.read().decode()
                for attempt in range(50):
                    if process.poll() is not None:
                        log.seek(0)
                        raise RuntimeError("Caddy startup failed: " + log.read())
                    try:
                        if request("/unprotected-control") == (200, "ALLOW"):
                            break
                    except OSError:
                        pass
                    time.sleep(.1)
                else:
                    raise RuntimeError("Caddy did not become ready")
                for path in ("/admin%2Fpanel", "/ADMIN%2Fpanel"):
                    status, body = request(path)
                    result["responses"][path] = {"status": status, "body": body}
                result["passed"] = all(r == {"status": 403, "body": "DENY"} for r in result["responses"].values())
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_text(json.dumps(result, indent=2) + "\n")
                if not result["passed"]:
                    raise ValueError("escaped-path case denial failed; see " + str(output))
            finally:
                if process.poll() is None:
                    process.terminate()
                    try:
                        process.wait(timeout=10)
                    except subprocess.TimeoutExpired:
                        process.kill()
                        process.wait()
    print("PASS packaged Caddy denies both escaped-path case variants; QWAC gate not configured")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--binary", type=Path)
    source.add_argument("--directory", type=Path, help="verified candidate directory (Linux amd64 runtime)")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if args.binary:
        check(args.binary, args.output)
        return
    script = Path(__file__).with_name("release-agent.py")
    spec = importlib.util.spec_from_file_location("release_agent", script)
    packages = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(packages)
    manifest = packages.verify(args.directory)
    archive = next(a for a in manifest["archives"] if a["component"] == "caddy")
    with tempfile.TemporaryDirectory(prefix="qwacamole-routing-package-") as scratch:
        with tarfile.open(args.directory / archive["name"]) as tar:
            tar.extractall(scratch, filter="data")
        check(Path(scratch) / archive["name"][:-7] / "caddy-qwac", args.output)


if __name__ == "__main__":
    main()
