package highlight

import (
	"context"
	"fmt"
	"time"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"

	diffpkg "diffbeacon/internal/diff"
)

const (
	DefaultMaxBytes    = 256 << 10
	DefaultMaxTokens   = 50_000
	DefaultMaxDuration = 250 * time.Millisecond
)

type Limits struct {
	MaxBytes    int
	MaxTokens   int
	MaxDuration time.Duration
}

func DefaultLimits() Limits {
	return Limits{MaxBytes: DefaultMaxBytes, MaxTokens: DefaultMaxTokens, MaxDuration: DefaultMaxDuration}
}

type Result struct {
	Applied        bool
	Lexer          string
	FallbackReason string
}

// Apply selects a lexer solely from path, avoiding content-dependent and
// potentially surprising language detection.
func Apply(ctx context.Context, path string, model *diffpkg.Model, limits Limits) Result {
	lexer := lexers.Match(path)
	if lexer == nil || lexer == lexers.Fallback {
		clearStyles(model)
		return Result{FallbackReason: "no lexer matched the file name"}
	}
	return ApplyLexer(ctx, lexer, model, limits)
}

// ApplyLexer is exported to make lexer failures testable without global state.
func ApplyLexer(ctx context.Context, lexer chroma.Lexer, model *diffpkg.Model, limits Limits) Result {
	clearStyles(model)
	if model == nil {
		return Result{FallbackReason: "no diff model was provided"}
	}
	if model.Degraded || model.Document.Kind != diffpkg.ContentText {
		return Result{FallbackReason: "highlighting is unavailable for non-textual or degraded content"}
	}
	if lexer == nil {
		return Result{FallbackReason: "no lexer was provided"}
	}
	limits = normalize(limits)
	inputBytes := len(model.Document.Before) + len(model.Document.After)
	if inputBytes > limits.MaxBytes {
		return Result{FallbackReason: fmt.Sprintf("highlight input exceeds %d-byte limit", limits.MaxBytes)}
	}
	lines := uniqueLines(model.Operations)
	texts := make([]string, len(lines))
	for index, line := range lines {
		texts[index] = line.Text
	}
	workCtx, cancel := context.WithTimeout(ctx, limits.MaxDuration)
	defer cancel()
	select {
	case highlightSlot <- struct{}{}:
	case <-workCtx.Done():
		return timeoutResult(ctx, limits.MaxDuration)
	}

	resultChannel := make(chan highlightWorkResult, 1)
	go func() {
		defer func() { <-highlightSlot }()
		resultChannel <- tokenizeLines(workCtx, lexer, texts, limits.MaxTokens)
	}()
	select {
	case work := <-resultChannel:
		if work.reason != "" {
			return Result{FallbackReason: work.reason}
		}
		for index, spans := range work.styles {
			lines[index].Spans = spans
		}
		return Result{Applied: true, Lexer: work.lexer}
	case <-workCtx.Done():
		return timeoutResult(ctx, limits.MaxDuration)
	}
}

var highlightSlot = make(chan struct{}, 1)

type highlightWorkResult struct {
	styles [][]diffpkg.Span
	lexer  string
	reason string
}

func tokenizeLines(ctx context.Context, lexer chroma.Lexer, lines []string, maxTokens int) (result highlightWorkResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			result = highlightWorkResult{reason: fmt.Sprintf("lexer failed: %v", recovered)}
		}
	}()
	result.styles = make([][]diffpkg.Span, len(lines))
	tokenCount := 0
	for index, line := range lines {
		if err := ctx.Err(); err != nil {
			return highlightWorkResult{reason: "highlighting canceled: " + err.Error()}
		}
		iterator, err := lexer.Tokenise(nil, line)
		if err != nil {
			return highlightWorkResult{reason: "lexer failed: " + err.Error()}
		}
		for token := iterator(); token != chroma.EOF; token = iterator() {
			tokenCount++
			if tokenCount > maxTokens {
				return highlightWorkResult{reason: fmt.Sprintf("highlight token count exceeds %d-token limit", maxTokens)}
			}
			if err := ctx.Err(); err != nil {
				return highlightWorkResult{reason: "highlighting canceled: " + err.Error()}
			}
			result.styles[index] = append(result.styles[index], diffpkg.Span{Text: token.Value, Style: token.Type.String()})
		}
	}
	result.lexer = lexer.Config().Name
	return result
}

func timeoutResult(parent context.Context, duration time.Duration) Result {
	if err := parent.Err(); err != nil {
		return Result{FallbackReason: "highlighting canceled: " + err.Error()}
	}
	return Result{FallbackReason: fmt.Sprintf("highlighting exceeded %s time limit", duration)}
}

func normalize(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxBytes <= 0 {
		limits.MaxBytes = defaults.MaxBytes
	}
	if limits.MaxTokens <= 0 {
		limits.MaxTokens = defaults.MaxTokens
	}
	if limits.MaxDuration <= 0 {
		limits.MaxDuration = defaults.MaxDuration
	}
	return limits
}

func uniqueLines(operations []diffpkg.Operation) []*diffpkg.Line {
	seen := make(map[*diffpkg.Line]struct{}, len(operations)*2)
	lines := make([]*diffpkg.Line, 0, len(operations)*2)
	for _, operation := range operations {
		for _, line := range []*diffpkg.Line{operation.Before, operation.After} {
			if line == nil {
				continue
			}
			if _, ok := seen[line]; ok {
				continue
			}
			seen[line] = struct{}{}
			lines = append(lines, line)
		}
	}
	return lines
}

func clearStyles(model *diffpkg.Model) {
	if model == nil {
		return
	}
	for _, line := range uniqueLines(model.Operations) {
		line.Spans = nil
	}
}
