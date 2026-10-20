#!/usr/bin/env bash
set -euo pipefail

license_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_dir="${1:-}"
if [ -n "${source_dir}" ]; then
    source_dir="$(cd "${source_dir}" && pwd)"
fi
cd "${license_root}"

# A retained build-graph dependency would bring the GPL converter back even if
# application code no longer imported it directly.
if grep -Eq 'github.com/(longbridgeapp/opencc|liuzl/(da|cedar-go)|adamzy/cedar-go)([[:space:]]|$)' go.mod go.sum; then
    echo "Removed conversion dependencies have reappeared in go.mod/go.sum" >&2
    exit 1
fi

test -s licenses/sources/modules.tsv
while read -r module_name module_version checksum; do
    [[ "${checksum}" =~ ^[0-9a-f]{64}$ ]] || { echo "Invalid source checksum for ${module_name}" >&2; exit 1; }
    actual_version="$(awk -v name="${module_name}" '$1 == name { print $2 }' go.mod)"
    if [ "${actual_version}" != "${module_version}" ]; then
        echo "Update the source manifest for ${module_name}: expected ${module_version}, got ${actual_version}" >&2
        exit 1
    fi
    if [ -n "${source_dir}" ]; then
        archive="${module_name##*/}-${module_version}.zip"
        (
            cd "${source_dir}"
            if command -v sha256sum >/dev/null 2>&1; then
                printf '%s  %s\n' "${checksum}" "${archive}" | sha256sum -c -
            else
                printf '%s  %s\n' "${checksum}" "${archive}" | shasum -a 256 -c -
            fi
        )
    fi
done < licenses/sources/modules.tsv

test -s licenses/components.tsv
# The Windows release runner may expose python3 only as a Store stub, so probe before use.
for python in python3 python; do "${python}" -c pass 2>/dev/null && break; done
"${python}" - <<'PY'
import glob, json, re, shutil, subprocess, sys
from importlib.metadata import distributions
from pathlib import Path

def fail(msg):
    print(msg, file=sys.stderr)
    sys.exit(1)

def norm(name):
    return re.sub(r"[-_.]+", "-", name).lower()

reviewed = set()
for line in Path("licenses/components.tsv").read_text().splitlines():
    fields = line.split("\t")
    if len(fields) != 2 or fields[0] not in ("npm", "python"):
        fail(f"licenses/components.tsv: expected 'npm|python<TAB>name', got {line!r}")
    reviewed.add(tuple(fields))
npm_packages = [(path.rsplit("node_modules/", 1)[-1], entry)
                for path, entry in json.loads(Path("frontend/package-lock.json").read_text())["packages"].items()
                if path and not entry.get("dev") and not entry.get("extraneous")]
uv_lock = Path("docreader/uv.lock").read_text()
notices = Path("THIRD_PARTY_NOTICES.md").read_text()

for eco, name in reviewed:
    if eco == "npm":
        locked = [entry["version"] for pkg, entry in npm_packages if pkg == name]
    else:
        locked = re.findall(r'^name = "' + re.escape(name) + r'"\nversion = "([^"]+)"', uv_lock, re.M)
    if not locked:
        fail(f"licenses/components.tsv names {name}, which the {eco} lockfile no longer has")
    for version in locked:
        if not re.search(re.escape(name) + r"\b.*`" + re.escape(version) + "`", notices):
            fail(f"THIRD_PARTY_NOTICES.md does not name {name} `{version}` on one line")

COPYLEFT = re.compile(r"\b(AGPL|GPL|LGPL|MPL|EPL|CDDL|EUPL|OSL|CPAL|SSPL|CeCILL|CC-BY-SA)|General Public License|Public License", re.I)

# An SPDX OR expression with one permissive alternative is used under that alternative.
def needs_notice(expression):
    if not expression or expression.strip().upper() in ("UNKNOWN", "OSI APPROVED") or "Proprietary" in expression:
        return True
    return all(COPYLEFT.search(alt) for alt in expression.split(" OR "))

unlisted = []

def check(eco, name, version, expression):
    if needs_notice(expression) and (eco, name) not in reviewed:
        unlisted.append(f"{eco} {name} {version} ({expression or 'no license'})")

for name, entry in npm_packages:
    check("npm", name, entry.get("version"), entry.get("license"))

# uv.lock carries no license metadata, so the Python pass needs the installed environment.
site = glob.glob("docreader/.venv/lib/python*/site-packages") + glob.glob("docreader/.venv/Lib/site-packages")
if site and shutil.which("uv"):
    export = subprocess.run(["uv", "export", "--project", "docreader", "--frozen", "--no-dev", "--no-hashes", "--no-header"],
                            check=True, capture_output=True, text=True).stdout
    runtime = {norm(m) for m in re.findall(r"^([A-Za-z0-9._-]+)==", export, re.M)}
    for dist in distributions(path=site):
        meta = dist.metadata
        if norm(meta["Name"]) not in runtime:
            continue
        classifiers = [c.rsplit("::", 1)[-1].strip() for c in meta.get_all("Classifier") or [] if c.startswith("License ::")]
        # Older metadata writes "License: UNKNOWN" or the whole license text; classifiers are the reliable field there.
        single_line = meta.get("License") if meta.get("License") and "\n" not in meta.get("License") else ""
        expression = meta.get("License-Expression") or " OR ".join(classifiers) or single_line
        check("python", meta["Name"], dist.version, expression)
else:
    print("skipped: python discovery, docreader/.venv or uv absent")
if unlisted:
    fail("Add these copyleft or unlicensed packages to licenses/components.tsv and THIRD_PARTY_NOTICES.md:\n  " + "\n  ".join(unlisted))
PY

for file in LICENSE THIRD_PARTY_NOTICES.md licenses/OpenCC-Apache-2.0.txt \
    licenses/elkjs-EPL-2.0.txt \
    licenses/go-sql-driver-mysql-MPL-2.0.txt \
    licenses/Wails-MIT.txt; do
    test -s "${file}"
done
