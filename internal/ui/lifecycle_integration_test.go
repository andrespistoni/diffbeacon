package ui

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	enterAltScreen = "\x1b[?1049h"
	exitAltScreen  = "\x1b[?1049l"
)

func TestProgramRestoresTerminalAfterNormalQuit(t *testing.T) {
	var output bytes.Buffer
	program := tea.NewProgram(New("repo", nil, nil),
		tea.WithInput(strings.NewReader("q")), tea.WithOutput(&output),
		tea.WithAltScreen(), tea.WithoutSignalHandler(),
	)
	if _, err := program.Run(); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	assertTerminalRestored(t, output.String())
}

func TestProgramRestoresTerminalAfterControlledKill(t *testing.T) {
	input, inputWriter := io.Pipe()
	defer input.Close()
	defer inputWriter.Close()
	output := &lockedBuffer{}
	program := tea.NewProgram(New("repo", nil, nil),
		tea.WithInput(input), tea.WithOutput(output),
		tea.WithAltScreen(), tea.WithoutSignalHandler(),
	)
	result := make(chan error, 1)
	go func() {
		_, err := program.Run()
		result <- err
	}()
	waitForOutput(t, output, enterAltScreen)
	program.Kill()
	select {
	case err := <-result:
		if !errors.Is(err, tea.ErrProgramKilled) {
			t.Fatalf("Run() error = %v, want ErrProgramKilled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("program did not stop after controlled kill")
	}
	assertTerminalRestored(t, output.String())
}

func assertTerminalRestored(t *testing.T, output string) {
	t.Helper()
	enter := strings.Index(output, enterAltScreen)
	exit := strings.LastIndex(output, exitAltScreen)
	if enter < 0 || exit <= enter {
		t.Fatalf("alternate screen was not restored in order: %q", output)
	}
}

type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(value)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}

func waitForOutput(t *testing.T, output *lockedBuffer, value string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), value) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for output %q; got %q", value, output.String())
}
