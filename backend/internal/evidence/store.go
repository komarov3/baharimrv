// Package evidence stores captured media off-chain and restricts it to authorised
// downloads. Files are content-addressed so a hash mismatch is detectable after
// upload, and stored paths can never escape the evidence root.
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrTooLarge      = errors.New("evidence file exceeds the permitted size")
	ErrHashMismatch  = errors.New("evidence file hash does not match the manifest")
	ErrUnknownFile   = errors.New("evidence file is unavailable")
	ErrInvalidDigest = errors.New("manifest sha256 is not a 64-character hex digest")
)

type Store struct {
	root     string
	maxBytes int64
}

func NewStore(root string, maxBytes int64) (*Store, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(absolute, 0o750); err != nil {
		return nil, fmt.Errorf("create evidence root: %w", err)
	}
	if maxBytes <= 0 {
		maxBytes = 25 * 1024 * 1024
	}
	return &Store{root: absolute, maxBytes: maxBytes}, nil
}

func (s *Store) Root() string { return s.root }

// Put streams the upload to a temporary file, hashes it while writing, and only
// then compares against the manifest digest. The declared hash is never trusted.
func (s *Store) Put(scope, fileID, declaredSHA string, reader io.Reader) (string, string, int64, error) {
	digest, err := normaliseDigest(declaredSHA)
	if err != nil {
		return "", "", 0, err
	}
	if !isSafeSegment(scope) || !isSafeSegment(fileID) {
		return "", "", 0, ErrUnknownFile
	}
	directory := filepath.Join(s.root, scope)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return "", "", 0, fmt.Errorf("prepare evidence directory: %w", err)
	}
	target := filepath.Join(directory, fileID)
	temporary, err := os.CreateTemp(directory, ".upload-*")
	if err != nil {
		return "", "", 0, fmt.Errorf("open evidence scratch file: %w", err)
	}
	defer os.Remove(temporary.Name())
	defer temporary.Close()

	hasher := sha256.New()
	limited := io.TeeReader(io.LimitReader(reader, s.maxBytes+1), hasher)
	written, err := io.Copy(temporary, limited)
	if err != nil {
		return "", "", 0, fmt.Errorf("write evidence file: %w", err)
	}
	if written > s.maxBytes {
		return "", "", 0, ErrTooLarge
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != digest {
		return "", "", 0, ErrHashMismatch
	}
	if err := temporary.Close(); err != nil {
		return "", "", 0, fmt.Errorf("close evidence file: %w", err)
	}
	if err := os.Rename(temporary.Name(), target); err != nil {
		return "", "", 0, fmt.Errorf("store evidence file: %w", err)
	}
	return "file://" + target, actual, written, nil
}

// Open returns the stored file. Stored URIs are resolved strictly inside the root.
func (s *Store) Open(uri string) (io.ReadCloser, int64, error) {
	if !strings.HasPrefix(uri, "file://") {
		return nil, 0, ErrUnknownFile
	}
	target := filepath.Clean(strings.TrimPrefix(uri, "file://"))
	root := filepath.Clean(s.root)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return nil, 0, ErrUnknownFile
	}
	info, err := os.Stat(target)
	if err != nil {
		return nil, 0, ErrUnknownFile
	}
	handle, err := os.Open(target)
	if err != nil {
		return nil, 0, ErrUnknownFile
	}
	return handle, info.Size(), nil
}

func normaliseDigest(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return "", ErrInvalidDigest
	}
	if _, err := hex.DecodeString(value); err != nil {
		return "", ErrInvalidDigest
	}
	return value, nil
}

func isSafeSegment(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, symbol := range value {
		allowed := symbol >= 'a' && symbol <= 'z' || symbol >= 'A' && symbol <= 'Z' ||
			symbol >= '0' && symbol <= '9' || symbol == '-' || symbol == '_' || symbol == '.'
		if !allowed {
			return false
		}
	}
	return value != "." && value != ".." && !strings.Contains(value, "..")
}
