#!/usr/bin/env python3
"""Refuse a product version bump while a cheaper fix is still pending.

Run by `make bump-version` before `version.py bump`. A release cycle (bump, tag, build, publish,
cluster bundle) costs an hour; a dependency fix or a red main found after it costs a second cycle.

Blocks (exit 1):
  - HEAD is not on origin, or a CI run on HEAD failed or has not finished: push, then wait.
  - An open Dependabot alert. guards.yml fails on one too, but an alert opened after HEAD's run
    finished leaves that run green, so the gate asks again.
Notes (printed, exit 0):
  - Upgrades `uv lock --upgrade` would take now in each uv project, past the release cooldown.

`make bump-version SKIP_PREFLIGHT=1` skips the gate.
"""

import json
import re
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

GREEN = {"success", "skipped", "neutral"}
UV_UPDATE = re.compile(r"^(Update|Add|Remove) (\S+) (.*)$", re.M)


@dataclass(frozen=True)
class Finding:
    block: bool
    text: str


def run(*args, check=True):
    return subprocess.run(args, cwd=ROOT, capture_output=True, text=True, check=check).stdout


def ci_findings(sha, runs):
    """`runs`: gh run list --json name,status,conclusion for the commit."""
    if not runs:
        return [Finding(True, f"no CI run found for {sha[:9]}: wait for the push's workflows to start")]
    out = []
    for r in runs:
        if r["status"] != "completed":
            out.append(Finding(True, f"CI run '{r['name']}' on {sha[:9]} is {r['status']}: wait for it"))
        elif r["conclusion"] not in GREEN:
            out.append(Finding(True, f"CI run '{r['name']}' on {sha[:9]} ended {r['conclusion']}"))
    return out


def alert_findings(alerts):
    """`alerts`: the Dependabot alerts API rows (state=open)."""
    out = []
    for a in alerts:
        pkg = a["dependency"]["package"]
        patched = (a["security_vulnerability"].get("first_patched_version") or {}).get("identifier")
        out.append(Finding(True, (
            f"Dependabot: {pkg['ecosystem']} {pkg['name']} in {a['dependency'].get('manifest_path', '?')}"
            f" ({a['security_advisory']['severity']}, {a['security_advisory']['ghsa_id']}),"
            f" fixed in {patched or 'no fix published'}"
        )))
    return out


def uv_findings(project, returncode, dry_run_output):
    """`dry_run_output`: `uv lock --upgrade --dry-run` text, one 'Update pkg vA -> vB' line per change."""
    if returncode:
        last = dry_run_output.strip().splitlines()[-1:] or ["no output"]
        return [Finding(False, f"{project}: uv dry run failed, upgrades unknown: {last[0]}")]
    notes = [
        Finding(False, f"{project}: uv would {m.group(1).lower()} {m.group(2)} {m.group(3)}")
        for m in UV_UPDATE.finditer(dry_run_output)
    ]
    if not notes and "changes detected" in dry_run_output:
        notes.append(Finding(False, f"{project}: uv would change the lockfile"))
    return notes


def gather():
    sha = run("git", "rev-parse", "HEAD").strip()
    if not run("git", "branch", "-r", "--contains", sha, check=False).strip():
        findings = [Finding(True, f"HEAD {sha[:9]} is on no fetched remote branch: push it (or fetch) and let CI finish")]
    else:
        runs = json.loads(run("gh", "run", "list", "--commit", sha, "--limit", "50", "--json", "name,status,conclusion"))
        findings = ci_findings(sha, runs)

    alerts = json.loads(run("gh", "api", "repos/{owner}/{repo}/dependabot/alerts?state=open&per_page=100"))
    findings += alert_findings(alerts)

    for lock in run("git", "ls-files", "*uv.lock").split():
        project = str(Path(lock).parent)
        uv = subprocess.run(
            ["uv", "lock", "--upgrade", "--dry-run", "--project", project], cwd=ROOT, capture_output=True, text=True
        )
        findings += uv_findings(project, uv.returncode, uv.stdout + uv.stderr)
    return findings


def main():
    try:
        findings = gather()
    except subprocess.CalledProcessError as e:
        print(f"release-preflight: `{' '.join(e.cmd)}` exited {e.returncode}\n{e.stderr}", file=sys.stderr)
        return 2
    blocks = [f for f in findings if f.block]
    for f in findings:
        print(("BLOCK " if f.block else "note  ") + f.text)
    if blocks:
        print(f"release-preflight: {len(blocks)} blocker(s); fix them or run with SKIP_PREFLIGHT=1", file=sys.stderr)
        return 1
    print("release-preflight: clear")
    return 0


if __name__ == "__main__":
    sys.exit(main())
