package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	diffpkg "diffbeacon/internal/diff"
)

var errUnsafeWorkingTreeEntry = errors.New("working-tree entry is not a safe regular file")

const defaultPatchTimeout = 5 * time.Second

// LoadContent obtains exactly the comparison represented by change.Identity().
func LoadContent(ctx context.Context, runner *Runner, repository Repository, change Change) (diffpkg.Document, error) {
	return LoadContentWithLimits(ctx, runner, repository, change, diffpkg.DefaultLimits())
}

// LoadContentWithLimits keeps changes-only loading independent from the
// smaller full-file content budget.
func LoadContentWithLimits(ctx context.Context, runner *Runner, repository Repository, change Change, limits diffpkg.Limits) (diffpkg.Document, error) {
	limits = limits.Normalized()
	base := diffpkg.Document{Path: change.Path, OldPath: change.OldPath}
	if change.Submodule.IsSubmodule {
		base.Kind = diffpkg.ContentSubmodule
		base.Capability = diffpkg.Capability{HunksReason: "submodule content and partial hunks are not supported"}
		base.Metadata.Summary = submoduleSummary(change.Submodule)
		return base, nil
	}
	if change.Status == StatusTypeChanged {
		base.Kind = diffpkg.ContentTypeChange
		base.Capability = diffpkg.Capability{HunksReason: "type changes are represented as metadata; textual hunks are disabled"}
		base.Metadata.Summary = "Git reports a filesystem type change"
		return base, nil
	}
	if change.Status == StatusUnmerged {
		return loadConflict(ctx, repository, change, base, limits.MaxContentBytes)
	}

	var before, after loadedContent
	var err error
	switch change.Scope {
	case ScopeStaged:
		if change.Status != StatusAdded {
			path := change.Path
			if change.OldPath != "" {
				path = change.OldPath
			}
			before, err = loadTreeBlob(ctx, runner, repository, "HEAD", path, limits.MaxContentBytes)
			if err != nil {
				return base, fmt.Errorf("load staged baseline for %q: %w", change.Path, err)
			}
		}
		if change.Status != StatusDeleted {
			after, err = loadIndexBlob(ctx, runner, repository, change.Path, limits.MaxContentBytes)
			if err != nil {
				return base, fmt.Errorf("load staged index content for %q: %w", change.Path, err)
			}
		}
	case ScopeUnstaged:
		if change.Status != StatusAdded {
			path := change.Path
			if change.OldPath != "" {
				path = change.OldPath
			}
			before, err = loadIndexBlob(ctx, runner, repository, path, limits.MaxContentBytes)
			if err != nil {
				return base, fmt.Errorf("load unstaged index baseline for %q: %w", change.Path, err)
			}
		}
		if change.Status != StatusDeleted {
			after, err = loadWorkingTree(ctx, repository, change.Path, limits.MaxContentBytes)
			if err != nil {
				return base, fmt.Errorf("load unstaged working-tree content for %q: %w", change.Path, err)
			}
		}
	case ScopeUntracked:
		after, err = loadWorkingTree(ctx, repository, change.Path, limits.MaxContentBytes)
		if err != nil {
			return base, fmt.Errorf("load untracked content for %q: %w", change.Path, err)
		}
	default:
		return base, fmt.Errorf("unsupported content scope %s", change.Scope)
	}

	if change.Scope == ScopeUntracked {
		return makeDocument(base, before, after, nil, "", true, limits), nil
	}
	patch, patchReason, err := loadPatch(ctx, runner, repository, before, after, limits)
	if err != nil {
		return base, fmt.Errorf("load %s patch for %q: %w", change.Scope, change.Path, err)
	}
	return makeDocument(base, before, after, patch, patchReason, false, limits), nil
}

type loadedContent struct {
	data      []byte
	present   bool
	truncated bool
	size      int64
	mode      string
	oid       string
	path      string
}

func makeDocument(base diffpkg.Document, before, after loadedContent, patch []byte, patchReason string, wholeFile bool, limits diffpkg.Limits) diffpkg.Document {
	base.BeforePresent, base.AfterPresent = before.present, after.present
	base.Metadata.BeforeBytes, base.Metadata.AfterBytes = before.size, after.size
	base.Metadata.BeforeMode, base.Metadata.AfterMode = before.mode, after.mode
	if isBinary(before.data) || isBinary(after.data) || patchIsBinary(patch) {
		base.Kind = diffpkg.ContentBinary
		base.Capability = diffpkg.Capability{HunksReason: "binary content is summarized; textual hunks are disabled"}
		base.Metadata.Summary = "Git content contains NUL bytes or invalid UTF-8"
		return base
	}
	if patchReason != "" {
		base.Kind = diffpkg.ContentLimited
		base.Capability = diffpkg.Capability{HunksReason: patchReason}
		base.Metadata.Summary = base.Capability.HunksReason
		return base
	}
	if wholeFile && (before.truncated || after.truncated) {
		base.Kind = diffpkg.ContentLimited
		base.Capability = diffpkg.Capability{FullFileReason: fmt.Sprintf("content exceeds %d-byte full-file limit", limits.MaxContentBytes)}
		base.Metadata.Summary = base.Capability.FullFileReason
		return base
	}
	base.Kind = diffpkg.ContentText
	base.Patch = string(patch)
	base.Capability = diffpkg.Capability{FullFile: !before.truncated && !after.truncated, Hunks: !wholeFile}
	if base.Capability.FullFile {
		base.Before, base.After = string(before.data), string(after.data)
	} else {
		base.Capability.FullFileReason = fmt.Sprintf("content exceeds %d-byte full-file limit", limits.MaxContentBytes)
	}
	if wholeFile {
		base.Before, base.After = string(before.data), string(after.data)
		base.Capability.HunksReason = "untracked content is always shown as a complete file"
	}
	return base
}

