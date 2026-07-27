package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"syscall"
)

var (
	ErrGitIncompatible = errors.New("Git version is not supported")
	gitVersionPattern  = regexp.MustCompile(`(?i)^git version (\d+)\.(\d+)(?:\.(\d+))?`)
)

// MinimumVersion is the oldest Git release exercised by the compatibility
// matrix. It is a validation floor, not a claim that older releases cannot run.
var MinimumVersion = Version{Major: 2, Minor: 31, Patch: 0}

type Version struct {
	Major int
	Minor int
	Patch int
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) AtLeast(minimum Version) bool {
	if v.Major != minimum.Major {
		return v.Major > minimum.Major
	}
	if v.Minor != minimum.Minor {
		return v.Minor > minimum.Minor
	}
	return v.Patch >= minimum.Patch
}

// CheckCompatibility runs before repository discovery so unsupported or
// unparseable Git installations fail explicitly before the TUI starts.
func CheckCompatibility(ctx context.Context, runner *Runner) (Version, error) {
	output, err := runner.Run(ctx, ".", "--version")
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.EACCES) {
			return Version{}, fmt.Errorf("%w: %v", ErrGitUnavailable, err)
		}
		return Version{}, fmt.Errorf("check Git version: %w", err)
	}
	version, err := ParseVersion(string(output))
	if err != nil {
		return Version{}, fmt.Errorf("%w: %v", ErrGitIncompatible, err)
	}
	if !version.AtLeast(MinimumVersion) {
		return version, fmt.Errorf("%w: found %s, require at least %s", ErrGitIncompatible, version, MinimumVersion)
	}
	return version, nil
}

func ParseVersion(value string) (Version, error) {
	match := gitVersionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return Version{}, fmt.Errorf("unrecognized output %q", strings.TrimSpace(value))
	}
	parts := [3]int{}
	for index := range parts {
		if match[index+1] == "" {
			continue
		}
		parsed, err := strconv.Atoi(match[index+1])
		if err != nil {
			return Version{}, fmt.Errorf("parse component %q: %w", match[index+1], err)
		}
		parts[index] = parsed
	}
	return Version{Major: parts[0], Minor: parts[1], Patch: parts[2]}, nil
}
