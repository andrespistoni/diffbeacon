package diff

const (
	DefaultMaxContentBytes   = 1 << 20
	DefaultMaxDiffInputBytes = 64 << 20
	DefaultMaxPatchBytes     = 8 << 20
	DefaultMaxLineBytes      = 16 << 10
	DefaultMaxPatchLineBytes = 256 << 10
	DefaultMaxLines          = 20_000
	DefaultMaxPatchLines     = 100_000
)

// Limits bounds every allocation whose size depends on repository content.
type Limits struct {
	MaxContentBytes   int
	MaxDiffInputBytes int
	MaxPatchBytes     int
	MaxLineBytes      int
	MaxPatchLineBytes int
	MaxLines          int
	MaxPatchLines     int
	ContextLines      int
}

func DefaultLimits() Limits {
	return Limits{
		MaxContentBytes:   DefaultMaxContentBytes,
		MaxDiffInputBytes: DefaultMaxDiffInputBytes,
		MaxPatchBytes:     DefaultMaxPatchBytes,
		MaxLineBytes:      DefaultMaxLineBytes,
		MaxPatchLineBytes: DefaultMaxPatchLineBytes,
		MaxLines:          DefaultMaxLines,
		MaxPatchLines:     DefaultMaxPatchLines,
		ContextLines:      3,
	}
}

func (l Limits) Normalized() Limits {
	defaults := DefaultLimits()
	if l.MaxContentBytes <= 0 {
		l.MaxContentBytes = defaults.MaxContentBytes
	}
	if l.MaxDiffInputBytes <= 0 {
		l.MaxDiffInputBytes = defaults.MaxDiffInputBytes
	}
	if l.MaxPatchBytes <= 0 {
		l.MaxPatchBytes = defaults.MaxPatchBytes
	}
	if l.MaxLineBytes <= 0 {
		l.MaxLineBytes = defaults.MaxLineBytes
	}
	if l.MaxPatchLineBytes <= 0 {
		l.MaxPatchLineBytes = defaults.MaxPatchLineBytes
	}
	if l.MaxLines <= 0 {
		l.MaxLines = defaults.MaxLines
	}
	if l.MaxPatchLines <= 0 {
		l.MaxPatchLines = defaults.MaxPatchLines
	}
	if l.ContextLines < 0 {
		l.ContextLines = defaults.ContextLines
	}
	return l
}
