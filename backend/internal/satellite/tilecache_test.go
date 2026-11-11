package satellite

import (
	"errors"
	"fmt"
	"testing"
)

// fakeStore is a TileStore backed by a map. It records the read count so a
// test can assert that a cache hit never touched the underlying volume.
type fakeStore struct {
	tiles map[string][]byte
	reads int
}

func (f *fakeStore) Available() bool { return true }

func (f *fakeStore) Get(prefix string, z, x, y int) ([]byte, error) {
	f.reads++
	key := fmt.Sprintf("%s/%d/%d/%d", prefix, z, x, y)
	data, ok := f.tiles[key]
	if !ok {
		return nil, ErrTileNotFound
	}
	return data, nil
}

func TestTileCacheRecordsHitsAndMisses(t *testing.T) {
	store := &fakeStore{tiles: map[string][]byte{"p/14/1/2": {1, 2, 3}}}
	cache := NewTileCache(store, 4)

	if _, err := cache.Get("p", 14, 1, 2); err != nil {
		t.Fatalf("first read: %v", err)
	}
	if _, err := cache.Get("p", 14, 1, 2); err != nil {
		t.Fatalf("second read: %v", err)
	}
	if store.reads != 1 {
		t.Fatalf("cache did not absorb the repeat read, got %d reads", store.reads)
	}
	if stats := cache.Stats(); stats.Hits != 1 || stats.Misses != 1 {
		t.Fatalf("stats = %+v, want hits=1 misses=1", stats)
	}
}

// Eviction is the whole reason an LRU exists: when the cache is full, the
// least recently used entry gives way so a hot tile can stay warm.
func TestTileCacheEvictsTheLeastRecentlyUsed(t *testing.T) {
	store := &fakeStore{tiles: map[string][]byte{
		"p/14/1/1": {1}, "p/14/1/2": {2}, "p/14/1/3": {3},
	}}
	cache := NewTileCache(store, 2)

	for _, coord := range []int{1, 2, 3} {
		if _, err := cache.Get("p", 14, 1, coord); err != nil {
			t.Fatalf("warm read %d: %v", coord, err)
		}
	}
	// The first coord was evicted by the third write. Reading it again must
	// touch the store rather than the cache.
	before := store.reads
	if _, err := cache.Get("p", 14, 1, 1); err != nil {
		t.Fatalf("re-read after eviction: %v", err)
	}
	if store.reads != before+1 {
		t.Fatalf("evicted entry was not re-read from the store, reads went %d -> %d", before, store.reads)
	}
}

// A cache must not invent a PNG for a tile that does not exist. The typed
// sentinel reaches the caller unchanged so the proxy can map it to 410.
func TestTileCachePropagatesMissing(t *testing.T) {
	store := &fakeStore{tiles: map[string][]byte{}}
	cache := NewTileCache(store, 4)
	_, err := cache.Get("p", 14, 1, 2)
	if !errors.Is(err, ErrTileNotFound) {
		t.Fatalf("missing tile = %v, want ErrTileNotFound", err)
	}
}

// A nil inner store means tiles were never configured. The cache refuses
// every read rather than panicking on the first one.
func TestTileCacheWithNoStoreRefusesReads(t *testing.T) {
	cache := NewTileCache(nil, 4)
	if cache.Available() {
		t.Fatalf("cache with no store must not report available")
	}
	_, err := cache.Get("p", 14, 1, 2)
	if !errors.Is(err, ErrTileNotFound) {
		t.Fatalf("read through nil store = %v, want ErrTileNotFound", err)
	}
}

// A misconfigured capacity cannot crash the cache. One entry is the floor so
// a zero capacity still admits a single read.
func TestTileCacheFloorsCapacity(t *testing.T) {
	store := &fakeStore{tiles: map[string][]byte{"p/14/1/2": {1}}}
	cache := NewTileCache(store, 0)
	if _, err := cache.Get("p", 14, 1, 2); err != nil {
		t.Fatalf("read with floored capacity: %v", err)
	}
}
