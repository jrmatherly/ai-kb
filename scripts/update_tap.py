"""Rewrite the homebrew-ai-kb tap for one release.

update_tap(tag, sha_dir, tap_dir) rewrites Formula/ai-kb-lite.rb, Formula/aikb.rb
and bucket/aikb.json in a tap checkout from the `<asset>.sha256` files of that
release and returns the files it changed. Every hash is keyed by the asset file
name its `url` ends with, so a formula with its blocks in any order stays right.
"""

import json
import os
import re
import sys

FORMULAS = ("Formula/ai-kb-lite.rb", "Formula/aikb.rb")
BUCKET = "bucket/aikb.json"
BUCKET_ASSETS = {"64bit": "windows_amd64", "arm64": "windows_arm64"}
RELEASE_BASE = "https://github.com/jrmatherly/ai-kb/releases/download"
URL_RE = re.compile(r'^(\s*url\s+")([^"]+)(")\s*$')
SHA_RE = re.compile(r'^(\s*sha256\s+")([0-9a-f]{64})(")\s*$')
VERSION_RE = re.compile(r'^(\s*version\s+")([^"]+)(")\s*$')


def fail(message):
    print(f"update_tap: {message}", file=sys.stderr)
    raise SystemExit(1)


def read_hash(sha_dir, asset):
    path = os.path.join(sha_dir, asset + ".sha256")
    if not os.path.isfile(path):
        fail(f"missing {path}")
    with open(path) as f:
        fields = f.read().split()
    if len(fields) < 1 or not re.fullmatch(r"[0-9a-f]{64}", fields[0]):
        fail(f"malformed hash line in {path}")
    return fields[0]


def rewrite_formula(text, version, sha_dir):
    out = []
    pending = None
    for line in text.splitlines(keepends=True):
        if m := VERSION_RE.match(line):
            line = f"{m.group(1)}{version}{m.group(3)}\n"
        elif m := URL_RE.match(line):
            url = m.group(2).replace("#{version}", version)
            pending = url.rsplit("/", 1)[-1]
        elif (m := SHA_RE.match(line)) and pending:
            line = f"{m.group(1)}{read_hash(sha_dir, pending)}{m.group(3)}\n"
            pending = None
        out.append(line)
    return "".join(out)


def rewrite_bucket(text, tag, version, sha_dir):
    data = json.loads(text)
    data["version"] = version
    for arch, suffix in BUCKET_ASSETS.items():
        asset = f"aikb_{tag}_{suffix}.zip"
        data["architecture"][arch]["url"] = f"{RELEASE_BASE}/{tag}/{asset}"
        data["architecture"][arch]["hash"] = read_hash(sha_dir, asset)
    return json.dumps(data, indent=2) + "\n"


def update_tap(tag, sha_dir, tap_dir):
    version = tag[1:] if tag.startswith("v") else tag
    changed = []
    for rel in FORMULAS + (BUCKET,):
        path = os.path.join(tap_dir, rel)
        if not os.path.isfile(path):
            continue
        with open(path) as f:
            before = f.read()
        if rel == BUCKET:
            after = rewrite_bucket(before, tag, version, sha_dir)
        else:
            after = rewrite_formula(before, version, sha_dir)
        if after != before:
            with open(path, "w") as f:
                f.write(after)
            changed.append(rel)
    return changed


def main(argv):
    if argv[1:] == ["--self-check"]:
        import unittest

        sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
        suite = unittest.defaultTestLoader.loadTestsFromName("test_update_tap")
        result = unittest.TextTestRunner().run(suite)
        return 0 if result.wasSuccessful() else 1
    if len(argv) != 4:
        print("usage: update_tap.py <tag> <sha256-dir> <tap-dir> | --self-check", file=sys.stderr)
        return 2
    for rel in update_tap(argv[1], argv[2], argv[3]):
        print(rel)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
