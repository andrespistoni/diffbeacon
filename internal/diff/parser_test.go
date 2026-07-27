package diff

import "testing"

func TestScopeModelPreservesNumbersAndMarks(t *testing.T) {
	document := NewTextDocument("sample.txt", "", []byte("one\ntwo\nthree\n"), true, []byte("one\nchanged\nthree\nfour\n"), true)
	document.Patch = "@@ -1,3 +1,4 @@\n one\n-two\n+changed\n three\n+four\n"
	model := Build(document, DefaultLimits())
	if model.Degraded {
		t.Fatalf("Build() degraded: %s", model.Reason)
	}
	if len(model.Operations) != 5 {
		t.Fatalf("operations = %#v", model.Operations)
	}
	deleted, added := model.Operations[1], model.Operations[2]
	if deleted.Kind != OperationDelete || deleted.Before.Number != 2 || added.Kind != OperationAdd || added.After.Number != 2 {
		t.Fatalf("replacement operations = %#v, %#v", deleted, added)
	}
	if model.Operations[4].After.Number != 4 {
		t.Fatalf("last after number = %d", model.Operations[4].After.Number)
	}
}

func TestContentANSIIsNeutralized(t *testing.T) {
	model := Build(NewTextDocument("unsafe.txt", "", nil, false, []byte("ok\x1b[31mred\u009b32mgreen\n"), true), DefaultLimits())
	if got := model.Operations[0].After.Text; got != "ok\\x1b[31mred\\u009b32mgreen" {
		t.Fatalf("safe text = %q", got)
	}
}

func TestContentFinalNewlineDifferenceIsModeled(t *testing.T) {
	model := Build(NewTextDocument("newline.txt", "", []byte("same"), true, []byte("same\n"), true), DefaultLimits())
	if len(model.Operations) != 2 || model.Operations[0].Kind != OperationDelete || model.Operations[1].Kind != OperationAdd {
		t.Fatalf("operations = %#v, want newline-only replacement", model.Operations)
	}
	if model.Operations[0].Before.Terminated || !model.Operations[1].After.Terminated {
		t.Fatalf("newline markers = %#v, %#v", model.Operations[0].Before, model.Operations[1].After)
	}
}

func TestGitPatchNoNewlineMarkerPreservesTermination(t *testing.T) {
	document := Document{
		Path:          "newline.txt",
		BeforePresent: true,
		AfterPresent:  true,
		Kind:          ContentText,
		Capability:    Capability{Hunks: true, FullFileReason: "full file intentionally unavailable"},
		Patch:         "@@ -1 +1 @@\n-old\n\\ No newline at end of file\n+new\n\\ No newline at end of file\n",
	}
	model := Build(document, DefaultLimits())
	if model.Degraded || len(model.Operations) != 2 {
		t.Fatalf("model = %#v", model)
	}
	if model.Operations[0].Before.Terminated || model.Operations[1].After.Terminated {
		t.Fatalf("newline markers = %#v", model.Operations)
	}
}
