#!/usr/bin/env python3
"""Tests for scripts/version.py. Each test bumps a throwaway clone of HEAD, never this tree.

python3 scripts/test_version.py
"""

import datetime
import json
import os
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
        # The make targets run the clone's own Makefile and script, so test the working tree's.
        for f in ["Makefile", "scripts/version.py"]:
            (self.repo / f).write_bytes((ROOT / f).read_bytes())
        sh(
            "git",
            "-c",
            "commit.gpgsign=false",
            "commit",
            "-qa",
            "--allow-empty",
            "-m",
            "test: tooling",
            cwd=self.repo,
        )

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

    def next_version(self, micro):
        """A version after the clone's current one: the next day, with MICRO."""
        y, m, d = map(int, self.text("VERSION").strip().split(".")[:3])
        t = datetime.date(y, m, d) + datetime.timedelta(days=1)
        return f"{t.year}.{t.month}.{t.day}.{micro}"

    def make(self, *args, env_version=None):
        env = {k: v for k, v in os.environ.items() if k != "VERSION"}
        if env_version:
            env["VERSION"] = env_version
        return subprocess.run(
            ["make", *args], cwd=self.repo, capture_output=True, text=True, env=env, check=False
        )

    def head(self):
        return sh("git", "rev-parse", "HEAD", cwd=self.repo).strip()

    def head_subject(self):
        return sh("git", "log", "-1", "--format=%s", cwd=self.repo).strip()

    def test_bump_rewrites_every_product_touchpoint(self):
        bad = self.run_tool("bump", "2099.01.02")
        self.assertEqual(bad.returncode, 2)
        self.assertIn("'2099.01.02' is not YYYY.M.D[.MICRO]", bad.stderr)
        self.assertEqual(self.run_tool("bump", "2020.1.1").returncode, 2)

        target = self.next_version(3)
        day = target.rsplit(".", 1)[0]
        chart_version = re.search(r"^version: .*$", self.text("helm/Chart.yaml"), re.MULTILINE)[0]
        r = self.run_tool("bump", target)
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertIn(f"git tag -s v{target} -m v{target}", r.stdout)
        self.assertEqual(self.text("VERSION"), f"{target}\n")
        self.assertEqual(self.text("website-docs/VERSION"), f"{target}\n")
        for d in ["frontend", "website-docs", "website-docs/homepage", "miniprogram"]:
            self.assertEqual(self.lock_versions(d), [target] * 3, d)
        self.assertIn(f'\nversion = "{target}"\n', self.text("docreader/pyproject.toml"))
        self.assertIn(f'name = "docreader"\nversion = "{target}"\n', self.text("docreader/uv.lock"))
        self.assertIn(f'\n{chart_version}\nappVersion: "v{target}"\n', self.text("helm/Chart.yaml"))
        self.assertEqual(json.loads(self.text("cmd/desktop/wails.json"))["info"]["productVersion"], day)
        self.assertIn(f"img.shields.io/badge/release-v{target}-b8863b", self.text("README.md"))

        self.assertEqual(self.head_subject(), f"chore(release): v{target}")
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

    def test_default_bump_takes_todays_date_then_micro_past_local_and_remote_tags(self):
        origin = self.repo.parent / "origin.git"
        sh("git", "init", "-q", "--bare", str(origin), cwd=self.repo.parent)
        sh("git", "remote", "set-url", "origin", str(origin), cwd=self.repo)
        t = datetime.date.today()
        today = f"{t.year}.{t.month}.{t.day}"
        if self.text("VERSION").strip() != today:
            self.assertEqual(self.run_tool("bump").returncode, 0)
            self.assertEqual(self.text("VERSION"), f"{today}\n")
        self.assertEqual(self.run_tool("bump").returncode, 0)
        self.assertEqual(self.text("VERSION"), f"{today}.1\n")
        sh("git", "-c", "tag.gpgsign=false", "tag", f"v{today}.2", cwd=self.repo)
        sh("git", "-c", "tag.gpgsign=false", "tag", f"v{today}.3", cwd=self.repo)
        sh("git", "push", "-q", "origin", f"v{today}.3", cwd=self.repo)
        sh("git", "tag", "-d", f"v{today}.3", cwd=self.repo)
        self.assertEqual(self.run_tool("bump").returncode, 0)
        self.assertEqual(self.text("VERSION"), f"{today}.4\n")
        self.assertEqual(self.head_subject(), f"chore(release): v{today}.4")

    def test_explicit_version_with_an_existing_tag_is_refused(self):
        target = self.next_version(1)
        sh("git", "-c", "tag.gpgsign=false", "tag", f"v{target}", cwd=self.repo)
        before = self.head()
        r = self.run_tool("bump", target)
        self.assertEqual(r.returncode, 2)
        self.assertIn(f"tag v{target} already exists", r.stderr)
        self.assertEqual(self.head(), before)
        self.assertEqual(sh("git", "status", "--porcelain", cwd=self.repo), "")

    def test_failed_commit_restores_every_file(self):
        hooks = self.repo.parent / "hooks"
        hooks.mkdir()
        (hooks / "pre-commit").write_text("#!/bin/sh\nexit 1\n")
        (hooks / "pre-commit").chmod(0o755)
        sh("git", "config", "core.hooksPath", str(hooks), cwd=self.repo)
        before, version = self.head(), self.text("VERSION")
        r = self.run_tool("bump", self.next_version(1))
        self.assertEqual(r.returncode, 2)
        self.assertIn("restored 15 file(s) to their original bytes", r.stderr)
        self.assertEqual(self.head(), before)
        self.assertEqual(self.text("VERSION"), version)
        self.assertEqual(sh("git", "status", "--porcelain", cwd=self.repo), "")

    def test_make_takes_version_only_from_the_command_line(self):
        target, version = self.next_version(1), self.text("VERSION")
        r = self.make("bump-version", env_version=target)
        self.assertNotEqual(r.returncode, 0)
        self.assertIn(f"VERSION={target} comes from the environment; pass it after make", r.stderr)
        r = self.make("bump-lib", "LIB=dsh-ai-kb", env_version="99.0.0")
        self.assertNotEqual(r.returncode, 0)
        self.assertIn("make bump-lib VERSION=...) or unset VERSION", r.stderr)
        self.assertEqual(self.text("VERSION"), version)
        self.assertEqual(self.make("check-version", env_version=target).returncode, 0)
        r = self.make("bump-version", f"VERSION={target}")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.text("VERSION"), f"{target}\n")

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
        self.assertEqual(self.run_tool("bump", self.next_version(1)).returncode, 2)

    def test_check_rejects_an_impossible_date(self):
        for bad in ["2026.2.30", "2026.13.1"]:
            (self.repo / "VERSION").write_text(f"{bad}\n")
            r = self.run_tool("check")
            self.assertEqual(r.returncode, 1)
            self.assertIn(f"VERSION: '{bad}' is not a real date (", r.stderr)

    def test_bump_lib_patch(self):
        major = int(json.loads(self.text("packages/dsh-ai-kb/package.json"))["version"].split(".")[0]) + 1
        self.assertEqual(self.run_tool("bump-lib", "dsh-ai-kb", f"{major}.4.9").returncode, 0)
        r = self.run_tool("bump-lib", "dsh-ai-kb", "patch")
        self.assertEqual(r.returncode, 0, r.stderr)
        self.assertEqual(self.lock_versions("packages/dsh-ai-kb"), [f"{major}.4.10"] * 3)
        self.assertEqual(self.head_subject(), f"chore(release): dsh-ai-kb v{major}.4.10")
        self.assertIn(f"git tag -s dsh-ai-kb-v{major}.4.10 -m dsh-ai-kb-v{major}.4.10", r.stdout)
        self.assertEqual(
            sh("git", "diff", "--name-only", "HEAD~2", cwd=self.repo).split(),
            ["packages/dsh-ai-kb/package-lock.json", "packages/dsh-ai-kb/package.json"],
        )


if __name__ == "__main__":
    unittest.main()
