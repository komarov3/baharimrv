// Tile storage reads worker-rendered PNGs over authenticated HTTP or a local shared volume.
package satellite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
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

const maxTileBytes = 1 << 20

var workerTilePrefix = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}/[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type WorkerTileStore struct {
	baseURL string
	token   string
	client  *http.Client
}

func NewWorkerTileStore(baseURL, token string) *WorkerTileStore {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	token = strings.TrimSpace(token)
	endpoint, err := url.Parse(baseURL)
	if err != nil || endpoint.Host == "" ||
		(endpoint.Scheme != "http" && endpoint.Scheme != "https") ||
		endpoint.User != nil || endpoint.RawQuery != "" || endpoint.Fragment != "" || token == "" {
		return nil
	}
	return &WorkerTileStore{
		baseURL: baseURL,
		token:   token,
		client: &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (s *WorkerTileStore) Available() bool {
	return s != nil && s.baseURL != "" && s.token != ""
}

func (s *WorkerTileStore) Get(keyPrefix string, z, x, y int) ([]byte, error) {
	if !s.Available() {
		return nil, ErrTileNotFound
	}
	if !workerTilePrefix.MatchString(keyPrefix) {
		return nil, errors.New("invalid worker tile prefix")
	}
	if z < 8 || z > 16 || x < 0 || y < 0 || x >= 1<<z || y >= 1<<z {
		return nil, errors.New("invalid worker tile coordinates")
	}
	parts := strings.SplitN(keyPrefix, "/", 2)
	address := s.baseURL + "/v1/tiles/" + url.PathEscape(parts[0]) + "/" + url.PathEscape(parts[1]) +
		fmt.Sprintf("/%d/%d/%d.png", z, x, y)
	request, err := http.NewRequest(http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("build worker tile request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+s.token)
	request.Header.Set("Accept", "image/png")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read worker tile: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrTileNotFound
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("worker tile returned HTTP %d", response.StatusCode)
	}
	mediaType, _, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || mediaType != "image/png" {
		return nil, errors.New("worker tile response is not image/png")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxTileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read worker tile body: %w", err)
	}
	if len(data) > maxTileBytes {
		return nil, errors.New("worker tile exceeds size limit")
	}
	if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
		return nil, errors.New("worker tile has an invalid PNG signature")
	}
	return data, nil
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
