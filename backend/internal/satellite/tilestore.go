// Tile storage. The worker writes rendered PNGs into a shared volume at
// `{root}/{project_id}/{scene_id}/{z}/{x}/{y}.png`, and this package reads them
// back. The root is a deployment-time constant (a Fly volume in production, a
// tmpfs on a developer's box); nothing here knows the address of a bucket or a
// credential, so a caller that holds a scene identifier cannot reach anything
// outside the prefix the API named.
//
// The store is a narrow read surface, not a general object client. The proxy
// asks for one file at a time; an LRU in front of this keeps a pan across a
// small AOI from hitting the volume per tile.
package satellite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrTileNotFound is the store's honest empty answer: no PNG at that address.
// The proxy translates it to 410, which tells the client to re-request the
// overlay instead of painting a gap.
var ErrTileNotFound = errors.New("satellite tile not found")

// TileStore is the read surface the proxy relies on. It stays small on purpose:
// writes happen inside the worker, and the only thing an API caller can ask for
// is one PNG at a known address.
type TileStore interface {
	// Get returns the PNG bytes at key_prefix/{z}/{x}/{y}.png. ErrTileNotFound is
	// the only typed refusal; every other error is a fault the proxy escalates.
	Get(keyPrefix string, z, x, y int) ([]byte, error)
	// Available reports whether the store has anything to read from at all. A
	// misconfigured store answers false and the overlay omits tileBaseUrl so the
	// client never asks for pixels the proxy cannot serve.
	Available() bool
}

// VolumeStore reads PNGs from a filesystem root shared with the worker. The
// root is required: a store with no root pretends not to exist rather than
// reading from whatever the process happens to consider its current directory.
type VolumeStore struct {
	root string
}

// NewVolumeStore builds a store from a configured root. An empty root is a
// deliberate "no tiles" signal, not a misconfiguration, so the caller gets nil
// back and every downstream check short-circuits cleanly.
func NewVolumeStore(root string) *VolumeStore {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	return &VolumeStore{root: root}
}

// Available is true when a root was named. It does not stat the directory on
// every call: a missing root at probe time is an operator problem that the
// admin route surfaces separately, and a per-call stat would not change the
// proxy's answer to a real tile request.
func (s *VolumeStore) Available() bool { return s != nil && s.root != "" }

// Get reads one PNG. The key prefix and tile coordinates come from the worker's
// output and a client's request, both of which are identifiers built from
// externally supplied text, so the path is rebuilt from parts rather than
// concatenated.
func (s *VolumeStore) Get(keyPrefix string, z, x, y int) ([]byte, error) {
	if s == nil || s.root == "" {
		return nil, ErrTileNotFound
	}
	if keyPrefix == "" {
		return nil, ErrTileNotFound
	}
	// The prefix and tile coordinates come from the worker's output and a
	// client's request. The prefix is built as project_id + "/" + scene_id,
	// which carries exactly one slash, and z, x, y are integers. Any backslash
	// or ".." is hostile and is refused before any path is assembled.
	if strings.ContainsAny(keyPrefix, `\`) || strings.Contains(keyPrefix, "..") {
		return nil, fmt.Errorf("%q cannot name a tile prefix", keyPrefix)
	}
	rel := filepath.Join(keyPrefix, fmt.Sprintf("%d", z), fmt.Sprintf("%d", x), fmt.Sprintf("%d.png", y))
	path := filepath.Join(s.root, rel)
	// The joined path must still sit under the configured root. A future
	// refactor that swaps the prefix or the worker's layout should fail loudly
	// rather than read outside the volume.
	cleanRoot := filepath.Clean(s.root)
	cleanPath := filepath.Clean(path)
	if cleanPath != cleanRoot && !strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) {
		return nil, fmt.Errorf("tile path %q escapes root", rel)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrTileNotFound
		}
		return nil, fmt.Errorf("read tile %s: %w", rel, err)
	}
	return data, nil
}
