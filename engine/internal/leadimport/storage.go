package leadimport

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type Storage struct {
	root string
}

func NewStorage(root string) (*Storage, error) {
	if root == "" {
		root = "/data/imports"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("import storage init %s: %w", root, err)
	}
	return &Storage{root: root}, nil
}

func (s *Storage) Root() string { return s.root }

func (s *Storage) Write(tenantID int64, fileKey string, body io.Reader) (int64, error) {
	if fileKey == "" {
		return 0, errors.New("import storage: empty file_key")
	}
	dir := filepath.Join(s.root, strconv.FormatInt(tenantID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, fileKey)
	f, err := os.Create(path)
	if err != nil {
		return 0, fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	n, err := io.Copy(f, body)
	if err != nil {
		_ = os.Remove(path)
		return 0, fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(path)
		return 0, fmt.Errorf("fsync %s: %w", path, err)
	}
	return n, nil
}

func (s *Storage) Open(tenantID int64, fileKey string) (*os.File, error) {
	path := filepath.Join(s.root, strconv.FormatInt(tenantID, 10), fileKey)
	return os.Open(path)
}

func (s *Storage) Delete(tenantID int64, fileKey string) error {
	path := filepath.Join(s.root, strconv.FormatInt(tenantID, 10), fileKey)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
