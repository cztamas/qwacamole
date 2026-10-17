"""Security gates must fail on JSON findings even when the scanner exits zero."""
import importlib.util
import json
from pathlib import Path
import unittest

spec = importlib.util.spec_from_file_location("security", Path(__file__).with_name("check-vulnerabilities.py"))
security = importlib.util.module_from_spec(spec)
spec.loader.exec_module(security)


class SecurityFindings(unittest.TestCase):
    def stream(self, *findings):
        return "\n".join(json.dumps(message) for message in [
            {"config": {"scanner_version": "v1.9.0", "db_last_modified": "2026-10-08T22:31:09Z"}},
            {"SBOM": {"modules": [{"path": "example.test/module", "version": "v1.0.0"}]}},
            *({"finding": finding} for finding in findings)])

    def finding(self, function=None):
        frame = {"module": "example.test/module", "version": "v1.0.0"}
        if function:
            frame["function"] = function
        return {"osv": "GO-2026-0000", "trace": [frame]}

    def test_reachable_finding_cannot_be_allowed_as_unreachable(self):
        key = ("GO-2026-0000", "example.test/module", "v1.0.0")
        _, inventory, failures = security.assess(self.stream(self.finding(), self.finding("Vulnerable")),
                                                {key: {"reason": "unreachable"}})
        self.assertTrue(inventory[0]["reachable"])
        self.assertEqual(len(failures), 1)
        self.assertIn("reachable vulnerability", failures[0])

    def test_module_inventory_requires_exact_specific_disposition(self):
        stream = self.stream(self.finding())
        _, _, failures = security.assess(stream, {})
        self.assertIn("exact disposition", failures[0])
        _, inventory, failures = security.assess(stream,
            {("GO-2026-0000", "example.test/module", "v1.0.0"): {"reason": "reviewed unused feature"}})
        self.assertFalse(failures)
        self.assertFalse(inventory[0]["reachable"])
        _, _, failures = security.assess(stream,
            {("GO-2026-0000", "example.test/module", "v0.9.0"): {"reason": "different version"}})
        self.assertTrue(failures)

    def test_empty_or_incomplete_scan_is_not_clean_evidence(self):
        for stream in ("", '{"config":{"scanner_version":"v1.9.0"}}',
                       self.stream().replace("v1.9.0", "v1.8.0")):
            with self.subTest(stream=stream), self.assertRaises(ValueError):
                security.assess(stream, {})
        _, inventory, failures = security.assess(self.stream(), {})
        self.assertFalse(inventory or failures)

    def test_unused_package_disposition_fails_when_package_appears(self):
        finding = self.finding()
        finding["trace"][0]["package"] = "example.test/module/unsafe/subpackage"
        _, _, failures = security.assess(self.stream(finding),
            {("GO-2026-0000", "example.test/module", "v1.0.0"):
                {"reason": "not imported", "excluded_packages": ["example.test/module/unsafe"]}})
        self.assertIn("excluded package is now present", failures[0])