func loadConflict(ctx context.Context, repository Repository, change Change, base diffpkg.Document, contentLimit int) (diffpkg.Document, error) {
	base.Kind = diffpkg.ContentConflict
	base.Capability = diffpkg.Capability{FullFile: true, HunksReason: "conflicted entries do not support partial hunks"}
	base.Metadata.Summary = "unmerged: " + change.Conflict.String()
	content, err := loadWorkingTree(ctx, repository, change.Path, contentLimit)
	if errors.Is(err, os.ErrNotExist) || errors.Is(err, errUnsafeWorkingTreeEntry) {
		return base, nil
	}
	if err != nil {
		return base, fmt.Errorf("load conflicted working-tree content for %q: %w", change.Path, err)
	}
	base.AfterPresent = content.present
	base.Metadata.AfterBytes = content.size
	if content.truncated {
		base.Kind = diffpkg.ContentLimited
		base.Capability.FullFile = false
		base.Capability.FullFileReason = fmt.Sprintf("conflicted content exceeds %d-byte full-file limit", contentLimit)
		return base, nil
	}
	if isBinary(content.data) {
		base.Kind = diffpkg.ContentBinary
		base.Capability.FullFile = false
		base.Capability.HunksReason = "binary conflicted content is summarized; textual hunks are disabled"
		return base, nil
	}
	base.After = string(content.data)
	return base, nil
}

func loadTreeBlob(ctx context.Context, runner *Runner, repository Repository, tree, path string, contentLimit int) (loadedContent, error) {
	output, err := runner.Run(ctx, repository.Root, "--no-optional-locks", "--literal-pathspecs", "ls-tree", "-z", tree, "--", path)
	if err != nil {
		return loadedContent{}, err
	}
	if len(output) == 0 {
		return loadedContent{}, nil
	}
	oid, mode, found, err := parseObjectRecord(output, path, false)
	if err != nil || !found {
		return loadedContent{}, err
	}
	content, err := loadObject(ctx, runner, repository, oid, contentLimit)
	content.mode = mode
	content.oid = oid
	return content, err
}

func loadIndexBlob(ctx context.Context, runner *Runner, repository Repository, path string, contentLimit int) (loadedContent, error) {
	output, err := runner.Run(ctx, repository.Root, "--no-optional-locks", "--literal-pathspecs", "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		return loadedContent{}, err
	}
	oid, mode, found, err := parseObjectRecord(output, path, true)
	if err != nil || !found {
		return loadedContent{}, err
	}
	content, err := loadObject(ctx, runner, repository, oid, contentLimit)
	content.mode = mode
	content.oid = oid
	return content, err
}

func parseObjectRecord(output []byte, wantedPath string, index bool) (string, string, bool, error) {
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		metadata, path, ok := bytes.Cut(record, []byte{'\t'})
		if !ok || string(path) != wantedPath {
			continue
		}
		fields := bytes.Fields(metadata)
		if len(fields) != 3 {
			return "", "", false, fmt.Errorf("malformed Git object record")
		}
		if index && string(fields[2]) != "0" {
			continue
		}
		oidIndex := 2
		if index {
			oidIndex = 1
		}
		return string(fields[oidIndex]), string(fields[0]), true, nil
	}
	return "", "", false, nil
}

func loadObject(ctx context.Context, runner *Runner, repository Repository, oid string, contentLimit int) (loadedContent, error) {
	available, err := objectAvailableLocally(repository, oid)
	if err != nil {
		return loadedContent{}, fmt.Errorf("inspect local Git object: %w", err)
	}
	if !available {
		return loadedContent{}, ErrObjectNotLocal
	}
	limit := contentLimit
	if limit <= 0 {
		limit = diffpkg.DefaultMaxContentBytes
	}
	output, truncated, err := runner.RunLimited(ctx, repository.Root, limit+1, "--no-optional-locks", "cat-file", "blob", oid)
	if err != nil {
		return loadedContent{}, err
	}
	size := int64(len(output))
	if len(output) > limit || truncated {
		output = nil
		truncated = true
		sizeOutput, sizeErr := runner.Run(ctx, repository.Root, "--no-optional-locks", "cat-file", "-s", oid)
		if sizeErr != nil {
			return loadedContent{}, sizeErr
		}
		size, sizeErr = strconv.ParseInt(strings.TrimSpace(string(sizeOutput)), 10, 64)
		if sizeErr != nil || size < 0 {
			return loadedContent{}, fmt.Errorf("parse Git object size")
		}
	}
	return loadedContent{data: output, present: true, truncated: truncated, size: size}, nil
}

