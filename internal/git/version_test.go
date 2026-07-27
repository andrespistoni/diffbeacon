package git

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestParseVersionAcceptsSupportedGitFormats(t *testing.T) {
	tests := map[string]Version{
		"git version 2.31.0":           {Major: 2, Minor: 31, Patch: 0},
		"git version 2.53.0.windows.1": {Major: 2, Minor: 53, Patch: 0},
		"git version 2.31":             {Major: 2, Minor: 31, Patch: 0},
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			got, err := ParseVersion(input)
			if err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("ParseVersion(%q) = %v, want %v", input, got, want)
			}
		})
	}
}

func TestCheckCompatibilityAcceptsMinimumVersion(t *testing.T) {
	runner := versionHelperRunner(t, "git version "+MinimumVersion.String())
	got, err := CheckCompatibility(context.Background(), runner)
	if err != nil {
		t.Fatal(err)
	}
	if got != MinimumVersion {
		t.Fatalf("version = %v, want %v", got, MinimumVersion)
	}
}

func TestCheckCompatibilityRejectsOlderVersionBeforeStartup(t *testing.T) {
	runner := versionHelperRunner(t, "git version 2.30.9")
	_, err := CheckCompatibility(context.Background(), runner)
	if !errors.Is(err, ErrGitIncompatible) {
		t.Fatalf("error = %v, want ErrGitIncompatible", err)
	}
	if !strings.Contains(err.Error(), MinimumVersion.String()) {
		t.Fatalf("error = %q, want minimum version", err)
	}
}

func TestCheckCompatibilityRejectsUnparseableVersion(t *testing.T) {
	runner := versionHelperRunner(t, "custom git build")
	_, err := CheckCompatibility(context.Background(), runner)
	if !errors.Is(err, ErrGitIncompatible) {
		t.Fatalf("error = %v, want ErrGitIncompatible", err)
	}
}

func versionHelperRunner(t *testing.T, output string) *Runner {
	t.Helper()
	runner := helperRunner(t)
	runner.extraEnv = append(runner.extraEnv, "DIFFBEACON_TEST_GIT_VERSION="+output)
	return runner
}
