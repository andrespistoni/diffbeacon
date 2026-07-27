package diff

// ContentKind identifies how safely loaded content may be represented.
type ContentKind string

const (
	ContentText       ContentKind = "text"
	ContentBinary     ContentKind = "binary"
	ContentConflict   ContentKind = "conflict"
	ContentSubmodule  ContentKind = "submodule"
	ContentTypeChange ContentKind = "type-change"
	ContentLimited    ContentKind = "limited"
)

// Capability makes unsupported partial operations explicit to later layers.
type Capability struct {
	FullFile       bool
	Hunks          bool
	FullFileReason string
	HunksReason    string
}

// Metadata contains bounded facts suitable for a special-content summary.
type Metadata struct {
	BeforeBytes int64
	AfterBytes  int64
	BeforeMode  string
	AfterMode   string
	Summary     string
}

// Document is a scope-specific before/after comparison. Presence is separate
// from text so an absent side is distinguishable from an empty file.
type Document struct {
	Path          string
	OldPath       string
	Before        string
	After         string
	Patch         string
	BeforePresent bool
	AfterPresent  bool
	Kind          ContentKind
	Capability    Capability
	Metadata      Metadata
}

func NewTextDocument(path, oldPath string, before []byte, beforePresent bool, after []byte, afterPresent bool) Document {
	return Document{
		Path:          path,
		OldPath:       oldPath,
		Before:        string(before),
		After:         string(after),
		BeforePresent: beforePresent,
		AfterPresent:  afterPresent,
		Kind:          ContentText,
		Capability:    Capability{FullFile: true, Hunks: true},
		Metadata:      Metadata{BeforeBytes: int64(len(before)), AfterBytes: int64(len(after))},
	}
}
