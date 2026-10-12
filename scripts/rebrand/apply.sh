#!/usr/bin/env bash
# apply.sh - apply scripts/rebrand/map.tsv to files in place (idempotent). See rebrand.pl.
exec "$(dirname "$0")/rebrand.pl" --apply "$@"
