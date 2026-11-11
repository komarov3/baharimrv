package satellite

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestVolumeStoreRefusesAnEmptyRoot(t *testing.T) {
	if store := NewVolumeStore(""); store != nil {
		t.Fatalf("empty root should produce a nil store, got %#v", store)
	}
	if store := NewVolumeStore("   "); store != nil {
		t.Fatalf("whitespace root should be treated as empty, got %#v", store)
	}
}

func TestVolumeStoreAvailableRequiresARoot(t *testing.T) {
	var store *VolumeStore
	if store.Available() {
		t.Fatalf("nil store must not report available")
	}
	store = NewVolumeStore(t.TempDir())
	if !store.Available() {
		t.Fatalf("configured store must report available")
	}
}

// A missing file answers the typed sentinel so the proxy can translate it
// into 410 without string-matching an os.PathError.
func TestVolumeStoreMissingFileIsTyped(t *testing.T) {
	store := NewVolumeStore(t.TempDir())
	_, err := store.Get("project/scene", 14, 9012, 7234)
	if !errors.Is(err, ErrTileNotFound) {
		t.Fatalf("missing tile = %v, want ErrTileNotFound", err)
	}
}

// The read path is rebuilt from parts rather than concatenated, so a prefix
// carrying a traversal is refused before any file is opened. Slashes are the
// expected separator the worker uses (project_id + "/" + scene_id).
func TestVolumeStoreRefusesAHostilePrefix(t *testing.T) {
	store := NewVolumeStore(t.TempDir())
	for _, bad := range []string{"../etc", "project/..", "a\\b"} {
		if _, err := store.Get(bad, 14, 0, 0); err == nil {
			t.Errorf("prefix %q should have been refused", bad)
		}
	}
}

// A real PNG round-trips through the store unchanged. The bytes come from a
// temporary file so the test exercises the same os.ReadFile path production
// uses, not an in-memory stub.
func TestVolumeStoreReadsARealTile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "project", "scene", "14", "9012", "7234.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	store := NewVolumeStore(root)
	got, err := store.Get("project/scene", 14, 9012, 7234)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("read %d bytes, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %#x, want %#x", i, got[i], want[i])
		}
	}
}
