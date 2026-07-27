package highlight

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/chroma/v2"

	diffpkg "diffbeacon/internal/diff"
)

func TestHighlightKnownLexerPreservesDiffSemantics(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("main.go", "", []byte("package old\n"), true, []byte("package main\n"), true), diffpkg.DefaultLimits())
	result := Apply(context.Background(), "main.go", &model, DefaultLimits())
	if !result.Applied || result.Lexer == "" {
		t.Fatalf("Apply() = %#v", result)
	}
	if model.Operations[0].Kind != diffpkg.OperationDelete || model.Operations[0].Before.Number != 1 || len(model.Operations[0].Before.Spans) == 0 {
		t.Fatalf("deleted operation changed or unstyled: %#v", model.Operations[0])
	}
	if model.Operations[1].Kind != diffpkg.OperationAdd || model.Operations[1].After.Number != 1 || len(model.Operations[1].After.Spans) == 0 {
		t.Fatalf("added operation changed or unstyled: %#v", model.Operations[1])
	}
}

func TestHighlightUnknownLexerFallsBackToSafePlainText(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("data.unknown-diffbeacon", "", nil, false, []byte("plain\x1b[31mred\n"), true), diffpkg.DefaultLimits())
	result := Apply(context.Background(), "data.unknown-diffbeacon", &model, DefaultLimits())
	if result.Applied || result.FallbackReason == "" {
		t.Fatalf("Apply() = %#v", result)
	}
	line := model.Operations[0].After
	if line.Text != "plain\\x1b[31mred" || strings.ContainsRune(line.Text, '\x1b') || len(line.Spans) != 0 {
		t.Fatalf("plain fallback line = %#v", line)
	}
}

func TestHighlightLexerErrorFallsBackWithoutPartialStyles(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("bad.txt", "", nil, false, []byte("text\n"), true), diffpkg.DefaultLimits())
	result := ApplyLexer(context.Background(), failingLexer{}, &model, DefaultLimits())
	if result.Applied || !strings.Contains(result.FallbackReason, "failed") || len(model.Operations[0].After.Spans) != 0 {
		t.Fatalf("ApplyLexer() = %#v, line = %#v", result, model.Operations[0].After)
	}
}

func TestHighlightLimitsAndCancellationFallBack(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("main.go", "", nil, false, []byte("package main\n"), true), diffpkg.DefaultLimits())
	result := Apply(context.Background(), "main.go", &model, Limits{MaxBytes: 4, MaxTokens: 100})
	if result.Applied || result.FallbackReason != "highlight input exceeds 4-byte limit" {
		t.Fatalf("byte-limited Apply() = %#v", result)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result = Apply(ctx, "main.go", &model, DefaultLimits())
	if result.Applied || !strings.Contains(result.FallbackReason, "canceled") {
		t.Fatalf("canceled Apply() = %#v", result)
	}
}

func TestHighlightTimeLimitFallsBackAndBoundsWorkers(t *testing.T) {
	model := diffpkg.Build(diffpkg.NewTextDocument("slow.go", "", nil, false, []byte("package main\n"), true), diffpkg.DefaultLimits())
	result := ApplyLexer(context.Background(), slowLexer{}, &model, Limits{MaxBytes: 100, MaxTokens: 100, MaxDuration: time.Millisecond})
	if result.Applied || !strings.Contains(result.FallbackReason, "time limit") {
		t.Fatalf("time-limited ApplyLexer() = %#v", result)
	}
	// Let the single bounded worker finish so it cannot affect later tests.
	time.Sleep(30 * time.Millisecond)
}

type failingLexer struct{}

func (failingLexer) Config() *chroma.Config { return &chroma.Config{Name: "failing"} }
func (failingLexer) Tokenise(*chroma.TokeniseOptions, string) (chroma.Iterator, error) {
	return nil, errors.New("intentional lexer failure")
}
func (lexer failingLexer) SetRegistry(*chroma.LexerRegistry) chroma.Lexer { return lexer }
func (lexer failingLexer) SetAnalyser(func(string) float32) chroma.Lexer  { return lexer }
func (failingLexer) AnalyseText(string) float32                           { return 0 }

type slowLexer struct{ failingLexer }

func (slowLexer) Config() *chroma.Config { return &chroma.Config{Name: "slow"} }
func (slowLexer) Tokenise(*chroma.TokeniseOptions, string) (chroma.Iterator, error) {
	time.Sleep(20 * time.Millisecond)
	return chroma.Literator(chroma.Token{Type: chroma.Text, Value: "package main"}), nil
}
