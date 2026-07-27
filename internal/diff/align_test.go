package diff

import (
	"os"
	"testing"
)

func TestContentAlignmentDoesNotShiftInsertionsOrDeletions(t *testing.T) {
	model := Build(NewTextDocument("align.txt", "", []byte("a\nb\nc\n"), true, []byte("a\nx\ny\nc\n"), true), DefaultLimits())
	want, err := os.ReadFile("testdata/alignment.golden")
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if got := formatModel(model); got != string(want) {
		t.Fatalf("operation golden mismatch:\n%s\nwant:\n%s", got, want)
	}
	if len(model.FullRows) != 4 {
		t.Fatalf("rows = %#v", model.FullRows)
	}
	if row := model.FullRows[1]; row.Kind != RowChanged || row.Before.Text != "b" || row.After.Text != "x" {
		t.Fatalf("replacement row = %#v", row)
	}
	if row := model.FullRows[2]; row.Kind != RowAdded || row.Before != nil || row.After.Text != "y" {
		t.Fatalf("insertion row = %#v", row)
	}
	if row := model.FullRows[3]; row.Kind != RowEqual || row.Before.Text != "c" || row.After.Text != "c" {
		t.Fatalf("following row shifted = %#v", row)
	}
}

func TestContentModelIsDeterministic(t *testing.T) {
	document := NewTextDocument("same.txt", "", []byte("a\nb\n"), true, []byte("b\na\n"), true)
	left := Build(document, DefaultLimits())
	right := Build(document, DefaultLimits())
	if formatModel(left) != formatModel(right) {
		t.Fatalf("models differ:\n%s\n%s", formatModel(left), formatModel(right))
	}
}

func formatModel(model Model) string {
	result := ""
	for _, operation := range model.Operations {
		result += string(rune('0' + operation.Kind))
		if operation.Before != nil {
			result += operation.Before.Text
		}
		result += "/"
		if operation.After != nil {
			result += operation.After.Text
		}
		result += "\n"
	}
	return result
}
