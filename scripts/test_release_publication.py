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
