#!/usr/bin/env python3
"""Tests for scripts/release-preflight.py: the classifiers that turn gh and uv output into findings."""

import importlib.util
import unittest
from pathlib import Path

spec = importlib.util.spec_from_file_location("preflight", Path(__file__).with_name("release-preflight.py"))
assert spec and spec.loader
preflight = importlib.util.module_from_spec(spec)
spec.loader.exec_module(preflight)

SHA = "9a8be0096deadbeef"


def alert(name, manifest, severity, fixed, eco="pip", ghsa="GHSA-x"):
    return {
        "dependency": {"package": {"ecosystem": eco, "name": name}, "manifest_path": manifest},
        "security_advisory": {"severity": severity, "ghsa_id": ghsa},
        "security_vulnerability": {"first_patched_version": {"identifier": fixed} if fixed else None},
    }


class CI(unittest.TestCase):
    def test_no_runs_yet_blocks(self):
        [f] = preflight.ci_findings(SHA, [])
        self.assertTrue(f.block)

    def test_pending_and_failed_runs_block_green_pass(self):
        runs = [
            {"name": "Guards", "status": "completed", "conclusion": "success"},
            {"name": "Docker Images", "status": "in_progress", "conclusion": None},
            {"name": "App", "status": "completed", "conclusion": "failure"},
            {"name": "Helm", "status": "completed", "conclusion": "skipped"},
        ]
        texts = [f.text for f in preflight.ci_findings(SHA, runs)]
        self.assertEqual(
            texts,
            [
                "CI run 'Docker Images' on 9a8be0096 is in_progress: wait for it",
                "CI run 'App' on 9a8be0096 ended failure",
            ],
        )


class Alerts(unittest.TestCase):
    def test_one_blocking_finding_per_alert(self):
        alerts = [
            alert("pypdf", "docreader/uv.lock", "medium", "6.20.1", ghsa="GHSA-qfv9-64jg-mh7p"),
            alert("lodash", "frontend/package-lock.json", "high", None, eco="npm", ghsa="GHSA-l"),
        ]
        findings = preflight.alert_findings(alerts)
        self.assertEqual(
            [f.text for f in findings],
            [
                "Dependabot: pip pypdf in docreader/uv.lock (medium, GHSA-qfv9-64jg-mh7p), fixed in 6.20.1",
                "Dependabot: npm lodash in frontend/package-lock.json (high, GHSA-l), fixed in no fix published",
            ],
        )
        self.assertTrue(all(f.block for f in findings))

    def test_no_alerts_no_findings(self):
        self.assertEqual(preflight.alert_findings([]), [])


class Uv(unittest.TestCase):
    def test_dry_run_changes_are_notes(self):
        out = "Resolved 67 packages in 280ms\nUpdate pypdf v6.19.0 -> v6.21.1\nAdd foo v1.0.0\n"
        findings = preflight.uv_findings("docreader", 0, out)
        self.assertEqual(
            [(f.block, f.text) for f in findings],
            [
                (False, "docreader: uv would update pypdf v6.19.0 -> v6.21.1"),
                (False, "docreader: uv would add foo v1.0.0"),
            ],
        )

    def test_resolved_only_is_clean(self):
        self.assertEqual(preflight.uv_findings("dataset", 0, "Resolved 12 packages in 3ms\n"), [])

    def test_failed_dry_run_is_a_note_not_silence(self):
        [f] = preflight.uv_findings("dataset", 2, "error: Failed to fetch: `https://pypi.org/simple/x/`\n")
        self.assertEqual((f.block, f.text), (False, "dataset: uv dry run failed, upgrades unknown: error: Failed to fetch: `https://pypi.org/simple/x/`"))

    def test_change_without_package_lines_is_a_note(self):
        [f] = preflight.uv_findings("dataset", 0, "Resolved 23 packages\nLockfile changes detected\n")
        self.assertEqual(f.text, "dataset: uv would change the lockfile")


if __name__ == "__main__":
    unittest.main()
