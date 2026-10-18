"""Tests for update_tap: run with `python3 -I scripts/test_update_tap.py`."""

import hashlib
import json
import os
import re
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import update_tap  # noqa: E402

TAG = "v2026.10.19"
BASE = "https://github.com/jrmatherly/ai-kb/releases/download"
LITE_ASSETS = [f"ai-kb-lite_{TAG}_{p}.tar.gz" for p in ("darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64")]
CLI_ASSETS = [f"aikb_{TAG}_{p}.tar.gz" for p in ("darwin_arm64", "darwin_amd64", "linux_arm64", "linux_amd64")]
ZIP_ASSETS = [f"aikb_{TAG}_windows_{a}.zip" for a in ("amd64", "arm64")]
OLD = "2026.10.18"


def fake_hash(name):
    return hashlib.sha256(name.encode()).hexdigest()


def formula(name, assets, version):
    lines = [f"class {name} < Formula", f'  version "{version}"']
    for asset in assets:
        # Real formulas interpolate the version into the URL, so the file name
        # the updater keys on is only known after substituting `version`.
        lines.append(f'      url "{BASE}/v#{{version}}/{asset.replace(TAG, "v#{version}")}"')
        lines.append(f'      sha256 "{"0" * 64}"')
    lines.append("end")
    return "\n".join(lines) + "\n"


class UpdateTapTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.mkdtemp()
        self.tap = os.path.join(self.tmp, "tap")
        self.shas = os.path.join(self.tmp, "shas")
        os.makedirs(os.path.join(self.tap, "Formula"))
        os.makedirs(os.path.join(self.tap, "bucket"))
        os.makedirs(self.shas)
        self.write("Formula/ai-kb-lite.rb", formula("AiKbLite", LITE_ASSETS, OLD))
        self.write("Formula/aikb.rb", formula("Aikb", CLI_ASSETS, OLD))
        self.write(
            "bucket/aikb.json",
            json.dumps(
                {
                    "version": OLD,
                    "architecture": {
                        "64bit": {"url": f"{BASE}/v{OLD}/aikb_v{OLD}_windows_amd64.zip", "hash": "0" * 64},
                        "arm64": {"url": f"{BASE}/v{OLD}/aikb_v{OLD}_windows_arm64.zip", "hash": "0" * 64},
                    },
                },
                indent=2,
            )
            + "\n",
        )
        for asset in LITE_ASSETS + CLI_ASSETS + ZIP_ASSETS:
            # Linux sha256sum writes two spaces; macOS shasum does the same.
            with open(os.path.join(self.shas, asset + ".sha256"), "w") as f:
                f.write(f"{fake_hash(asset)}  {asset}\n")

    def tearDown(self):
        shutil.rmtree(self.tmp)

    def write(self, rel, text):
        with open(os.path.join(self.tap, rel), "w") as f:
            f.write(text)

    def read(self, rel):
        with open(os.path.join(self.tap, rel)) as f:
            return f.read()

    def test_rewrites_every_hash_version_and_the_bucket(self):
        changed = update_tap.update_tap(TAG, self.shas, self.tap)
        self.assertEqual(sorted(changed), ["Formula/ai-kb-lite.rb", "Formula/aikb.rb", "bucket/aikb.json"])
        for rel, assets in (("Formula/ai-kb-lite.rb", LITE_ASSETS), ("Formula/aikb.rb", CLI_ASSETS)):
            text = self.read(rel)
            self.assertIn('version "2026.10.19"', text)
            self.assertNotIn('version "2026.10.18"', text)
            hashes = re.findall(r'sha256 "([0-9a-f]{64})"', text)
            self.assertEqual(hashes, [fake_hash(a) for a in assets])
        bucket = json.loads(self.read("bucket/aikb.json"))
        self.assertEqual(bucket["version"], "2026.10.19")
        self.assertEqual(bucket["architecture"]["64bit"]["url"], f"{BASE}/{TAG}/{ZIP_ASSETS[0]}")
        self.assertEqual(bucket["architecture"]["64bit"]["hash"], fake_hash(ZIP_ASSETS[0]))
        self.assertEqual(bucket["architecture"]["arm64"]["url"], f"{BASE}/{TAG}/{ZIP_ASSETS[1]}")
        self.assertEqual(bucket["architecture"]["arm64"]["hash"], fake_hash(ZIP_ASSETS[1]))
        self.assertTrue(self.read("bucket/aikb.json").endswith("}\n"))

    def test_second_run_changes_nothing(self):
        update_tap.update_tap(TAG, self.shas, self.tap)
        self.assertEqual(update_tap.update_tap(TAG, self.shas, self.tap), [])

    def test_missing_sha256_file_fails(self):
        os.remove(os.path.join(self.shas, CLI_ASSETS[2] + ".sha256"))
        with self.assertRaises(SystemExit):
            update_tap.update_tap(TAG, self.shas, self.tap)

    def test_malformed_sha256_line_fails(self):
        with open(os.path.join(self.shas, LITE_ASSETS[0] + ".sha256"), "w") as f:
            f.write("not-a-hash\n")
        with self.assertRaises(SystemExit):
            update_tap.update_tap(TAG, self.shas, self.tap)


if __name__ == "__main__":
    unittest.main()
