package e2e

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestStartupDiagnosticNeutralizesOSC52AndCSI(t *testing.T) {
	hostile := filepath.Join(t.TempDir(), "osc\x1b]52;c;clipboard\x07-csi\x1b[31m-c1\u009b32m")
	command := exec.Command(builtBinary(t), hostile)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("diffbeacon error = %v, stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	got := stderr.String()
	for _, control := range []rune{'\x1b', '\x07', '\u009b'} {
		if strings.ContainsRune(got, control) {
			t.Fatalf("startup stderr retained U+%04X: %q", control, got)
		}
	}
	for _, visible := range []string{`\x1b]52`, `\x07`, `\x1b[31m`, `\u009b32m`} {
		if !strings.Contains(got, visible) {
			t.Fatalf("startup stderr = %q, want %q", got, visible)
		}
	}
	if stdout.Len() != 0 {
		t.Fatalf("startup stdout = %q, want empty", stdout.String())
	}
}

func TestBinaryRestoresAlternateScreenAndNeutralizesContentControls(t *testing.T) {
	fixture := testrepo.New(t)
	const hostileOSC = "\x1b]52;c;payload\x07"
	fixture.Write("ansi.txt", "before "+hostileOSC+" after\n")
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, `\x1b]52;c;payload\x07`)
	session.quit(t)

	output := session.stdout.String()
	assertAlternateScreenRestored(t, output)
	if strings.Contains(output, hostileOSC) {
		t.Fatal("hostile OSC sequence was emitted as a terminal control")
	}
	assertSafeProcessRecords(t, shim.Records(t))
}

func TestBinaryRestoresAlternateScreenAfterInterrupt(t *testing.T) {
	fixture := testrepo.New(t)
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	waitForOutputContains(t, session, "No changes in all")
	if err := session.command.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("interrupt diffbeacon: %v", err)
	}
	session.wait(t, true)
	assertAlternateScreenRestored(t, session.stdout.String())
	assertSafeProcessRecords(t, shim.Records(t))
}

func assertAlternateScreenRestored(t *testing.T, output string) {
	t.Helper()
	enter := strings.Index(output, enterAltScreen)
	exit := strings.LastIndex(output, exitAltScreen)
	if enter < 0 || exit <= enter {
		t.Fatalf("alternate screen was not restored in order: %q", output)
	}
}
