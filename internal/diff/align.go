package diff

type RowKind uint8

const (
	RowEqual RowKind = iota + 1
	RowAdded
	RowDeleted
	RowChanged
)

type Row struct {
	Kind   RowKind
	Before *Line
	After  *Line
}

// Align zips each adjacent delete/add block, leaving an explicit empty side for
// unmatched lines. Insertions and deletions therefore cannot shift later rows.
func Align(operations []Operation) []Row {
	rows := make([]Row, 0, len(operations))
	for index := 0; index < len(operations); {
		if operations[index].Kind == OperationEqual {
			rows = append(rows, Row{Kind: RowEqual, Before: operations[index].Before, After: operations[index].After})
			index++
			continue
		}
		end := index
		var deleted, added []*Line
		for end < len(operations) && operations[end].Kind != OperationEqual {
			if operations[end].Kind == OperationDelete {
				deleted = append(deleted, operations[end].Before)
			} else {
				added = append(added, operations[end].After)
			}
			end++
		}
		count := len(deleted)
		if len(added) > count {
			count = len(added)
		}
		for offset := 0; offset < count; offset++ {
			row := Row{Kind: RowChanged}
			if offset < len(deleted) {
				row.Before = deleted[offset]
			}
			if offset < len(added) {
				row.After = added[offset]
			}
			if row.Before == nil {
				row.Kind = RowAdded
			} else if row.After == nil {
				row.Kind = RowDeleted
			}
			rows = append(rows, row)
		}
		index = end
	}
	return rows
}
