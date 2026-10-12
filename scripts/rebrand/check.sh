#!/usr/bin/env bash
# check.sh - exit 1 when apply.sh would still change a file, printing each old/new line. See rebrand.pl.
exec "$(dirname "$0")/rebrand.pl" --check "$@"
