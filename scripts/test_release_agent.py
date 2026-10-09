"""Behavioral checks for deterministic packaging and rejection of unsafe input."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("release_agent", Path(__file__).with_name("release-agent.py"))
r = importlib.util.module_from_spec(spec)
spec.loader.exec_module(r)


class Packages(unittest.TestCase):
    def setUp(self):
        self.scratch = tempfile.TemporaryDirectory()
        self.addCleanup(self.scratch.cleanup)
        self.directory = Path(self.scratch.name)
        self.manifest = {"schema": 1, "version": "0.1.0-alpha.1", "source_commit": "a" * 40,
                         "core_version": "v0.0.0-20261008203612-8e62f4b8c340", "go_toolchain": "go1.27.1", "go_version": "go1.27.1-X:nojsonv2",
                         "go_experiment": "nojsonv2", "cgo_enabled": False, "candidate": True, "archives": []}
        for goos, arch in r.TARGETS:
            prefix = f"qwacamole-agent-0.1.0-alpha.1-{goos}-{arch}"
            files = {n: (n.encode(), 0o644) for n in ("LICENSE-PROPOSAL.txt", "NOTICE-PROPOSAL.txt", "GO-NOTICES.txt", "INSTALL.txt")}
            files["qwacamole-agent"] = (b"fake non-executable test binary", 0o755)
            data = r.archive_bytes(prefix, files)
            (self.directory / (prefix + ".tar.gz")).write_bytes(data)
            self.manifest["archives"].append({"name": prefix + ".tar.gz", "goos": goos, "goarch": arch,
                "sha256": r.digest(data), "size": len(data), "files": {n: {"sha256": r.digest(d), "mode": oct(m)} for n, (d, m) in files.items()}})
        self.seal()

    def seal(self):
        (self.directory / "manifest.json").write_text(json.dumps(self.manifest))
        (self.directory / "SHA256SUMS").write_text("".join(f"{r.digest(p.read_bytes())}  {p.name}\n" for p in sorted(self.directory.iterdir()) if p.name != "SHA256SUMS"))

    def test_valid_exact_contents_and_repeatable_archives(self):
        r.verify(self.directory)
        files = {"z": (b"z", 0o644), "a": (b"a", 0o755)}
        self.assertEqual(r.archive_bytes("p", files), r.archive_bytes("p", dict(reversed(list(files.items())))))
        self.assertEqual(r.archive_bytes("p", files)[4:8], b"\0" * 4)

    def test_invalid_version_and_source(self):
        for version, source in [("v0.1.0-alpha.1", "a" * 40), ("0.1.0", "a" * 40), ("0.1.0-alpha.1", "main"), ("0.1.0-alpha.0", "a" * 40)]:
            with self.subTest(version=version, source=source), self.assertRaises(ValueError):
                r.validate_identity(version, source)

    def test_required_native_run_cannot_be_skipped(self):
        with self.assertRaisesRegex(ValueError, "no matching target"):
            r.verify(self.directory, smoke=False, require_native=True)

    def test_corruption_cannot_pass_outer_checksums(self):
        archive = self.directory / self.manifest["archives"][0]["name"]
        archive.write_bytes(archive.read_bytes() + b"corruption")
        self.seal()
        with self.assertRaisesRegex(ValueError, "hash/size"):
            r.verify(self.directory)

    def test_secret_or_extra_artifact_rejected(self):
        (self.directory / "signing-key.pem").write_text("synthetic marker")
        self.seal()
        with self.assertRaisesRegex(ValueError, "unexpected artifact"):
            r.verify(self.directory)

    def test_unsafe_member_even_with_matching_hash_rejected(self):
        a = self.manifest["archives"][0]
        prefix = a["name"][:-7]
        data = r.archive_bytes(prefix, {"../secret.pem": (b"not a real secret", 0o600)})
        (self.directory / a["name"]).write_bytes(data)
        a["sha256"], a["size"] = r.digest(data), len(data)
        self.seal()
        with self.assertRaisesRegex(ValueError, "unexpected archive member"):
            r.verify(self.directory)

    def test_symlink_rejected(self):
        a = self.directory / self.manifest["archives"][0]["name"]
        other = self.directory / "elsewhere"
        a.rename(other)
        a.symlink_to(other)
        with self.assertRaisesRegex(ValueError, "unexpected artifact"):
            r.verify(self.directory)

    def test_name_cannot_escape_artifact_directory(self):
        self.manifest["archives"][0]["name"] = "../../secret.tar.gz"
        self.seal()
        with self.assertRaisesRegex(ValueError, "name mismatch"):
            r.verify(self.directory)

    def test_unapproved_files_cannot_become_release_legal_files(self):
        (self.directory / "publication.json").write_text('{"approved": false}')
        release = self.directory / "release"
        release.mkdir()
        (release / "publication.json").write_text('{"approved": false}')
        for name in ("LICENSE", "NOTICE"):
            (self.directory / name).write_text("draft")
        with self.assertRaisesRegex(ValueError, "approval missing"):
            r.legal_files(self.directory, approved=True)


if __name__ == "__main__":
    unittest.main()
