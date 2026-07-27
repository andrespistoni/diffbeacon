package git

import (
	"errors"
	"fmt"
	"regexp"
)

var (
	// ErrGitCommandNotReadOnly means an invocation was rejected before a process
	// could start because it is outside DiffBeacon's explicit query allowlist.
	ErrGitCommandNotReadOnly = errors.New("Git invocation is not an approved read-only query")
	objectIDPattern          = regexp.MustCompile(`^[0-9a-fA-F]{40}(?:[0-9a-fA-F]{24})?$`)
	unifiedOptionPattern     = regexp.MustCompile(`^--unified=[0-9]+$`)
)

// validateReadOnlyGitInvocation is the production process boundary. Keep this
// allowlist exact: adding a caller is insufficient to permit a new Git shape.
func validateReadOnlyGitInvocation(args []string) error {
	if len(args) == 1 && args[0] == "--version" {
		return nil
	}

	original := append([]string(nil), args...)
	literalPaths := containsArgument(original, "--literal-pathspecs")
	args, ok := consumeReadOnlyGlobalOptions(args)
	if !ok || len(args) == 0 {
		return rejectedGitInvocation(original)
	}

	allowed := false
	switch args[0] {
	case "rev-parse":
		allowed = equalArguments(args[1:], "--is-bare-repository") ||
			equalArguments(args[1:], "--show-toplevel", "--absolute-git-dir")
	case "status":
		allowed = equalArguments(args[1:],
			"--porcelain=v2", "-z", "--branch", "--no-ahead-behind",
			"--untracked-files=all", "--ignored=no", "--ignore-submodules=none")
	case "ls-tree":
		allowed = literalPaths && len(args) == 5 && args[1] == "-z" && args[2] == "HEAD" && args[3] == "--" && args[4] != ""
	case "ls-files":
		allowed = literalPaths && len(args) == 5 && args[1] == "--stage" && args[2] == "-z" && args[3] == "--" && args[4] != ""
	case "cat-file":
		allowed = len(args) == 3 && (args[1] == "blob" || args[1] == "-s") && objectIDPattern.MatchString(args[2])
	case "diff":
		allowed = allowedDiffArguments(args[1:])
	}
	if !allowed {
		return rejectedGitInvocation(original)
	}
	return nil
}

func allowedDiffArguments(args []string) bool {
	if len(args) != 10 || !equalArguments(args[:5],
		"--patch", "--no-ext-diff", "--no-textconv", "--no-color",
		"--diff-algorithm=myers",
	) || !unifiedOptionPattern.MatchString(args[5]) {
		return false
	}
	return equalArguments(args[6:], "--no-index", "--", "before", "after")
}

func containsArgument(args []string, wanted string) bool {
	for _, argument := range args {
		if argument == wanted {
			return true
		}
	}
	return false
}

func consumeReadOnlyGlobalOptions(args []string) ([]string, bool) {
	seenGlobal := map[string]bool{}
	for len(args) > 0 && (args[0] == "--no-optional-locks" || args[0] == "--literal-pathspecs") {
		if seenGlobal[args[0]] {
			return nil, false
		}
		seenGlobal[args[0]] = true
		args = args[1:]
	}
	allowedConfig := map[string]bool{
		"color.status=false":    true,
		"core.quotepath=false":  true,
		"status.renames=copies": true,
	}
	for len(args) >= 2 && args[0] == "-c" {
		if !allowedConfig[args[1]] {
			return nil, false
		}
		args = args[2:]
	}
	return args, true
}

func equalArguments(got []string, want ...string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range want {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func rejectedGitInvocation(args []string) error {
	return fmt.Errorf("%w: %q", ErrGitCommandNotReadOnly, args)
}
