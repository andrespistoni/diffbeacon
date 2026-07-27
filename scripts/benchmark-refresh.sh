#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
SAMPLES=${DIFFBEACON_PERF_SAMPLES:-25}
REPORT=${DIFFBEACON_PERF_REPORT:-$ROOT/build/performance/report.json}

case "$SAMPLES" in
	''|*[!0-9]*) echo "DIFFBEACON_PERF_SAMPLES must be an integer >= 20" >&2; exit 2 ;;
esac
if [ "$SAMPLES" -lt 20 ]; then
	echo "DIFFBEACON_PERF_SAMPLES must be >= 20 for a meaningful p95" >&2
	exit 2
fi

if filesystem=$(stat -f -c %T "$ROOT" 2>/dev/null); then
	:
elif filesystem=$(stat -f %T "$ROOT" 2>/dev/null); then
	:
else
	filesystem=unknown
fi

mkdir -p "$(dirname -- "$REPORT")"
export DIFFBEACON_PERF_SAMPLES=$SAMPLES
export DIFFBEACON_PERF_REPORT=$REPORT
export DIFFBEACON_PERF_FILESYSTEM=$filesystem

echo "DiffBeacon reproducible latency run"
echo "  samples:    $SAMPLES startup + $SAMPLES refresh"
echo "  filesystem: $filesystem"
echo "  system:     $(uname -a)"
echo "  go:         $(go version)"
echo "  git:        $(git --version)"
(
	cd "$ROOT"
	go test ./test/performance -run '^TestRefreshLatency$' -count=1 -v
)
test -s "$REPORT"
echo "Performance report: $REPORT"
