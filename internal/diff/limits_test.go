package diff

import (
	"fmt"
	"strings"
	"testing"
)

func TestFullFileLimitsDoNotDisableAvailableHunks(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		patch  string
		limits Limits
		want   string
	}{
		{"bytes", "12345\n", "abcde\n", "@@ -1 +1 @@\n-12345\n+abcde\n", Limits{MaxContentBytes: 4}, "content exceeds 4-byte full-file limit"},
		{"line", "12345\n", "abcde\n", "@@ -1 +1 @@\n-12345\n+abcde\n", Limits{MaxContentBytes: 100, MaxLineBytes: 4}, "line 1 on before exceeds 4-byte full-file limit"},
		{"lines", "a\nb\n", "a\nc\n", "@@ -1,2 +1,2 @@\n a\n-b\n+c\n", Limits{MaxContentBytes: 100, MaxLineBytes: 100, MaxLines: 3}, "line count exceeds 3-line full-file limit"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			document := NewTextDocument("limit.txt", "", []byte(test.before), true, []byte(test.after), true)
			document.Patch = test.patch
			model := Build(document, test.limits)
			if model.Degraded || model.Document.Capability.FullFile || !model.Document.Capability.Hunks || len(model.Hunks) != 1 {
				t.Fatalf("model = %#v, want changes-only capability", model)
			}
			if model.Document.Capability.FullFileReason != test.want {
				t.Fatalf("full-file reason = %q, want %q", model.Document.Capability.FullFileReason, test.want)
			}
		})
	}
}

func TestPatchLimitsDegradeWithStableReasons(t *testing.T) {
	document := NewTextDocument("limit.txt", "", []byte("a\n"), true, []byte("b\n"), true)
	document.Patch = "@@ -1 +1 @@\n-a\n+b\n"
	model := Build(document, Limits{MaxPatchBytes: 4})
	if !model.Degraded || model.Reason != "Git patch exceeds 4-byte changes limit" {
		t.Fatalf("model = %#v", model)
	}
}

func TestLargeLineCountsNoLongerRequireQuadraticMatrix(t *testing.T) {
	beforeLines := make([]string, 2_000)
	afterLines := make([]string, 2_000)
	for index := range beforeLines {
		beforeLines[index] = fmt.Sprintf("line-%d", index+1)
		afterLines[index] = beforeLines[index]
	}
	afterLines[999] = "changed"
	document := NewTextDocument(
		"large.txt", "",
		[]byte(strings.Join(beforeLines, "\n")+"\n"), true,
		[]byte(strings.Join(afterLines, "\n")+"\n"), true,
	)
	document.Patch = "@@ -1000 +1000 @@\n-line-1000\n+changed\n"
	model := Build(document, DefaultLimits())
	if model.Degraded || len(model.Hunks) != 1 || len(model.FullRows) != 2_000 {
		t.Fatalf("large model = degraded %v reason %q hunks %d rows %d", model.Degraded, model.Reason, len(model.Hunks), len(model.FullRows))
	}
}
