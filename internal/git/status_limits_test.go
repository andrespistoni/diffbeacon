package git

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseStatusRejectsEntryBudgetWithoutPartialSnapshot(t *testing.T) {
	data := []byte("# branch.oid (initial)\x00# branch.head main\x00? one\x00? two\x00")
	snapshot, err := parseStatusLimited(data, 1)
	if !errors.Is(err, ErrStatusBudget) {
		t.Fatalf("parseStatusLimited() error = %v, want ErrStatusBudget", err)
	}
	if len(snapshot.Changes) != 0 || snapshot.Revision.Valid() {
		t.Fatalf("partial snapshot escaped: %#v", snapshot)
	}
}

func TestQueryStatusRejectsByteBudget(t *testing.T) {
	runner := helperRunner(t)
	repository := Repository{Root: t.TempDir(), GitDir: t.TempDir()}
	_, err := queryStatusWithLimits(context.Background(), runner, repository, statusLimits{maxBytes: 4, maxEntries: 10, timeout: 10 * time.Second})
	if !errors.Is(err, ErrStatusBudget) || !strings.Contains(err.Error(), "4 bytes") {
		t.Fatalf("queryStatusWithLimits() error = %v", err)
	}
}

func TestQueryStatusCancelsAtTimeBudget(t *testing.T) {
	runner := helperRunner(t)
	runner.extraEnv = append(runner.extraEnv, "DIFFBEACON_TEST_SLOW_STATUS=1")
	repository := Repository{Root: t.TempDir(), GitDir: t.TempDir()}
	started := time.Now()
	_, err := queryStatusWithLimits(context.Background(), runner, repository, statusLimits{maxBytes: 1024, maxEntries: 10, timeout: 250 * time.Millisecond})
	if !errors.Is(err, ErrStatusBudget) {
		t.Fatalf("queryStatusWithLimits() error = %v, want ErrStatusBudget", err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("status cancellation took %s", elapsed)
	}
}
