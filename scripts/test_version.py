#!/usr/bin/env python3
"""Tests for scripts/version.py. Each test bumps a throwaway clone of HEAD, never this tree.

python3 scripts/test_version.py
"""

import datetime
import json
import re
import subprocess
import tempfile
import unittest
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "version.py"
ROOT = SCRIPT.parents[1]


def sh(*args, cwd):
    return subprocess.run(args, cwd=cwd, capture_output=True, text=True, check=True).stdout


class VersionTest(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.repo = Path(tmp.name) / "repo"
        sh("git", "clone", "-q", str(ROOT), str(self.repo), cwd=tmp.name)
        # A throwaway SSH key, so the signed release commit works without the operator's signer.
        key = Path(tmp.name) / "key"
        sh("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key), cwd=tmp.name)
        for k, v in [
            ("user.name", "Test"),
            ("user.email", "test@example.com"),
            ("gpg.format", "ssh"),
            ("gpg.ssh.program", "ssh-keygen"),
            ("user.signingkey", str(key)),
            ("core.hooksPath", "/dev/null"),
        ]:
            sh("git", "config", k, v, cwd=self.repo)

    def run_tool(self, *args):
        return subprocess.run(
            ["python3", str(SCRIPT), *args], cwd=self.repo, capture_output=True, text=True, check=False
        )

    def text(self, path):
        return (self.repo / path).read_text()

    def lock_versions(self, d):
        lock = json.loads(self.text(f"{d}/package-lock.json"))
        return [
            json.loads(self.text(f"{d}/package.json"))["version"],
            lock["version"],
            lock["packages"][""]["version"],
        ]

    def head_subject(self):
        return sh("git", "log", "-1", "--format=%s", cwd=self.repo).strip()

    def test_bump_rewrites_every_product_touchpoint(self):
        bad = self.run_tool("bump", "2099.01.02")
        self.assertEqual(bad.returncode, 2)
        self.assertIn("'2099.01.02' is not YYYY.M.D[.MICRO]", bad.stderr)
        self.assertEqual(self.run_tool("bump", "2020.1.1").returncode, 2)

        r = self.run_tool("bump", "2099.1.2.3")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn("git tag -s v2099.1.2.3 -m v2099.1.2.3", r.stdout)
        self.assertEqual(self.text("VERSION"), "2099.1.2.3\n")
        self.assertEqual(self.text("website-docs/VERSION"), "2099.1.2.3\n")
        for d in ["frontend", "website-docs", "website-docs/homepage", "miniprogram"]:
            self.assertEqual(self.lock_versions(d), ["2099.1.2.3"] * 3, d)
        self.assertIn('\nversion = "2099.1.2.3"\n', self.text("docreader/pyproject.toml"))
        self.assertIn('name = "docreader"\nversion = "2099.1.2.3"\n', self.text("docreader/uv.lock"))
        self.assertIn('\nversion: 0.1.0\nappVersion: "v2099.1.2.3"\n', self.text("helm/Chart.yaml"))
        self.assertEqual(
            json.loads(self.text("cmd/desktop/wails.json"))["info"]["productVersion"], "2099.1.2"
        )
        self.assertIn("img.shields.io/badge/release-v2099.1.2.3-b8863b", self.text("README.md"))

        self.assertEqual(self.head_subject(), "chore(release): v2099.1.2.3")
        self.assertIn("\ngpgsig ", sh("git", "cat-file", "commit", "HEAD", cwd=self.repo))
        self.assertEqual(sh("git", "status", "--porcelain", cwd=self.repo), "")
        changed = sh("git", "diff", "--numstat", "HEAD~1", cwd=self.repo).splitlines()
        self.assertEqual(
            sorted(line.split("\t")[2] for line in changed),
            [
                "README.md",
                "VERSION",
                "cmd/desktop/wails.json",
                "docreader/pyproject.toml",
                "docreader/uv.lock",
                "frontend/package-lock.json",
                "frontend/package.json",
                "helm/Chart.yaml",
                "miniprogram/package-lock.json",
                "miniprogram/package.json",
                "website-docs/VERSION",
                "website-docs/homepage/package-lock.json",
                "website-docs/homepage/package.json",
                "website-docs/package-lock.json",
                "website-docs/package.json",
            ],
        )
        self.assertTrue(all(line.startswith(("1\t1\t", "2\t2\t")) for line in changed), changed)
        self.assertEqual(self.run_tool("check").returncode, 0)

    def test_default_bump_takes_todays_date_then_micro(self):
        t = datetime.date.today()
        today = f"{t.year}.{t.month}.{t.day}"
        if self.text("VERSION").strip() != today:
            self.assertEqual(self.run_tool("bump").returncode, 0)
            self.assertEqual(self.text("VERSION"), f"{today}\n")
        self.assertEqual(self.run_tool("bump").returncode, 0)
        self.assertEqual(self.text("VERSION"), f"{today}.1\n")
        sh("git", "-c", "tag.gpgsign=false", "tag", f"v{today}.2", cwd=self.repo)
        self.assertEqual(self.run_tool("bump").returncode, 0)
        self.assertEqual(self.text("VERSION"), f"{today}.3\n")
        self.assertEqual(self.head_subject(), f"chore(release): v{today}.3")

    def test_check_names_the_hand_edited_file(self):
        chart = self.repo / "helm/Chart.yaml"
        chart.write_text(re.sub(r'appVersion: "v[^"]+"', 'appVersion: "v1.2.3"', chart.read_text()))
        lock = self.repo / "packages/dsh-ai-kb/package-lock.json"
        lock.write_text(lock.read_text().replace('"version": ', '"version": "9.9.9", "old": ', 1))
        r = self.run_tool("check")
        self.assertEqual(r.returncode, 1)
        self.assertRegex(r.stderr, r'helm/Chart\.yaml:8: "1\.2\.3", want "[0-9.]+" \(product, from VERSION\)')
        self.assertRegex(
            r.stderr, r'packages/dsh-ai-kb/package-lock\.json:3: "9\.9\.9", want "[0-9.]+" \(dsh-ai-kb'
        )
        self.assertEqual(len(r.stderr.splitlines()), 2, r.stderr)
        self.assertEqual(self.run_tool("bump", "2099.1.1").returncode, 2)

    def test_bump_lib_patch(self):
        self.assertEqual(self.run_tool("bump-lib", "dsh-ai-kb", "1.4.9").returncode, 0)
        r = self.run_tool("bump-lib", "dsh-ai-kb", "patch")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.lock_versions("packages/dsh-ai-kb"), ["1.4.10"] * 3)
        self.assertEqual(self.head_subject(), "chore(release): dsh-ai-kb v1.4.10")
        self.assertIn("git tag -s dsh-ai-kb-v1.4.10 -m dsh-ai-kb-v1.4.10", r.stdout)
        self.assertEqual(
            sh("git", "diff", "--name-only", "HEAD~2", cwd=self.repo).split(),
            ["packages/dsh-ai-kb/package-lock.json", "packages/dsh-ai-kb/package.json"],
        )


if __name__ == "__main__":
    unittest.main()
