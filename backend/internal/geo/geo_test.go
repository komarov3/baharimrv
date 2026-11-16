package geo

import (
	"encoding/json"
	"math"
	"testing"
)

// squareRing builds a closed square of the given side in metres centred on (lat, lon).
func squareRing(lat, lon, side float64) Ring {
	dlat := side / 2 / MetresPerDegreeLat
	cosLat := math.Cos(lat * math.Pi / 180)
	dlon := side / 2 / (MetresPerDegreeLat * cosLat)
	w, e := lon-dlon, lon+dlon
	s, n := lat-dlat, lat+dlat
	return Ring{{w, s}, {e, s}, {e, n}, {w, n}, {w, s}}
}

func raw(ring Ring) []byte {
	b, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": []Ring{ring}})
	return b
}

func TestParsePolygonAcceptsClosedSquare(t *testing.T) {
	ring, err := ParsePolygon(raw(squareRing(-4.05, 39.67, 100)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ring) != 5 {
		t.Fatalf("expected 5 points, got %d", len(ring))
	}
}

func TestParsePolygonRejectsBadShapes(t *testing.T) {
	cases := map[string][]byte{
		"not json":      []byte("{"),
		"not polygon":   []byte(`{"type":"Point","coordinates":[39.67,-4.05]}`),
		"two rings":     []byte(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,0]],[[0,0],[1,0],[1,1],[0,0]]]}`),
		"too few":       []byte(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[0,0]]]}`),
		"unclosed":      []byte(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`),
		"out of range":  []byte(`{"type":"Polygon","coordinates":[[[0,0],[200,0],[200,1],[0,0]]]}`),
	}
	for name, body := range cases {
		if _, err := ParsePolygon(body); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

func TestAreaHectaresOf100mSquareIsOneHectare(t *testing.T) {
	for _, lat := range []float64{0, -4.05, 51.5} {
		got := AreaHectares(squareRing(lat, 39.67, 100))
		if math.Abs(got-1.0) > 1e-6 {
			t.Errorf("lat %v: expected ~1 ha, got %v", lat, got)
		}
	}
}

func TestCentroidLandsAtSquareCentre(t *testing.T) {
	lat, lon := Centroid(squareRing(-4.05, 39.67, 100))
	if math.Abs(lat-(-4.05)) > 1e-9 || math.Abs(lon-39.67) > 1e-9 {
		t.Errorf("expected (-4.05, 39.67), got (%v, %v)", lat, lon)
	}
}

func TestCentroidOfDegenerateRingIsVertexMean(t *testing.T) {
	ring := Ring{{0, 0}, {0, 0}, {0, 0}, {0, 0}}
	lat, lon := Centroid(ring)
	if lat != 0 || lon != 0 {
		t.Errorf("expected (0,0), got (%v,%v)", lat, lon)
	}
}