func loadWorkingTree(ctx context.Context, repository Repository, path string, contentLimit int) (loadedContent, error) {
	if err := ctx.Err(); err != nil {
		return loadedContent{}, err
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if filepath.IsAbs(clean) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return loadedContent{}, errUnsafeWorkingTreeEntry
	}
	fullPath := filepath.Join(repository.Root, clean)
	info, err := os.Lstat(fullPath)
	if err != nil {
		return loadedContent{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return loadedContent{}, errUnsafeWorkingTreeEntry
	}
	root, err := os.OpenRoot(repository.Root)
	if err != nil {
		return loadedContent{}, err
	}
	defer root.Close()
	file, err := root.Open(filepath.ToSlash(clean))
	if err != nil {
		return loadedContent{}, err
	}
	defer file.Close()
	limit := int64(contentLimit)
	if limit <= 0 {
		limit = diffpkg.DefaultMaxContentBytes
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return loadedContent{}, err
	}
	if int64(len(data)) > limit {
		return loadedContent{present: true, truncated: true, size: info.Size(), path: clean}, nil
	}
	mode := "100644"
	if info.Mode().Perm()&0o111 != 0 {
		mode = "100755"
	}
	return loadedContent{data: data, present: true, size: info.Size(), mode: mode, path: clean}, nil
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data)
}

func loadPatch(ctx context.Context, runner *Runner, repository Repository, before, after loadedContent, limits diffpkg.Limits) ([]byte, string, error) {
	if before.size > int64(limits.MaxDiffInputBytes) || after.size > int64(limits.MaxDiffInputBytes) {
		return nil, fmt.Sprintf("content exceeds %d-byte changes input limit", limits.MaxDiffInputBytes), nil
	}
	temporary, err := os.MkdirTemp("", "diffbeacon-diff-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(temporary)
	if err := materializeComparisonContent(ctx, runner, repository, before, filepath.Join(temporary, "before"), limits.MaxDiffInputBytes); err != nil {
		return nil, "", fmt.Errorf("materialize before content: %w", err)
	}
	if err := materializeComparisonContent(ctx, runner, repository, after, filepath.Join(temporary, "after"), limits.MaxDiffInputBytes); err != nil {
		return nil, "", fmt.Errorf("materialize after content: %w", err)
	}

	patchCtx, cancel := context.WithTimeout(ctx, defaultPatchTimeout)
	defer cancel()
	args := []string{
		"--no-optional-locks", "diff",
		"--patch", "--no-ext-diff", "--no-textconv", "--no-color",
		"--diff-algorithm=myers", fmt.Sprintf("--unified=%d", max(0, limits.ContextLines)),
		"--no-index", "--", "before", "after",
	}
	output, truncated, err := runner.RunDiffLimited(patchCtx, temporary, limits.MaxPatchBytes+1, args...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
			return nil, "", fmt.Errorf("Git diff exceeded %s", defaultPatchTimeout)
		}
		return nil, "", err
	}
	if len(output) > limits.MaxPatchBytes || truncated {
		return nil, fmt.Sprintf("Git patch exceeds %d-byte changes limit", limits.MaxPatchBytes), nil
	}
	return output, "", nil
}

func materializeComparisonContent(ctx context.Context, runner *Runner, repository Repository, content loadedContent, destination string, limit int) error {
	if !content.present || !content.truncated {
		return os.WriteFile(destination, content.data, 0o600)
	}
	if content.oid != "" {
		output, truncated, err := runner.RunLimited(ctx, repository.Root, limit+1, "--no-optional-locks", "cat-file", "blob", content.oid)
		if err != nil {
			return err
		}
		if truncated || len(output) > limit {
			return fmt.Errorf("Git object exceeded %d-byte changes input limit", limit)
		}
		return os.WriteFile(destination, output, 0o600)
	}
	if content.path == "" {
		return errors.New("large content has no safe source")
	}
	root, err := os.OpenRoot(repository.Root)
	if err != nil {
		return err
	}
	defer root.Close()
	source, err := root.Open(filepath.ToSlash(content.path))
	if err != nil {
		return err
	}
	defer source.Close()
	target, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(target, io.LimitReader(source, int64(limit)+1))
	closeErr := target.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > int64(limit) {
		return fmt.Errorf("working-tree content exceeded %d-byte changes input limit", limit)
	}
	return nil
}

func patchIsBinary(patch []byte) bool {
	return !utf8.Valid(patch) || bytes.Contains(patch, []byte("\nBinary files ")) || bytes.Contains(patch, []byte("\nGIT binary patch\n"))
}

func submoduleSummary(state SubmoduleState) string {
	parts := make([]string, 0, 3)
	if state.CommitChanged {
		parts = append(parts, "commit changed")
	}
	if state.TrackedModified {
		parts = append(parts, "tracked content modified")
	}
	if state.UntrackedPresent {
		parts = append(parts, "untracked content present")
	}
	if len(parts) == 0 {
		return "submodule"
	}
	return "submodule: " + strings.Join(parts, ", ")
}
