package git

import (
	"bytes"
	"fmt"
	"strings"
)

func ParseStatus(data []byte) (Snapshot, error) {
	if len(data) > DefaultStatusMaxBytes {
		return Snapshot{}, fmt.Errorf("%w: porcelain input exceeded %d bytes", ErrStatusBudget, DefaultStatusMaxBytes)
	}
	return parseStatusLimited(data, DefaultStatusMaxEntries)
}

func parseStatusLimited(data []byte, maxEntries int) (Snapshot, error) {
	snapshot := Snapshot{}
	entries := 0
	for offset, index := 0, 0; offset < len(data); index++ {
		record, next := nextStatusRecord(data, offset)
		offset = next
		if len(record) == 0 {
			continue
		}

		switch record[0] {
		case '#':
			parseHeader(record, &snapshot.Revision)
		case '1':
			entries++
			changes, err := parseTrackedRecord(record, "", false)
			if err != nil {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: %w", index, err)
			}
			snapshot.Changes = append(snapshot.Changes, changes...)
		case '2':
			entries++
			if offset >= len(data) {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: rename/copy has no original path", index)
			}
			original, next := nextStatusRecord(data, offset)
			if len(original) == 0 {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: rename/copy has no original path", index)
			}
			oldPath := string(original)
			offset = next
			index++
			changes, err := parseTrackedRecord(record, oldPath, true)
			if err != nil {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: %w", index-1, err)
			}
			snapshot.Changes = append(snapshot.Changes, changes...)
		case 'u':
			entries++
			change, err := parseUnmergedRecord(record)
			if err != nil {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: %w", index, err)
			}
			snapshot.Changes = append(snapshot.Changes, change)
		case '?':
			entries++
			if len(record) < 3 || record[1] != ' ' {
				return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: malformed untracked entry", index)
			}
			snapshot.Changes = append(snapshot.Changes, Change{
				Path:   string(record[2:]),
				Scope:  ScopeUntracked,
				Status: StatusUntracked,
			})
		case '!':
			// Ignored records are deliberately not part of the product model.
		default:
			return Snapshot{}, fmt.Errorf("parse porcelain v2 record %d: unsupported record type %q", index, record[0])
		}
		if entries > maxEntries || len(snapshot.Changes) > maxEntries {
			return Snapshot{}, fmt.Errorf("%w: porcelain output exceeded %d entries", ErrStatusBudget, maxEntries)
		}
	}
	return snapshot, nil
}

func nextStatusRecord(data []byte, offset int) ([]byte, int) {
	relativeEnd := bytes.IndexByte(data[offset:], 0)
	if relativeEnd < 0 {
		return data[offset:], len(data)
	}
	end := offset + relativeEnd
	return data[offset:end], end + 1
}

func parseHeader(record []byte, revision *Revision) {
	value := string(record)
	if strings.HasPrefix(value, "# branch.oid ") {
		revision.HeadOID = strings.TrimPrefix(value, "# branch.oid ")
	}
	if strings.HasPrefix(value, "# branch.head ") {
		revision.Branch = strings.TrimPrefix(value, "# branch.head ")
	}
}

