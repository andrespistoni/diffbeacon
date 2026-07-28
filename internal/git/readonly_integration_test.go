package git_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitpkg "github.com/andrespistoni/diffbeacon/internal/git"
	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestPromisorContentQueriesFailLocallyWithoutTransportOrMutation(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("custom transport sentinel is Unix-only")
	}
	fixture := testrepo.NewMissingPromisorBlobs(t)
	repositoryFixture := fixture.Repository
	marker := filepath.Join(t.TempDir(), "transport-started")
	helperDirectory := t.TempDir()
	helper := filepath.Join(helperDirectory, "git-remote-probe")
	script := "#!/bin/sh\nprintf started > '" + strings.ReplaceAll(marker, "'", "'\\''") + "'\nexit 97\n"
	if err := os.WriteFile(helper, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", helperDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
	repositoryFixture.ExternalGit("config", "remote.origin.url", "probe::instrumented")
	repositoryFixture.ExternalGit("config", "protocol.allow", "always")
	repositoryFixture.ExternalGit("config", "protocol.probe.allow", "always")
	before := repositoryFixture.Snapshot()

	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, repositoryFixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []gitpkg.ChangeIdentity{
		{Path: fixture.IndexPath, Scope: gitpkg.ScopeUnstaged},
		{Path: fixture.TreePath, Scope: gitpkg.ScopeStaged},
	} {
		change, ok := findChange(snapshot, identity)
		if !ok {
			t.Fatalf("missing promisor change %#v in %#v", identity, snapshot.Changes)
		}
		if _, err := gitpkg.LoadContent(context.Background(), runner, repository, change); err == nil {
			t.Fatalf("LoadContent(%#v) succeeded with an absent promised blob", identity)
		}
	}
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("promisor transport process started: %v", err)
	}
	if after := repositoryFixture.Snapshot(); before != after {
		t.Fatalf("promisor queries mutated repository\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestReadOnlyRepositoryQueriesPreserveEverySurface(t *testing.T) {
	fixture := testrepo.New(t)
	for _, path := range []string{"tracked.txt", ":(glob)*.txt", "*.txt"} {
		fixture.Write(path, "base\n")
	}
	fixture.CommitAll("base")
	fixture.Write("tracked.txt", "working\n")
	fixture.Write(":(glob)*.txt", "literal glob path\n")
	fixture.ExternalGit("add", "--", ":(glob)*.txt")
	fixture.Write("*.txt", "literal star path\n")
	before := fixture.Snapshot()

	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range snapshot.Changes {
		if _, err := gitpkg.LoadContent(context.Background(), runner, repository, change); err != nil {
			t.Fatalf("LoadContent(%q, %s): %v", change.Path, change.Scope, err)
		}
	}
	after := fixture.Snapshot()
	if before != after {
		t.Fatalf("read-only queries changed repository\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestExternalIndexChangesAreObservedWithoutCompensation(t *testing.T) {
	fixture := testrepo.New(t)
	fixture.Write("external.txt", "base\n")
	fixture.CommitAll("base")
	fixture.Write("external.txt", "changed\n")
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil || before.Count(gitpkg.ScopeUnstaged) != 1 {
		t.Fatalf("initial snapshot = %#v, %v", before, err)
	}
	fixture.ExternalGit("add", "--", "external.txt")
	externalState := fixture.Snapshot()
	after, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil || after.Count(gitpkg.ScopeStaged) != 1 {
		t.Fatalf("externally staged snapshot = %#v, %v", after, err)
	}
	if got := fixture.Snapshot(); got != externalState {
		t.Fatal("query compensated for or altered the external index change")
	}
}

func TestExternalStageAndUnstageWithoutHEADAreObservedWithoutCompensation(t *testing.T) {
	t.Run("file", func(t *testing.T) {
		fixture := testrepo.New(t)
		fixture.Write("file.txt", "external file\n")
		runner, repository := discoverFixture(t, fixture)

		fixture.ExternalGit("add", "--", "file.txt")
		assertObservedWithoutCompensation(t, runner, repository, fixture, gitpkg.ChangeIdentity{Path: "file.txt", Scope: gitpkg.ScopeStaged})
		fixture.ExternalGit("rm", "--cached", "--", "file.txt")
		assertObservedWithoutCompensation(t, runner, repository, fixture, gitpkg.ChangeIdentity{Path: "file.txt", Scope: gitpkg.ScopeUntracked})
	})

	t.Run("hunk", func(t *testing.T) {
		fixture := testrepo.New(t)
		fixture.Write("hunk.txt", "first\nsecond\n")
		runner, repository := discoverFixture(t, fixture)
		patch := "diff --git a/hunk.txt b/hunk.txt\nnew file mode 100644\n--- /dev/null\n+++ b/hunk.txt\n@@ -0,0 +1 @@\n+first\n"

		fixture.ExternalGitInput(patch, "apply", "--cached")
		assertObservedWithoutCompensation(t, runner, repository, fixture,
			gitpkg.ChangeIdentity{Path: "hunk.txt", Scope: gitpkg.ScopeStaged},
			gitpkg.ChangeIdentity{Path: "hunk.txt", Scope: gitpkg.ScopeUnstaged},
		)
		fixture.ExternalGitInput(patch, "apply", "--cached", "--reverse")
		assertObservedWithoutCompensation(t, runner, repository, fixture, gitpkg.ChangeIdentity{Path: "hunk.txt", Scope: gitpkg.ScopeUntracked})
	})

	t.Run("set", func(t *testing.T) {
		fixture := testrepo.New(t)
		fixture.Write("one.txt", "one\n")
		fixture.Write("two.txt", "two\n")
		runner, repository := discoverFixture(t, fixture)

		fixture.ExternalGit("add", "-A")
		assertObservedWithoutCompensation(t, runner, repository, fixture,
			gitpkg.ChangeIdentity{Path: "one.txt", Scope: gitpkg.ScopeStaged},
			gitpkg.ChangeIdentity{Path: "two.txt", Scope: gitpkg.ScopeStaged},
		)
		fixture.ExternalGit("rm", "--cached", "-r", "--", ".")
		assertObservedWithoutCompensation(t, runner, repository, fixture,
			gitpkg.ChangeIdentity{Path: "one.txt", Scope: gitpkg.ScopeUntracked},
			gitpkg.ChangeIdentity{Path: "two.txt", Scope: gitpkg.ScopeUntracked},
		)
	})
}

func discoverFixture(t *testing.T, fixture *testrepo.Repository) (*gitpkg.Runner, gitpkg.Repository) {
	t.Helper()
	runner := gitpkg.NewRunner("git")
	repository, err := gitpkg.Discover(context.Background(), runner, fixture.Path)
	if err != nil {
		t.Fatal(err)
	}
	return runner, repository
}

func assertObservedWithoutCompensation(t *testing.T, runner *gitpkg.Runner, repository gitpkg.Repository, fixture *testrepo.Repository, identities ...gitpkg.ChangeIdentity) {
	t.Helper()
	externalState := fixture.Snapshot()
	snapshot, err := gitpkg.QueryStatus(context.Background(), runner, repository)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range identities {
		if _, ok := findChange(snapshot, identity); !ok {
			t.Errorf("external change %#v was not observed in %#v", identity, snapshot.Changes)
		}
	}
	if got := fixture.Snapshot(); got != externalState {
		t.Fatal("DiffBeacon query compensated for or altered an external index change")
	}
}

func findChange(snapshot gitpkg.Snapshot, identity gitpkg.ChangeIdentity) (gitpkg.Change, bool) {
	for _, change := range snapshot.Changes {
		if change.Identity() == identity {
			return change, true
		}
	}
	return gitpkg.Change{}, false
}
