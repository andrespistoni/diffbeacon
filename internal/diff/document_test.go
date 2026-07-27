package diff

import "testing"

func TestContentDocumentDistinguishesAbsentAndEmptySides(t *testing.T) {
	document := NewTextDocument("new.txt", "", nil, false, nil, true)
	if document.BeforePresent || !document.AfterPresent {
		t.Fatalf("presence = before %v after %v", document.BeforePresent, document.AfterPresent)
	}
	if document.Kind != ContentText || !document.Capability.Hunks {
		t.Fatalf("document = %#v", document)
	}
}
