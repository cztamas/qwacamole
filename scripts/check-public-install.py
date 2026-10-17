#!/usr/bin/env python3
"""Post-publication anonymous installs: fresh caches and credential-free environment."""
import argparse
import json
from pathlib import Path
import re
import shutil
import subprocess


def anonymous_environment(scratch, go):
    return {"PATH": str(Path(go).parent) + ":/usr/bin:/bin:/usr/sbin:/sbin", "HOME": str(scratch / "home"),
            "TMPDIR": str(scratch / "tmp"), "GOWORK": "off", "GOENV": "off", "GOTOOLCHAIN": "local",
            "GOEXPERIMENT": "jsonv2", "GOFLAGS": "", "CGO_ENABLED": "0", "GOPATH": str(scratch / "gopath"), "GOMODCACHE": str(scratch / "modules"),
            "GOCACHE": str(scratch / "cache"), "GOBIN": str(scratch / "bin"),
            "GOPROXY": "https://proxy.golang.org", "GOSUMDB": "sum.golang.org",
            "GOPRIVATE": "", "GONOPROXY": "", "GONOSUMDB": "",
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": "/dev/null", "GIT_CONFIG_SYSTEM": "/dev/null",
            "GIT_TERMINAL_PROMPT": "0", "GCM_INTERACTIVE": "Never", "LANG": "C"}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument("version", help="published Go version, e.g. v0.1.0-alpha.1")
    p.add_argument("--go", default="go")
    p.add_argument("--output", required=True, help="new directory; retain binaries/cache/evidence")
    p.add_argument("--harness", help="optional existing devtools binary for real Caddy then Nginx smoke")
    args = p.parse_args()
    if not re.fullmatch(r"v0\.1\.0-alpha\.[1-9][0-9]*", args.version):
        raise SystemExit("require plain published module version v0.1.0-alpha.N")
    go = shutil.which(args.go) if "/" not in args.go else str(Path(args.go).resolve())
    if not go:
        raise SystemExit("Go executable unavailable")
    output = Path(args.output).resolve()
    output.mkdir(mode=0o700, parents=True, exist_ok=False)
    for name in ("home", "tmp", "bin"):
        (output / name).mkdir(mode=0o700)
    env = anonymous_environment(output, go)

    def command(name, argv, cwd=output):
        with (output / (name + ".log")).open("w") as log:
            subprocess.run([str(a) for a in argv], cwd=cwd, env=env, stdout=log, stderr=subprocess.STDOUT, check=True)

    for component in ("", "/agent", "/caddy"):
        name = component.lstrip("/") or "core"
        command(name + "-resolution", [go, "list", "-m", "-json", "github.com/cztamas/qwacamole" + component + "@" + args.version])
        resolved = json.loads((output / (name + "-resolution.log")).read_text())
        if resolved["Version"] != args.version or "Replace" in resolved:
            raise SystemExit("unexpected module identity/replacement")
    command("agent-install", [go, "install", "github.com/cztamas/qwacamole/agent/cmd/qwacamole-agent@" + args.version])
    command("agent-version", [output / "bin/qwacamole-agent", "version", "-json"])
    agent = json.loads((output / "agent-version.log").read_text())
    if agent["version"] != args.version or agent["core_version"] != args.version:
        raise SystemExit("anonymous agent identity mismatch")
    command("xcaddy-install", [go, "install", "github.com/caddyserver/xcaddy/cmd/xcaddy@v0.4.5"])
    command("caddy-install", [output / "bin/xcaddy", "build", "v2.11.7", "--with", "github.com/cztamas/qwacamole/caddy@" + args.version, "--output", output / "bin/caddy"])
    command("caddy-modules", [output / "bin/caddy", "list-modules"])
    for name in ("qwacamole-agent", "caddy"):
        command(name + "-metadata", [go, "version", "-m", output / "bin" / name])
        text = (output / (name + "-metadata.log")).read_text()
        if "=>" in text or "qwacamole/devtools" in text:
            raise SystemExit("anonymous install boundary/replacement violation")
        for module in ("github.com/cztamas/qwacamole", "github.com/cztamas/qwacamole/" + ("agent" if name == "qwacamole-agent" else "caddy")):
            if not re.search(r"\t(?:mod|dep)\t" + re.escape(module) + r"\t" + re.escape(args.version) + r"\t", text):
                raise SystemExit("anonymous installed metadata version mismatch")
    modules = (output / "caddy-modules.log").read_text().splitlines()
    if not all(name in modules for name in ("tls.issuance.qwac", "http.handlers.qwac")) or any("qwac_demo" in m for m in modules):
        raise SystemExit("anonymous plugin module set mismatch")
    print("PASS anonymous public module/binary installation; evidence", output)
    if args.harness:
        harness = Path(args.harness).resolve()
        # The harness's development environment is isolated from the anonymous
        # installers. It receives only already-verified absolute binary paths.
        for name, mode, flag, binary in (("caddy-smoke", "plugin-smoke", "-plugin-binary", "caddy"),
                                         ("agent-smoke", "nginx-smoke", "-agent-binary", "qwacamole-agent")):
            with (output / (name + ".log")).open("w") as log:
                subprocess.run([harness, mode, "-state", str(output / name), flag, str(output / "bin" / binary)],
                               stdout=log, stderr=subprocess.STDOUT, check=True)
        print("PASS real smoke with anonymously installed binaries")


if __name__ == "__main__":
    main()
