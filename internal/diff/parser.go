package diff

import (
	"fmt"
	"strconv"
	"strings"

	"diffbeacon/internal/sanitize"
)

type OperationKind uint8

const (
	OperationEqual OperationKind = iota + 1
	OperationDelete
	OperationAdd
)

type Span struct {
	Text  string
	Style string
}

type Line struct {
	Number     int
	Text       string
	Terminated bool
	Spans      []Span
}

type Operation struct {
	Kind   OperationKind
	Before *Line
	After  *Line
}

type Model struct {
	Document   Document
	Operations []Operation
	FullRows   []Row
	Hunks      []Hunk
	Degraded   bool
	Reason     string
}

type patchHunk struct {
	beforeStart int
	beforeCount int
	afterStart  int
	afterCount  int
	operations  []Operation
}

// Build creates one model shared by full-file, changes-only, inline and
// side-by-side presentations. Tracked changes use Git's unified patch as the
// edit script; complete contents are only required for the full-file rows.
func Build(document Document, limits Limits) Model {
	model := Model{Document: document}
	if document.Kind != ContentText {
		return degradedModel(model, nonEmptyReason(document.Capability.HunksReason, "content kind "+string(document.Kind)+" is not a textual diff"))
	}
	limits = limits.Normalized()

	var parsed []patchHunk
	if document.Capability.Hunks && document.Patch != "" {
		var reason string
		parsed, reason = parsePatch(document.Patch, limits)
		if reason != "" {
			return degradedModel(model, reason)
		}
	}

	before, after, fullAvailable := fullFileLines(&model, limits)
	if fullAvailable {
		if len(parsed) > 0 {
			operations, reason := materializeFull(before, after, parsed)
			if reason != "" {
				return degradedModel(model, reason)
			}
			model.Operations = operations
		} else {
			model.Operations = buildSimpleOperations(before, after)
		}
		model.FullRows = Align(model.Operations)
	}

	if len(parsed) > 0 {
		model.Hunks = patchHunks(parsed)
		if !fullAvailable {
			for _, hunk := range parsed {
				model.Operations = append(model.Operations, hunk.operations...)
			}
		}
	} else if fullAvailable {
		model.Hunks = BuildHunks(model.Operations, limits.ContextLines)
	}

	if !model.Document.Capability.FullFile && !model.Document.Capability.Hunks {
		return degradedModel(model, nonEmptyReason(model.Document.Capability.HunksReason, model.Document.Capability.FullFileReason))
	}
	return model
}

func fullFileLines(model *Model, limits Limits) ([]sourceLine, []sourceLine, bool) {
	document := &model.Document
	if !document.Capability.FullFile {
		return nil, nil, false
	}
	if len(document.Before) > limits.MaxContentBytes || len(document.After) > limits.MaxContentBytes {
		document.Capability.FullFile = false
		document.Capability.FullFileReason = fmt.Sprintf("content exceeds %d-byte full-file limit", limits.MaxContentBytes)
		return nil, nil, false
	}
	before, reason := splitLines(document.Before, "before", limits)
	if reason != "" {
		document.Capability.FullFile = false
		document.Capability.FullFileReason = reason
		return nil, nil, false
	}
	after, reason := splitLines(document.After, "after", limits)
	if reason != "" {
		document.Capability.FullFile = false
		document.Capability.FullFileReason = reason
		return nil, nil, false
	}
	if len(before)+len(after) > limits.MaxLines {
		document.Capability.FullFile = false
		document.Capability.FullFileReason = fmt.Sprintf("line count exceeds %d-line full-file limit", limits.MaxLines)
		return nil, nil, false
	}
	return before, after, true
}

