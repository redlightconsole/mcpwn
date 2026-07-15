package output

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
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

func (s *Store) path(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", fmt.Errorf("invalid output_id: %s", id)
	}
	return filepath.Join(s.dir, id+".log"), nil
}

func newID() (string, error) {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("failed to generate output id: %w", err)
	}

	return time.Now().UTC().Format("20060102T150405.000000000Z") + "-" + hex.EncodeToString(random[:]), nil
}
