package e2e

import (
	"testing"
	"time"

	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestSpecialPathsRemainReadOnlyIncludingFormerFVER01Cases(t *testing.T) {
	paths := []string{
		":(glob)*.txt",
		"*.txt",
		"path with spaces.txt",
		"unicodé-文件.txt",
		"line\nbreak.txt",
		"-leading-option.txt",
		"$(touch injected);[literal]*.txt",
	}
	fixture := testrepo.New(t)
	for _, path := range paths {
		fixture.Write(path, "hostile path content\n")
	}
	before := fixture.Snapshot()
	shim := testrepo.NewGitShim(t)
	session := startBinary(t, fixture, shim, nil)
	for _, visible := range []string{":(glob)*.txt", "*.txt", "path with spaces.txt", "unicodé-文件.txt", "-leading-option.txt"} {
		waitForOutputContains(t, session, visible)
	}
	session.send(t, "suSUr?jk[]vf1234")
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	session.quit(t)

	assertRepositorySnapshotEqual(t, before, fixture.Snapshot())
	assertSafeProcessRecords(t, shim.Records(t))
}
