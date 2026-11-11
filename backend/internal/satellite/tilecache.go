// Bounded in-memory cache for rendered tiles. A pan across a small AOI asks for
// the same handful of PNGs over and over; an LRU in front of the volume store
// absorbs those repeats without re-reading the disk.
//
// The cache is keyed by the full tile address (project, scene, z, x, y) rather
// than by the pixel content, because two scenes for the same project are
// different layers and must not share bytes. An eviction removes the least
// recently used entry; a hit moves it to the front of the list so a busy tile
// stays warm.
//
// Hit and miss counters are exposed for the admin health route. They are
// updated under the same mutex that guards the list, so a probe cannot race a
// write and report a number that never happened.
package satellite

import (
	"container/list"
	"fmt"
	"sync"
	"sync/atomic"
)

// DefaultTileCacheEntries is the number of rendered PNGs kept warm. It is sized
// for one manager panning across a single AOI at two or three zooms: enough to
// absorb the back-and-forth, small enough that a megabyte render cannot pin
// the whole store in memory.
const DefaultTileCacheEntries = 256

// TileCache is a read-through LRU in front of a TileStore. A nil inner store
// means tiles were never configured; the cache then refuses every read so the
// proxy returns 410 rather than an error.
type TileCache struct {
	inner TileStore

	mu       sync.Mutex
	capacity int
	items    map[string]*list.Element
	order    *list.List

	hits   atomic.Int64
	misses atomic.Int64
}

type cacheEntry struct {
	key  string
	data []byte
}

// NewTileCache wraps an inner store (which may be nil when the deployment did
// not name a tile root). Capacity is floored at one so a misconfiguration
// cannot produce a panic on the first read.
func NewTileCache(inner TileStore, capacity int) *TileCache {
	if capacity < 1 {
		capacity = 1
	}
	return &TileCache{
		inner:    inner,
		capacity: capacity,
		items:    make(map[string]*list.Element, capacity),
		order:    list.New(),
	}
}

// Get returns a cached PNG or reads it from the store. Cache misses propagate
// ErrTileNotFound unchanged so the proxy's 410 path keeps working; every other
// inner error reaches the caller as-is.
func (c *TileCache) Get(keyPrefix string, z, x, y int) ([]byte, error) {
	key := tileKey(keyPrefix, z, x, y)
	c.mu.Lock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		bytes := elem.Value.(*cacheEntry).data
		c.mu.Unlock()
		c.hits.Add(1)
		return bytes, nil
	}
	c.mu.Unlock()
	c.misses.Add(1)
	if c.inner == nil {
		return nil, ErrTileNotFound
	}
	data, err := c.inner.Get(keyPrefix, z, x, y)
	if err != nil {
		return nil, err
	}
	c.store(key, data)
	return data, nil
}

// Available reports whether there is anything to read through. The cache does
// not mask an absent store; a nil inner keeps the overlay honest about the
// missing tile base URL.
func (c *TileCache) Available() bool { return c.inner != nil && c.inner.Available() }

// Stats is the pair of counters the admin route prints. They are int64 so the
// JSON envelope stays a number, not a struct.
type CacheStats struct {
	Hits     int64 `json:"hits"`
	Misses   int64 `json:"misses"`
	Capacity int   `json:"capacity"`
	Entries  int   `json:"entries"`
}

// Stats reads the counters and the current entry count. It is safe to call
// while reads are in flight.
func (c *TileCache) Stats() CacheStats {
	c.mu.Lock()
	entries := len(c.items)
	c.mu.Unlock()
	return CacheStats{
		Hits: c.hits.Load(), Misses: c.misses.Load(),
		Capacity: c.capacity, Entries: entries,
	}
}

func (c *TileCache) store(key string, data []byte) {
	copied := append([]byte(nil), data...)
	c.mu.Lock()
	defer c.mu.Unlock()
	if elem, ok := c.items[key]; ok {
		c.order.MoveToFront(elem)
		elem.Value.(*cacheEntry).data = copied
		return
	}
	elem := c.order.PushFront(&cacheEntry{key: key, data: copied})
	c.items[key] = elem
	for c.order.Len() > c.capacity {
		back := c.order.Back()
		if back == nil {
			break
		}
		c.order.Remove(back)
		delete(c.items, back.Value.(*cacheEntry).key)
	}
}

func tileKey(prefix string, z, x, y int) string {
	return fmt.Sprintf("%s/%d/%d/%d", prefix, z, x, y)
}