func parsePatch(patch string, limits Limits) ([]patchHunk, string) {
	if len(patch) > limits.MaxPatchBytes {
		return nil, fmt.Sprintf("Git patch exceeds %d-byte changes limit", limits.MaxPatchBytes)
	}
	lines := strings.Split(patch, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > limits.MaxPatchLines {
		return nil, fmt.Sprintf("Git patch exceeds %d-line changes limit", limits.MaxPatchLines)
	}

	var hunks []patchHunk
	for index := 0; index < len(lines); {
		if !strings.HasPrefix(lines[index], "@@ ") {
			index++
			continue
		}
		hunk, reason := parseHunkHeader(lines[index])
		if reason != "" {
			return nil, reason
		}
		index++
		beforeNumber, afterNumber := hunk.beforeStart, hunk.afterStart
		beforeSeen, afterSeen := 0, 0
		for index < len(lines) {
			line := lines[index]
			if line == `\ No newline at end of file` {
				if len(hunk.operations) == 0 {
					return nil, "Git patch has a newline marker without a preceding line"
				}
				previous := &hunk.operations[len(hunk.operations)-1]
				if previous.Before != nil {
					previous.Before.Terminated = false
				}
				if previous.After != nil {
					previous.After.Terminated = false
				}
				index++
				continue
			}
			if beforeSeen == hunk.beforeCount && afterSeen == hunk.afterCount {
				break
			}
			if line == "" || (line[0] != ' ' && line[0] != '-' && line[0] != '+') {
				return nil, fmt.Sprintf("malformed Git patch hunk at line %d", index+1)
			}
			if len(line)-1 > limits.MaxPatchLineBytes {
				return nil, fmt.Sprintf("Git patch line %d exceeds %d-byte changes limit", index+1, limits.MaxPatchLineBytes)
			}
			text := SafeDisplayText(line[1:])
			switch line[0] {
			case ' ':
				hunk.operations = append(hunk.operations, Operation{
					Kind:   OperationEqual,
					Before: &Line{Number: beforeNumber, Text: text, Terminated: true},
					After:  &Line{Number: afterNumber, Text: text, Terminated: true},
				})
				beforeNumber++
				afterNumber++
				beforeSeen++
				afterSeen++
			case '-':
				hunk.operations = append(hunk.operations, Operation{Kind: OperationDelete, Before: &Line{Number: beforeNumber, Text: text, Terminated: true}})
				beforeNumber++
				beforeSeen++
			case '+':
				hunk.operations = append(hunk.operations, Operation{Kind: OperationAdd, After: &Line{Number: afterNumber, Text: text, Terminated: true}})
				afterNumber++
				afterSeen++
			}
			if beforeSeen > hunk.beforeCount || afterSeen > hunk.afterCount {
				return nil, fmt.Sprintf("Git patch hunk exceeds its declared range at line %d", index+1)
			}
			index++
		}
		if beforeSeen != hunk.beforeCount || afterSeen != hunk.afterCount {
			return nil, "Git patch hunk ended before its declared range"
		}
		hunks = append(hunks, hunk)
	}
	return hunks, ""
}

func parseHunkHeader(header string) (patchHunk, string) {
	fields := strings.Fields(header)
	if len(fields) < 4 || fields[0] != "@@" || fields[3] != "@@" {
		return patchHunk{}, "malformed Git patch hunk header"
	}
	beforeStart, beforeCount, ok := parseRange(fields[1], '-')
	if !ok {
		return patchHunk{}, "malformed Git patch before range"
	}
	afterStart, afterCount, ok := parseRange(fields[2], '+')
	if !ok {
		return patchHunk{}, "malformed Git patch after range"
	}
	return patchHunk{beforeStart: beforeStart, beforeCount: beforeCount, afterStart: afterStart, afterCount: afterCount}, ""
}

func parseRange(value string, prefix byte) (int, int, bool) {
	if len(value) < 2 || value[0] != prefix {
		return 0, 0, false
	}
	startText, countText, hasCount := strings.Cut(value[1:], ",")
	start, err := strconv.Atoi(startText)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	count := 1
	if hasCount {
		count, err = strconv.Atoi(countText)
		if err != nil || count < 0 {
			return 0, 0, false
		}
	}
	return start, count, true
}

func materializeFull(before, after []sourceLine, hunks []patchHunk) ([]Operation, string) {
	operations := make([]Operation, 0, len(before)+len(after))
	beforeIndex, afterIndex := 0, 0
	for hunkIndex := range hunks {
		hunk := &hunks[hunkIndex]
		beforeTarget := rangeIndex(hunk.beforeStart, hunk.beforeCount)
		afterTarget := rangeIndex(hunk.afterStart, hunk.afterCount)
		if beforeTarget < beforeIndex || afterTarget < afterIndex || beforeTarget > len(before) || afterTarget > len(after) {
			return nil, "Git patch ranges are outside the loaded content"
		}
		if beforeTarget-beforeIndex != afterTarget-afterIndex {
			return nil, "Git patch has inconsistent unchanged ranges"
		}
		for beforeIndex < beforeTarget {
			if before[beforeIndex] != after[afterIndex] {
				return nil, "Git patch does not account for differing full-file lines"
			}
			operations = append(operations, equalOperation(beforeIndex, afterIndex, before[beforeIndex], after[afterIndex]))
			beforeIndex++
			afterIndex++
		}

		for operationIndex := range hunk.operations {
			operation := &hunk.operations[operationIndex]
			if operation.Before != nil {
				if beforeIndex >= len(before) || !lineMatches(operation.Before, before[beforeIndex]) {
					return nil, "Git patch before line does not match loaded content"
				}
				operation.Before = newLine(beforeIndex+1, before[beforeIndex])
				beforeIndex++
			}
			if operation.After != nil {
				if afterIndex >= len(after) || !lineMatches(operation.After, after[afterIndex]) {
					return nil, "Git patch after line does not match loaded content"
				}
				operation.After = newLine(afterIndex+1, after[afterIndex])
				afterIndex++
			}
			operations = append(operations, *operation)
		}
	}
	if len(before)-beforeIndex != len(after)-afterIndex {
		return nil, "Git patch does not account for the full-file line count"
	}
	for beforeIndex < len(before) {
		if before[beforeIndex] != after[afterIndex] {
			return nil, "Git patch does not account for differing trailing lines"
		}
		operations = append(operations, equalOperation(beforeIndex, afterIndex, before[beforeIndex], after[afterIndex]))
		beforeIndex++
		afterIndex++
	}
	return operations, ""
}

func patchHunks(parsed []patchHunk) []Hunk {
	hunks := make([]Hunk, 0, len(parsed))
	operationOffset := 0
	for index, parsedHunk := range parsed {
		hunks = append(hunks, Hunk{
			ID:              fmt.Sprintf("hunk-%d", index+1),
			OperationStart:  operationOffset,
			OperationEnd:    operationOffset + len(parsedHunk.operations),
			BeforeStart:     parsedHunk.beforeStart,
			BeforeLineCount: parsedHunk.beforeCount,
			AfterStart:      parsedHunk.afterStart,
			AfterLineCount:  parsedHunk.afterCount,
			Rows:            Align(parsedHunk.operations),
		})
		operationOffset += len(parsedHunk.operations)
	}
	return hunks
}

func buildSimpleOperations(before, after []sourceLine) []Operation {
	prefix := 0
	for prefix < len(before) && prefix < len(after) && before[prefix] == after[prefix] {
		prefix++
	}
	beforeEnd, afterEnd := len(before), len(after)
	for beforeEnd > prefix && afterEnd > prefix && before[beforeEnd-1] == after[afterEnd-1] {
		beforeEnd--
		afterEnd--
	}
	operations := make([]Operation, 0, len(before)+len(after))
	for index := 0; index < prefix; index++ {
		operations = append(operations, equalOperation(index, index, before[index], after[index]))
	}
	for index := prefix; index < beforeEnd; index++ {
		operations = append(operations, Operation{Kind: OperationDelete, Before: newLine(index+1, before[index])})
	}
	for index := prefix; index < afterEnd; index++ {
		operations = append(operations, Operation{Kind: OperationAdd, After: newLine(index+1, after[index])})
	}
	beforeOffset, afterOffset := beforeEnd, afterEnd
	for beforeOffset < len(before) {
		operations = append(operations, equalOperation(beforeOffset, afterOffset, before[beforeOffset], after[afterOffset]))
		beforeOffset++
		afterOffset++
	}
	return operations
}

type sourceLine struct {
	text       string
	terminated bool
}

func splitLines(content, side string, limits Limits) ([]sourceLine, string) {
	if content == "" {
		return nil, ""
	}
	parts := strings.Split(content, "\n")
	endsWithNewline := parts[len(parts)-1] == ""
	if endsWithNewline {
		parts = parts[:len(parts)-1]
	}
	lines := make([]sourceLine, 0, len(parts))
	for index, line := range parts {
		if len(line) > limits.MaxLineBytes {
			return nil, fmt.Sprintf("line %d on %s exceeds %d-byte full-file limit", index+1, side, limits.MaxLineBytes)
		}
		lines = append(lines, sourceLine{text: line, terminated: index < len(parts)-1 || endsWithNewline})
	}
	return lines, ""
}

func rangeIndex(start, count int) int {
	if count == 0 {
		return start
	}
	return start - 1
}

func lineMatches(line *Line, source sourceLine) bool {
	return line.Text == SafeDisplayText(source.text) && line.Terminated == source.terminated
}

func equalOperation(beforeIndex, afterIndex int, before, after sourceLine) Operation {
	return Operation{Kind: OperationEqual, Before: newLine(beforeIndex+1, before), After: newLine(afterIndex+1, after)}
}

func newLine(number int, source sourceLine) *Line {
	return &Line{Number: number, Text: SafeDisplayText(source.text), Terminated: source.terminated}
}

func degradedModel(model Model, reason string) Model {
	model.Degraded = true
	model.Reason = nonEmptyReason(reason, "text diff unavailable")
	return model
}

func nonEmptyReason(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// SafeDisplayText makes terminal control characters visible data. Tabs are
// retained; all other C0 controls, DEL and C1 controls use escape notation.
func SafeDisplayText(value string) string {
	return sanitize.DisplayText(value)
}
