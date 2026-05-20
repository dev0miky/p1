package sound

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// Storage holds uploaded sound files on the local filesystem at
// {root}/{tenant_id}/{file_key}, where file_key is set by the caller (usually
// the sound's UUID + a known extension). The directory must be writable.
type Storage struct {
	root string
}

func NewStorage(root string) (*Storage, error) {
	if root == "" {
		root = "/data/sounds"
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, fmt.Errorf("sound storage init %s: %w", root, err)
	}
	return &Storage{root: root}, nil
}

func (s *Storage) Root() string { return s.root }

// WriteResult is returned by Write and carries the bytes written + sha256 hash
// of the stream. Useful for callers who want to persist the hash on the row.
type WriteResult struct {
	Size   int64
	SHA256 string
}

// Write copies the body bytes to disk and returns size + sha256.
// Body is closed by the caller; this just consumes it.
func (s *Storage) Write(tenantID int64, fileKey string, body io.Reader) (WriteResult, error) {
	if fileKey == "" {
		return WriteResult{}, errors.New("sound storage: empty file_key")
	}
	dir := filepath.Join(s.root, strconv.FormatInt(tenantID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, fileKey)
	f, err := os.Create(path)
	if err != nil {
		return WriteResult{}, fmt.Errorf("create %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), body)
	if err != nil {
		_ = os.Remove(path)
		return WriteResult{}, fmt.Errorf("write %s: %w", path, err)
	}
	if err := f.Sync(); err != nil {
		_ = os.Remove(path)
		return WriteResult{}, fmt.Errorf("fsync %s: %w", path, err)
	}
	return WriteResult{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// WriteTranscoded saves the uploaded audio (any ffmpeg-readable format) as an
// 8kHz 16-bit mono PCM WAV at {root}/{tenant_id}/{fileKey}. This is the
// telephony-native format, so FreeSWITCH plays it through mod_sndfile with no
// per-call resampling. fileKey must end in .wav.
func (s *Storage) WriteTranscoded(tenantID int64, fileKey string, body io.Reader) (WriteResult, error) {
	if fileKey == "" {
		return WriteResult{}, errors.New("sound storage: empty file_key")
	}
	dir := filepath.Join(s.root, strconv.FormatInt(tenantID, 10))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return WriteResult{}, fmt.Errorf("mkdir %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "upload-*")
	if err != nil {
		return WriteResult{}, fmt.Errorf("temp: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if _, err := io.Copy(tmp, body); err != nil {
		_ = tmp.Close()
		return WriteResult{}, fmt.Errorf("buffer upload: %w", err)
	}
	_ = tmp.Close()

	dest := filepath.Join(dir, fileKey)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error", "-y",
		"-i", tmpPath,
		"-ar", "8000", "-ac", "1", "-c:a", "pcm_s16le",
		dest,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(dest)
		return WriteResult{}, fmt.Errorf("transcode: %w: %s", err, string(out))
	}

	f, err := os.Open(dest)
	if err != nil {
		return WriteResult{}, fmt.Errorf("reopen %s: %w", dest, err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return WriteResult{}, fmt.Errorf("hash %s: %w", dest, err)
	}
	return WriteResult{Size: n, SHA256: hex.EncodeToString(h.Sum(nil))}, nil
}

// Open returns the sound bytes from disk, ready to stream. Caller closes.
func (s *Storage) Open(tenantID int64, fileKey string) (io.ReadSeekCloser, error) {
	path := filepath.Join(s.root, strconv.FormatInt(tenantID, 10), fileKey)
	return os.Open(path)
}

// Stat returns os.FileInfo for the underlying file (size, modtime).
func (s *Storage) Stat(tenantID int64, fileKey string) (os.FileInfo, error) {
	path := filepath.Join(s.root, strconv.FormatInt(tenantID, 10), fileKey)
	return os.Stat(path)
}

// Delete removes the file. Missing file is not an error.
func (s *Storage) Delete(tenantID int64, fileKey string) error {
	path := filepath.Join(s.root, strconv.FormatInt(tenantID, 10), fileKey)
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
