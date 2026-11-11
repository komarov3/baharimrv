package satellite

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerTileStoreRequiresConfiguration(t *testing.T) {
	for _, config := range [][2]string{
		{"", "token"}, {"http://worker.internal:9000", ""},
		{"http://worker.internal:9000", "   "}, {"not-a-url", "token"},
		{"file:///data/tiles", "token"}, {"http://user:pass@worker", "token"},
		{"http://worker?token=other", "token"}, {"http://worker#fragment", "token"},
		{"http://%invalid", "token"},
	} {
		store := NewWorkerTileStore(config[0], config[1])
		if store.Available() {
			t.Errorf("invalid configuration reports available: %q", config[0])
		}
		if _, err := store.Get("project/scene", 14, 1, 1); !errors.Is(err, ErrTileNotFound) {
			t.Errorf("unconfigured read = %v", err)
		}
	}
	store := NewWorkerTileStore(" http://worker.internal:9000/ ", " token ")
	if !store.Available() || store.baseURL != "http://worker.internal:9000" || store.token != "token" {
		t.Fatal("valid configuration was not normalized")
	}
	if store.client.Timeout <= 0 || store.client.Timeout > 10*time.Second {
		t.Fatal("tile reads need a bounded timeout")
	}
}

func TestWorkerTileStoreReadsAuthenticatedPNGThroughCache(t *testing.T) {
	want := []byte("\x89PNG\r\n\x1a\n")
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/v1/tiles/project/scene:1/14/9012/7234.png" ||
			r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer test-token" ||
			r.Header.Get("Accept") != "image/png" {
			t.Error("worker request did not match authenticated tile contract")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(want)
	}))
	defer worker.Close()
	cache := NewTileCache(NewWorkerTileStore(worker.URL, "test-token"), 4)
	for i := 0; i < 2; i++ {
		got, err := cache.Get("project/scene:1", 14, 9012, 7234)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("read = %x, %v", got, err)
		}
	}
	if calls.Load() != 1 || cache.Stats().Hits != 1 {
		t.Fatal("repeat tile read did not use cache")
	}
}

func TestWorkerTileStoreRejectsInvalidKeysBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer worker.Close()
	store := NewWorkerTileStore(worker.URL, "test-token")
	for _, prefix := range []string{"", "../scene", "project/..", "/project/scene", "a/b/c", "a\\b/c", "p/s?x=1", "p/s#fragment", "p/%2e%2e", "p/s\n", strings.Repeat("a", 65) + "/s", "p/" + strings.Repeat("s", 129)} {
		if _, err := store.Get(prefix, 14, 0, 0); err == nil {
			t.Errorf("accepted prefix %q", prefix)
		}
	}
	for _, coords := range [][3]int{{7, 0, 0}, {17, 0, 0}, {14, -1, 0}, {14, 0, -1}, {14, 1 << 14, 0}, {14, 0, 1 << 14}, {100, 0, 0}} {
		if _, err := store.Get("project/scene", coords[0], coords[1], coords[2]); err == nil {
			t.Errorf("accepted coordinates %v", coords)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("invalid key reached worker")
	}
}

func TestWorkerTileStoreDoesNotCacheErrors(t *testing.T) {
	for _, code := range []int{400, 401, 403, 404, 410, 429, 500, 503} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			var calls atomic.Int32
			worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(code)
				_, _ = w.Write([]byte("private worker error details"))
			}))
			defer worker.Close()
			cache := NewTileCache(NewWorkerTileStore(worker.URL, "test-token"), 4)
			for i := 0; i < 2; i++ {
				_, err := cache.Get("project/scene", 14, 1, 1)
				if err == nil || errors.Is(err, ErrTileNotFound) != (code == http.StatusNotFound) {
					t.Fatalf("HTTP %d error = %v", code, err)
				}
				if strings.Contains(err.Error(), "private worker") || strings.Contains(err.Error(), "test-token") {
					t.Fatal("error leaked worker response or credential")
				}
			}
			if calls.Load() != 2 || cache.Stats().Hits != 0 {
				t.Fatal("failed reads were cached")
			}
		})
	}
}

func TestWorkerTileStoreRejectsRedirectsAndInvalidBodies(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n")
	for _, test := range []struct {
		name        string
		contentType string
		body        []byte
	}{
		{"html", "text/html", []byte("<html>login</html>")},
		{"json", "application/json", []byte(`{"error":"unavailable"}`)},
		{"wrong-signature", "image/png", []byte("not a png")},
		{"empty", "image/png", nil},
		{"oversized", "image/png", append(png, make([]byte, maxTileBytes)...)},
	} {
		t.Run(test.name, func(t *testing.T) {
			worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", test.contentType)
				_, _ = w.Write(test.body)
			}))
			defer worker.Close()
			if _, err := NewWorkerTileStore(worker.URL, "test-token").Get("project/scene", 14, 0, 0); err == nil {
				t.Fatal("invalid worker response was accepted")
			}
		})
	}
	var followed atomic.Int32
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			followed.Add(1)
		}
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer worker.Close()
	_, err := NewWorkerTileStore(worker.URL, "test-token").Get("project/scene", 14, 0, 0)
	if err == nil || followed.Load() != 0 {
		t.Fatal("worker redirect was followed")
	}
}

func TestWorkerTileStoreNetworkAndTruncatedBodyErrors(t *testing.T) {
	worker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Content-Length", "100")
		_, _ = w.Write([]byte("\x89PNG\r\n\x1a\n"))
	}))
	store := NewWorkerTileStore(worker.URL, "test-token")
	if _, err := store.Get("project/scene", 14, 0, 0); err == nil || errors.Is(err, ErrTileNotFound) {
		t.Fatalf("truncated body error = %v", err)
	}
	worker.Close()
	if _, err := store.Get("project/scene", 14, 0, 0); err == nil || errors.Is(err, ErrTileNotFound) {
		t.Fatalf("network error = %v", err)
	}
}

func TestServiceSelectsWorkerTileReader(t *testing.T) {
	for _, test := range []struct {
		name      string
		opts      Options
		remote    bool
		available bool
	}{
		{"remote", Options{Provider: ProviderOpenData, WorkerURL: "http://worker:9000", WorkerToken: "token"}, true, true},
		{"local-precedence", Options{Provider: ProviderOpenData, WorkerURL: "http://worker:9000", WorkerToken: "token", TilesRoot: t.TempDir()}, false, true},
		{"missing-token", Options{Provider: ProviderOpenData, WorkerURL: "http://worker:9000"}, true, false},
		{"missing-url", Options{Provider: ProviderOpenData, WorkerToken: "token"}, true, false},
		{"sandbox", Options{Provider: ProviderSandbox, WorkerURL: "http://worker:9000", WorkerToken: "token"}, false, false},
		{"unconfigured", Options{}, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(nil, test.opts)
			_, remote := service.tiles.inner.(*WorkerTileStore)
			if remote != test.remote || service.tiles.Available() != test.available {
				t.Fatalf("reader remote=%v available=%v", remote, service.tiles.Available())
			}
		})
	}
}

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
