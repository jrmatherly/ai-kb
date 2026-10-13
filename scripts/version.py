#!/usr/bin/env python3
"""Check and bump every version string in the repository from one table.

  version.py check                         product strings equal VERSION; each library agrees with itself
  version.py bump [YYYY.M.D[.MICRO]]       product CalVer bump (default: today, then .1, .2 ...), one commit
  version.py bump-lib <lib> <part|x.y.z>   library semver bump (part: patch, minor, major), one commit

Each touchpoint is a regex whose group `v` must match exactly once, so a file that changes shape fails
loudly instead of being skipped. A bump rewrites only that group, which keeps formatting, line endings
and lockfile dependencies untouched (`npm install --package-lock-only` also rewrites unrelated entries).
Bumps never tag or push: they print the commands.
"""

import datetime
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(
    subprocess.run(
        ["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True
    ).stdout.strip()
)
CALVER = re.compile(r"(\d{4})\.([1-9]\d?)\.([1-9]\d?)(?:\.([1-9]\d*))?")
SEMVER = re.compile(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)")
Q = r'"(?P<v>[^"]+)"'


@dataclass(frozen=True)
class Touchpoint:
    group: str
    path: str
    pattern: str
    # Windows version resources: Wails appends ".0" to productVersion in the NSIS installer and the
    # exe manifest, which allow four parts, so this one carries the date without MICRO.
    day_only: bool = False


def npm(group, d):
    return [
        Touchpoint(group, f"{d}/package.json", rf"^  \"version\": {Q}"),
        Touchpoint(group, f"{d}/package-lock.json", rf"^  \"version\": {Q}"),
        Touchpoint(
            group, f"{d}/package-lock.json", rf"^    \"\": \{{\n(?:      .*\n)*?      \"version\": {Q}"
        ),
    ]


def uv(group, d, name):
    return [
        Touchpoint(group, f"{d}/pyproject.toml", rf"^\[project\]\n(?:[^\[].*\n|\n)*?version = {Q}"),
        Touchpoint(group, f"{d}/uv.lock", rf"^\[\[package\]\]\nname = \"{name}\"\nversion = {Q}"),
    ]


TOUCHPOINTS = [
    Touchpoint("product", "VERSION", r"\A(?P<v>\S+)\n\Z"),
    Touchpoint("product", "website-docs/VERSION", r"\A(?P<v>\S+)\n\Z"),
    *npm("product", "frontend"),
    *npm("product", "website-docs"),
    *npm("product", "website-docs/homepage"),
    *npm("product", "miniprogram"),
    *uv("product", "docreader", "docreader"),
    Touchpoint("product", "helm/Chart.yaml", r'^appVersion: "v(?P<v>[^"]+)"$'),
    Touchpoint("product", "cmd/desktop/wails.json", rf"^    \"productVersion\": {Q}", day_only=True),
    Touchpoint("product", "README.md", r"img\.shields\.io/badge/release-v(?P<v>[^-]+)-"),
    *npm("dsh-ai-kb", "packages/dsh-ai-kb"),
    *uv("mcp-server", "mcp-server", "ai-kb-mcp"),
    Touchpoint("mcp-server", "mcp-server/setup.py", rf"^    version={Q}"),
    Touchpoint("mcp-server", "mcp-server/__init__.py", rf"^__version__ = {Q}"),
    Touchpoint(
        "mcp-server", "mcp-server/aikb_mcp_server.py", r'MCPServer\("ai-kb-server", version="(?P<v>[^"]+)"\)'
    ),
    Touchpoint("mcp-server", "mcp-server/main.py", r'version="AI KB MCP Server (?P<v>[^"]+)"'),
    Touchpoint("helm-chart", "helm/Chart.yaml", r"^version: (?P<v>\S+)$"),
]
# The tag each publish workflow expects; None: nothing publishes it.
TAG_PREFIX = {"product": "v", "dsh-ai-kb": "dsh-ai-kb-v", "mcp-server": "mcp-server-v", "helm-chart": None}
# Lockfiles uv can verify offline after their version line changes.
UV_PROJECTS = {"product": ["docreader"], "mcp-server": ["mcp-server"]}


class Fail(Exception):
    pass


def read_text(path):
    with open(ROOT / path, encoding="utf-8", newline="") as f:
        return f.read()


def locate(tp, text):
    matches = list(re.finditer(tp.pattern, text, re.MULTILINE))
    if len(matches) != 1:
        raise Fail(f"{tp.path}: pattern {tp.pattern!r} matched {len(matches)} times, want 1")
    return matches[0]


def read(tp):
    return locate(tp, read_text(tp.path))["v"]


def where(tp):
    text = read_text(tp.path)
    return f"{tp.path}:{text.count(chr(10), 0, locate(tp, text).start('v')) + 1}"


def write(tp, value):
    text = read_text(tp.path)
    m = locate(tp, text)
    with open(ROOT / tp.path, "w", encoding="utf-8", newline="") as f:
        f.write(text[: m.start("v")] + value + text[m.end("v") :])


def form(tp, version):
    return ".".join(version.split(".")[:3]) if tp.day_only else version


def rows(group):
    return [tp for tp in TOUCHPOINTS if tp.group == group]


def check():
    """Every mismatch as 'path: "found", want "expected"'; empty when consistent."""
    errors = []
    for group in TAG_PREFIX:
        tps = rows(group)
        want = read(tps[0])
        shape = CALVER if group == "product" else SEMVER
        if not shape.fullmatch(want):
            errors.append(f"{tps[0].path}: {want!r} is not {'CalVer' if group == 'product' else 'semver'}")
        for tp in tps[1:]:
            try:
                got = read(tp)
            except Fail as e:
                errors.append(str(e))
                continue
            if got != form(tp, want):
                errors.append(f'{where(tp)}: "{got}", want "{form(tp, want)}" ({group}, from {tps[0].path})')
    return errors


def key(version):
    return tuple(int(p) for p in version.split(".")) + (0,) * (4 - version.count(".") - 1)


def git(*args):
    return subprocess.run(["git", *args], cwd=ROOT, capture_output=True, text=True, check=True).stdout


def next_calver(current, today):
    day = f"{today.year}.{today.month}.{today.day}"
    if key(day)[:3] < key(current)[:3]:
        raise Fail(f"today ({day}) is before the current version {current}; pass VERSION=")
    candidate, micro = day, 0
    while key(candidate) <= key(current) or git("tag", "-l", f"v{candidate}").strip():
        micro += 1
        candidate = f"{day}.{micro}"
    return candidate


def next_semver(current, arg):
    if arg in ("patch", "minor", "major"):
        major, minor, patch = map(int, current.split("."))
        return {
            "major": f"{major + 1}.0.0",
            "minor": f"{major}.{minor + 1}.0",
            "patch": f"{major}.{minor}.{patch + 1}",
        }[arg]
    return arg


def bump(group, target):
    errors = check()
    if errors:
        raise Fail("fix the drift first (make check-version):\n  " + "\n  ".join(errors))
    if git("status", "--porcelain", "--untracked-files=no").strip():
        raise Fail("the working tree has uncommitted changes; commit them first")
    tps = rows(group)
    current = read(tps[0])
    shape = CALVER if group == "product" else SEMVER
    if not shape.fullmatch(target):
        raise Fail(
            f"{target!r} is not {'YYYY.M.D[.MICRO] (unpadded, MICRO >= 1)' if group == 'product' else 'x.y.z'}"
        )
    if group == "product":
        datetime.date(*map(int, target.split(".")[:3]))
    if key(target) <= key(current):
        raise Fail(f"{target} is not greater than the current {group} version {current}")
    for tp in tps:
        write(tp, form(tp, target))
    for project in UV_PROJECTS.get(group, []):
        subprocess.run(["uv", "lock", "--check", "--offline", "--project", project], cwd=ROOT, check=True)
    errors = check()
    if errors:
        raise Fail("check failed after the bump:\n  " + "\n  ".join(errors))
    name = f"v{target}" if group == "product" else f"{group} v{target}"
    paths = sorted({tp.path for tp in tps})
    subprocess.run(
        ["git", "commit", "-S", "-q", "-m", f"chore(release): {name}", "--", *paths], cwd=ROOT, check=True
    )
    print(f"committed chore(release): {name} ({len(paths)} files)")
    prefix = TAG_PREFIX[group]
    if prefix is None:
        print(f"no tag: no workflow publishes the {group} (helm.yml only lints and renders it)")
        return
    branch = git("rev-parse", "--abbrev-ref", "HEAD").strip()
    print(
        f"next:\n  git tag -s {prefix}{target} -m {prefix}{target}\n  git push origin {branch} {prefix}{target}"
    )


def main(argv):
    cmd, args = (argv[0], argv[1:]) if argv else ("", [])
    try:
        if cmd == "check" and not args:
            errors = check()
            for e in errors:
                print(e, file=sys.stderr)
            if not errors:
                print(f"versions consistent: product {read(TOUCHPOINTS[0])}")
            return 1 if errors else 0
        if cmd == "bump" and len(args) <= 1:
            current = read(rows("product")[0])
            bump("product", args[0] if args else next_calver(current, datetime.date.today()))
            return 0
        if cmd == "bump-lib" and len(args) == 2:
            libs = [g for g in TAG_PREFIX if g != "product"]
            if args[0] not in libs:
                raise Fail(f"unknown library {args[0]!r}; one of {', '.join(libs)}")
            bump(args[0], next_semver(read(rows(args[0])[0]), args[1]))
            return 0
    except (Fail, ValueError, OSError) as e:
        print(f"version.py: {e}", file=sys.stderr)
        return 2
    except subprocess.CalledProcessError as e:
        print(f"version.py: {' '.join(e.cmd)} exited {e.returncode}", file=sys.stderr)
        return 2
    print("usage:\n" + "\n".join(__doc__.splitlines()[2:5]), file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