func parseTrackedRecord(record []byte, oldPath string, renamedOrCopied bool) ([]Change, error) {
	fieldCount := 9
	if renamedOrCopied {
		fieldCount = 10
	}
	fields := bytes.SplitN(record, []byte{' '}, fieldCount)
	if len(fields) != fieldCount || len(fields[1]) != 2 || len(fields[2]) != 4 || len(fields[fieldCount-1]) == 0 {
		return nil, errorsForMalformedTracked(renamedOrCopied)
	}
	expectedKind := "1"
	if renamedOrCopied {
		expectedKind = "2"
	}
	if string(fields[0]) != expectedKind {
		return nil, errorsForMalformedTracked(renamedOrCopied)
	}

	xy := fields[1]
	if renamedOrCopied {
		score := fields[8]
		if len(score) < 2 || (score[0] != 'R' && score[0] != 'C') || (xy[0] != score[0] && xy[1] != score[0]) {
			return nil, fmt.Errorf("malformed rename/copy score")
		}
	}
	submodule, err := parseSubmodule(fields[2])
	if err != nil {
		return nil, err
	}
	path := string(fields[fieldCount-1])
	changes := make([]Change, 0, 2)
	if xy[0] != '.' {
		status, err := statusFromCode(xy[0])
		if err != nil {
			return nil, err
		}
		changes = append(changes, Change{Path: path, OldPath: oldPathFor(status, oldPath), Scope: ScopeStaged, Status: status, Submodule: submodule})
	}
	if xy[1] != '.' {
		status, err := statusFromCode(xy[1])
		if err != nil {
			return nil, err
		}
		changes = append(changes, Change{Path: path, OldPath: oldPathFor(status, oldPath), Scope: ScopeUnstaged, Status: status, Submodule: submodule})
	}
	if len(changes) == 0 {
		return nil, fmt.Errorf("tracked entry has no change")
	}
	return changes, nil
}

func errorsForMalformedTracked(renamedOrCopied bool) error {
	if renamedOrCopied {
		return fmt.Errorf("malformed rename/copy entry")
	}
	return fmt.Errorf("malformed tracked entry")
}

func parseUnmergedRecord(record []byte) (Change, error) {
	fields := bytes.SplitN(record, []byte{' '}, 11)
	if len(fields) != 11 || string(fields[0]) != "u" || len(fields[1]) != 2 || len(fields[2]) != 4 || len(fields[10]) == 0 {
		return Change{}, fmt.Errorf("malformed unmerged entry")
	}
	conflict := ConflictKind(string(fields[1]))
	if !validConflict(conflict) {
		return Change{}, fmt.Errorf("unknown conflict code %q", conflict)
	}
	submodule, err := parseSubmodule(fields[2])
	if err != nil {
		return Change{}, err
	}
	return Change{
		Path:      string(fields[10]),
		Scope:     ScopeUnstaged,
		Status:    StatusUnmerged,
		Conflict:  conflict,
		Submodule: submodule,
	}, nil
}

func statusFromCode(code byte) (ChangeStatus, error) {
	switch code {
	case 'A':
		return StatusAdded, nil
	case 'M':
		return StatusModified, nil
	case 'D':
		return StatusDeleted, nil
	case 'R':
		return StatusRenamed, nil
	case 'C':
		return StatusCopied, nil
	case 'T':
		return StatusTypeChanged, nil
	case 'U':
		return StatusUnmerged, nil
	default:
		return 0, fmt.Errorf("unknown status code %q", code)
	}
}

func oldPathFor(status ChangeStatus, oldPath string) string {
	if status == StatusRenamed || status == StatusCopied {
		return oldPath
	}
	return ""
}

func parseSubmodule(field []byte) (SubmoduleState, error) {
	if bytes.Equal(field, []byte("N...")) {
		return SubmoduleState{}, nil
	}
	if len(field) != 4 || field[0] != 'S' {
		return SubmoduleState{}, fmt.Errorf("malformed submodule field %q", field)
	}
	if (field[1] != '.' && field[1] != 'C') || (field[2] != '.' && field[2] != 'M') || (field[3] != '.' && field[3] != 'U') {
		return SubmoduleState{}, fmt.Errorf("malformed submodule field %q", field)
	}
	return SubmoduleState{
		IsSubmodule:      true,
		CommitChanged:    field[1] == 'C',
		TrackedModified:  field[2] == 'M',
		UntrackedPresent: field[3] == 'U',
	}, nil
}

func validConflict(conflict ConflictKind) bool {
	switch conflict {
	case ConflictBothDeleted, ConflictAddedByUs, ConflictDeletedByThem, ConflictAddedByThem, ConflictDeletedByUs, ConflictBothAdded, ConflictBothModified:
		return true
	default:
		return false
	}
}
