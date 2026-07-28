package e2e

import (
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/andrespistoni/diffbeacon/internal/testrepo"
)

func TestBinaryFunctionalFlowMakesNoProxyObservedNetworkAttemptOrTelemetryFile(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("start network observer: %v", err)
	}
	defer listener.Close()
	attempts := make(chan net.Conn, 1)
	go func() {
		connection, err := listener.Accept()
		if err == nil {
			attempts <- connection
		}
	}()

	fixture := testrepo.New(t)
	const contentMarker = "PRIVATE-CONTENT-MUST-NOT-BE-DIAGNOSTIC-LOGGED"
	fixture.Write("private.txt", "base\n")
	fixture.CommitAll("base")
	fixture.Write("private.txt", contentMarker+"\n")
	homeBefore := workingTreeSnapshot(t, fixture.Home)
	shim := testrepo.NewGitShim(t)
	proxy := "http://" + listener.Addr().String()
	session := startBinary(t, fixture, shim, map[string]string{
		"HTTP_PROXY": proxy, "HTTPS_PROXY": proxy, "ALL_PROXY": proxy,
		"http_proxy": proxy, "https_proxy": proxy, "all_proxy": proxy,
		"NO_PROXY": "", "no_proxy": "",
	})
	waitForOutputContains(t, session, "private.txt")
	waitForPathDetailLoad(t, shim, "private.txt")
	statusCalls := countGitCommand(shim.Records(t), "status")
	session.send(t, "r")
	waitFor(t, "manual refresh", func() bool { return countGitCommand(shim.Records(t), "status") > statusCalls })
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	session.send(t, "suSU?vf[]1234")
	waitForProcessQuiescence(t, shim, 350*time.Millisecond)
	session.quit(t)

	select {
	case connection := <-attempts:
		_ = connection.Close()
		t.Fatal("DiffBeacon connected to the proxy network observer")
	case <-time.After(200 * time.Millisecond):
	}
	if after := workingTreeSnapshot(t, fixture.Home); !reflect.DeepEqual(after, homeBefore) {
		t.Fatalf("application created a home/config/telemetry file:\n before=%#v\n after=%#v", homeBefore, after)
	}
	if strings.Contains(session.stderr.String(), contentMarker) {
		t.Fatalf("diagnostic stderr exposed full file content: %q", session.stderr.String())
	}
	assertSafeProcessRecords(t, shim.Records(t))
}
