package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"runtime/debug"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/andrespistoni/diffbeacon/internal/app"
	diffpkg "github.com/andrespistoni/diffbeacon/internal/diff"
	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/highlight"
	"github.com/andrespistoni/diffbeacon/internal/sanitize"
	"github.com/andrespistoni/diffbeacon/internal/ui"
	watchpkg "github.com/andrespistoni/diffbeacon/internal/watch"
)

// version is injected by release builds. When it is empty, effectiveVersion
// falls back to the module version embedded by `go install module@version`.
var version string

var pseudoVersionPattern = regexp.MustCompile(`^v?0\.0\.0-\d{14}-[0-9a-f]{12}(?:\+.*)?$`)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, gitpkg.NewRunner("git")))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, runner *gitpkg.Runner) int {
	if len(args) == 1 && args[0] == "--version" {
		fmt.Fprintf(stdout, "diffbeacon %s\n", effectiveVersion())
		return 0
	}
	if len(args) > 1 {
		fmt.Fprintln(stderr, "usage: diffbeacon [path]\n       diffbeacon --version")
		return 2
	}
	if _, err := gitpkg.CheckCompatibility(ctx, runner); err != nil {
		fmt.Fprintf(stderr, "diffbeacon: %s\n", sanitize.Text(err.Error()))
		return 1
	}

	path := "."
	if len(args) == 1 {
		path = args[0]
	}
	repository, err := gitpkg.Discover(ctx, runner, path)
	if err != nil {
		fmt.Fprintf(stderr, "diffbeacon: %s\n", sanitize.Text(err.Error()))
		return 1
	}
	watcher, err := watchpkg.New(repository.Root, repository.GitDir, watchpkg.DefaultConfig())
	if err != nil {
		fmt.Fprintf(stderr, "diffbeacon: initialize watcher: %s\n", sanitize.Text(err.Error()))
		return 1
	}

	runtimeCtx, cancel := context.WithCancel(ctx)
	coordinator := app.NewCoordinator(app.GitLoader{
		Runner: runner, Repository: repository,
		DiffLimits: diffpkg.DefaultLimits(), HighlightLimits: highlight.DefaultLimits(),
	})
	defer func() {
		cancel()
		coordinator.Close()
	}()

	model := ui.New(repository.Root, coordinator, watcher.Run(runtimeCtx))
	program := tea.NewProgram(model, tea.WithContext(runtimeCtx), tea.WithInput(stdin), tea.WithOutput(stdout), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(stderr, "diffbeacon: TUI failed: %s\n", sanitize.Text(err.Error()))
		return 1
	}
	return 0
}

func effectiveVersion() string {
	if version != "" {
		return strings.TrimPrefix(version, "v")
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if value := strings.TrimSpace(info.Main.Version); value != "" && value != "(devel)" {
			if !strings.Contains(value, "+dirty") && !pseudoVersionPattern.MatchString(value) {
				return strings.TrimPrefix(value, "v")
			}
		}
	}
	return "development"
}
