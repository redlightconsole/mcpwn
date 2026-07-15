package output

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const outputDirEnv = "MCPWN_OUTPUT_DIR"

var validID = regexp.MustCompile(`^[0-9T.Z-]+-[a-f0-9]{16}$`)

type Store struct {
	dir string
}

type Entry struct {
	ID   string
	Path string
}

type ReadResult struct {
	Data       []byte
	Offset     int64
	NextOffset int64
	Size       int64
	EOF        bool
}

type SearchMatch struct {
	Line int
	Text string
}

func DefaultStore() (*Store, error) {
	if dir := os.Getenv(outputDirEnv); dir != "" {
		return NewStore(dir)
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve cache directory: %w", err)
	}

	return NewStore(filepath.Join(cacheDir, "mcpwn", "outputs"))
}

func NewStore(dir string) (*Store, error) {
	if dir == "" {
		return nil, fmt.Errorf("output directory is empty")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	return &Store{dir: dir}, nil
}

func (s *Store) Create() (*Entry, io.WriteCloser, error) {
	id, err := newID()
	if err != nil {
		return nil, nil, err
	}

	path := filepath.Join(s.dir, id+".log")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create output file: %w", err)
	}

	return &Entry{ID: id, Path: path}, file, nil
}

func (s *Store) ReadAll(id string) ([]byte, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read output %q: %w", id, err)
	}
	return data, nil
}

func (s *Store) Read(id string, offset, limit int64) (ReadResult, error) {
	if offset < 0 {
		return ReadResult{}, fmt.Errorf("offset must be zero or greater")
	}
	if limit <= 0 {
		return ReadResult{}, fmt.Errorf("limit must be greater than zero")
	}

	path, err := s.path(id)
	if err != nil {
		return ReadResult{}, err
	}

	file, err := os.Open(path)
	if err != nil {
		return ReadResult{}, fmt.Errorf("failed to open output %q: %w", id, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return ReadResult{}, fmt.Errorf("failed to stat output %q: %w", id, err)
	}

	if offset > stat.Size() {
		offset = stat.Size()
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return ReadResult{}, fmt.Errorf("failed to seek output %q: %w", id, err)
	}

	data, err := io.ReadAll(io.LimitReader(file, limit))
	if err != nil {
		return ReadResult{}, fmt.Errorf("failed to read output %q: %w", id, err)
	}

	nextOffset := offset + int64(len(data))
	return ReadResult{
		Data:       data,
		Offset:     offset,
		NextOffset: nextOffset,
		Size:       stat.Size(),
		EOF:        nextOffset >= stat.Size(),
	}, nil
}

func (s *Store) Tail(id string, lines int) (string, error) {
	if lines <= 0 {
		return "", fmt.Errorf("lines must be greater than zero")
	}

	path, err := s.path(id)
	if err != nil {
		return "", err
	}

	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open output %q: %w", id, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat output %q: %w", id, err)
	}

	const chunkSize int64 = 4096
	var data []byte
	for offset := stat.Size(); offset > 0 && countLines(data) <= lines; {
		readSize := chunkSize
		if offset < readSize {
			readSize = offset
		}
		offset -= readSize

		chunk := make([]byte, readSize)
		if _, err := file.ReadAt(chunk, offset); err != nil {
			return "", fmt.Errorf("failed to read output %q: %w", id, err)
		}
		data = append(chunk, data...)
	}

	data = bytes.TrimRight(data, "\n")
	if len(data) == 0 {
		return "", nil
	}

	parts := bytes.Split(data, []byte{'\n'})
	if len(parts) > lines {
		parts = parts[len(parts)-lines:]
	}
	return string(bytes.Join(parts, []byte{'\n'})), nil
}

func (s *Store) Search(id, query string, maxMatches int) ([]SearchMatch, error) {
	if query == "" {
		return nil, fmt.Errorf("query must not be empty")
	}
	if maxMatches <= 0 {
		return nil, fmt.Errorf("max_matches must be greater than zero")
	}

	path, err := s.path(id)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open output %q: %w", id, err)
	}
	defer file.Close()

	var matches []SearchMatch
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		if strings.Contains(text, query) {
			matches = append(matches, SearchMatch{Line: line, Text: text})
			if len(matches) >= maxMatches {
				break
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to search output %q: %w", id, err)
	}

	return matches, nil
}

func (s *Store) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid output_id: %s", id)
	}
	return filepath.Join(s.dir, id+".log"), nil
}

func countLines(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	return bytes.Count(data, []byte{'\n'})
}

func newID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("failed to generate output id: %w", err)
	}

	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random[:]), nil
}
