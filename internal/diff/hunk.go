package diff

import "fmt"

type Hunk struct {
	ID              string
	OperationStart  int
	OperationEnd    int
	BeforeStart     int
	BeforeLineCount int
	AfterStart      int
	AfterLineCount  int
	Rows            []Row
}

func BuildHunks(operations []Operation, contextLines int) []Hunk {
	if contextLines < 0 {
		contextLines = 0
	}
	var ranges [][2]int
	for index, operation := range operations {
		if operation.Kind == OperationEqual {
			continue
		}
		start := index - contextLines
		if start < 0 {
			start = 0
		}
		end := index + contextLines + 1
		if end > len(operations) {
			end = len(operations)
		}
		if len(ranges) > 0 && start <= ranges[len(ranges)-1][1] {
			if end > ranges[len(ranges)-1][1] {
				ranges[len(ranges)-1][1] = end
			}
		} else {
			ranges = append(ranges, [2]int{start, end})
		}
	}

	hunks := make([]Hunk, 0, len(ranges))
	for index, operationRange := range ranges {
		hunk := Hunk{
			ID:             fmt.Sprintf("hunk-%d", index+1),
			OperationStart: operationRange[0],
			OperationEnd:   operationRange[1],
			Rows:           Align(operations[operationRange[0]:operationRange[1]]),
		}
		hunk.BeforeStart, hunk.BeforeLineCount = sideRange(operations, operationRange[0], operationRange[1], true)
		hunk.AfterStart, hunk.AfterLineCount = sideRange(operations, operationRange[0], operationRange[1], false)
		hunks = append(hunks, hunk)
	}
	return hunks
}

func sideRange(operations []Operation, operationStart, operationEnd int, before bool) (int, int) {
	consumed := 0
	for _, operation := range operations[:operationStart] {
		line := operation.After
		if before {
			line = operation.Before
		}
		if line != nil {
			consumed++
		}
	}
	count := 0
	for _, operation := range operations[operationStart:operationEnd] {
		line := operation.After
		if before {
			line = operation.Before
		}
		if line == nil {
			continue
		}
		count++
	}
	if count == 0 {
		return consumed, 0
	}
	return consumed + 1, count
}

func (m Model) NextHunk(current int) int {
	if len(m.Hunks) == 0 {
		return -1
	}
	if current < -1 || current >= len(m.Hunks)-1 {
		return 0
	}
	return current + 1
}

func (m Model) PreviousHunk(current int) int {
	if len(m.Hunks) == 0 {
		return -1
	}
	if current <= 0 || current >= len(m.Hunks) {
		return len(m.Hunks) - 1
	}
	return current - 1
}

// ChangesRows returns the contextual rows in stable hunk order.
func (m Model) ChangesRows() []Row {
	count := 0
	for _, hunk := range m.Hunks {
		count += len(hunk.Rows)
	}
	rows := make([]Row, 0, count)
	for _, hunk := range m.Hunks {
		rows = append(rows, hunk.Rows...)
	}
	return rows
}
