#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
MINIMUM=2.31.0

if [ "$#" -eq 0 ]; then
	set -- "$(command -v git)"
fi

ORIGINAL_PATH=$PATH
seen_minimum=0
for candidate do
	case "$candidate" in
		*/*) git_bin=$candidate ;;
		*) git_bin=$(command -v "$candidate") ;;
	esac
	if [ ! -x "$git_bin" ]; then
		echo "Git candidate is not executable: $git_bin" >&2
		exit 2
	fi
	version=$($git_bin --version)
	case "$version" in
		"git version $MINIMUM"*) seen_minimum=1 ;;
	esac
	echo "==> compatibility matrix: $version ($git_bin)"
	PATH=$(dirname -- "$git_bin"):$ORIGINAL_PATH
	export PATH
	resolved=$(command -v git)
	if [ "$resolved" != "$git_bin" ]; then
		echo "Selected Git does not resolve first in PATH: wanted $git_bin, got $resolved" >&2
		exit 2
	fi
	(
		cd "$ROOT"
		go test ./... -count=1
	)
done

if [ "$seen_minimum" -eq 0 ]; then
	echo "NOTE: Git $MINIMUM was not supplied; this run validates only the listed local version(s)." >&2
	echo "CI supplies Git $MINIMUM and records the complete minimum-version matrix." >&2
fi
