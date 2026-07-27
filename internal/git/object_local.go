package git

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxAlternatesBytes = 1024 * 1024

var ErrObjectNotLocal = errors.New("Git object is not available locally; remote retrieval is disabled")

func objectAvailableLocally(repository Repository, oidText string) (bool, error) {
	oid, err := hex.DecodeString(oidText)
	if err != nil || (len(oid) != 20 && len(oid) != 32) {
		return false, fmt.Errorf("invalid object ID")
	}
	objectDirectory, err := repositoryObjectDirectory(repository.GitDir)
	if err != nil {
		return false, err
	}
	return objectAvailableInDirectory(objectDirectory, oid, make(map[string]bool))
}

func repositoryObjectDirectory(gitDir string) (string, error) {
	commonDir := gitDir
	content, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err == nil {
		value := strings.TrimSpace(string(content))
		if value == "" {
			return "", fmt.Errorf("Git commondir is empty")
		}
		if filepath.IsAbs(value) {
			commonDir = filepath.Clean(value)
		} else {
			commonDir = filepath.Clean(filepath.Join(gitDir, value))
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read Git commondir: %w", err)
	}
	return filepath.Join(commonDir, "objects"), nil
}

func objectAvailableInDirectory(directory string, oid []byte, seen map[string]bool) (bool, error) {
	directory = filepath.Clean(directory)
	if seen[directory] {
		return false, nil
	}
	seen[directory] = true
	hexOID := hex.EncodeToString(oid)
	if info, err := os.Stat(filepath.Join(directory, hexOID[:2], hexOID[2:])); err == nil {
		return info.Mode().IsRegular(), nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("inspect loose Git object: %w", err)
	}

	packDirectory := filepath.Join(directory, "pack")
	entries, err := os.ReadDir(packDirectory)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("read Git pack directory: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".idx" {
			continue
		}
		found, err := packedObjectAvailable(filepath.Join(packDirectory, entry.Name()), oid)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}

	alternates, err := readAlternates(filepath.Join(directory, "info", "alternates"), directory)
	if err != nil {
		return false, err
	}
	for _, alternate := range alternates {
		found, err := objectAvailableInDirectory(alternate, oid, seen)
		if err != nil {
			return false, err
		}
		if found {
			return true, nil
		}
	}
	return false, nil
}

func packedObjectAvailable(path string, oid []byte) (bool, error) {
	packPath := strings.TrimSuffix(path, filepath.Ext(path)) + ".pack"
	if info, err := os.Stat(packPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("inspect Git pack: %w", err)
	} else if !info.Mode().IsRegular() {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("open Git pack index: %w", err)
	}
	defer file.Close()

	header := make([]byte, 8)
	if _, err := io.ReadFull(file, header); err != nil {
		return false, fmt.Errorf("read Git pack index header: %w", err)
	}
	version := uint32(1)
	fanoutOffset := int64(0)
	if bytes.Equal(header[:4], []byte{0xff, 0x74, 0x4f, 0x63}) {
		version = binary.BigEndian.Uint32(header[4:])
		if version != 2 {
			return false, fmt.Errorf("unsupported Git pack index version %d", version)
		}
		fanoutOffset = 8
	}
	fanout := make([]byte, 256*4)
	if _, err := file.ReadAt(fanout, fanoutOffset); err != nil {
		return false, fmt.Errorf("read Git pack index fanout: %w", err)
	}
	first := int(oid[0])
	low := uint32(0)
	if first > 0 {
		low = binary.BigEndian.Uint32(fanout[(first-1)*4 : first*4])
	}
	high := binary.BigEndian.Uint32(fanout[first*4 : (first+1)*4])
	oidTableOffset := fanoutOffset + int64(len(fanout))
	entrySize := int64(len(oid))
	if version == 1 {
		entrySize += 4
		oidTableOffset += 4
	}
	buffer := make([]byte, len(oid))
	for low < high {
		middle := low + (high-low)/2
		offset := oidTableOffset + int64(middle)*entrySize
		if version == 1 {
			offset = fanoutOffset + int64(len(fanout)) + int64(middle)*entrySize + 4
		}
		if _, err := file.ReadAt(buffer, offset); err != nil {
			return false, fmt.Errorf("read Git pack index object table: %w", err)
		}
		switch bytes.Compare(buffer, oid) {
		case -1:
			low = middle + 1
		case 1:
			high = middle
		default:
			return true, nil
		}
	}
	return false, nil
}

func readAlternates(path, objectDirectory string) ([]string, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open Git alternates: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(io.LimitReader(file, maxAlternatesBytes+1))
	var result []string
	total := 0
	for scanner.Scan() {
		total += len(scanner.Bytes()) + 1
		if total > maxAlternatesBytes {
			return nil, fmt.Errorf("Git alternates exceeds %d bytes", maxAlternatesBytes)
		}
		value := strings.TrimSpace(scanner.Text())
		if value == "" {
			continue
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(objectDirectory, value)
		}
		result = append(result, filepath.Clean(value))
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Git alternates: %w", err)
	}
	return result, nil
}
