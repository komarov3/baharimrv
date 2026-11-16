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

func TestPointInRingSquare(t *testing.T) {
	ring := squareRing(-4.05, 39.67, 200) // 200m box centred on (-4.05, 39.67)
	inside := [][2]float64{{-4.05, 39.67}, {-4.0505, 39.6695}, {-4.0495, 39.6705}}
	outside := [][2]float64{{-4.06, 39.67}, {-4.05, 39.69}, {0, 0}, {-4.04, 39.67}}
	for _, p := range inside {
		if !PointInRing(ring, p[0], p[1]) {
			t.Errorf("expected (%v,%v) inside the square", p[0], p[1])
		}
	}
	for _, p := range outside {
		if PointInRing(ring, p[0], p[1]) {
			t.Errorf("expected (%v,%v) outside the square", p[0], p[1])
		}
	}
}

func TestPointInRingConcave(t *testing.T) {
	// An L-shaped polygon: a 2x2 block with the top-right 1x1 removed.
	ring := Ring{{0, 0}, {2, 0}, {2, 1}, {1, 1}, {1, 2}, {0, 2}, {0, 0}} // [lon, lat]
	cases := []struct {
		lon, lat float64
		want     bool
	}{
		{0.5, 0.5, true},  // lower-left arm
		{0.5, 1.5, true},  // upper-left arm
		{1.5, 0.5, true},  // lower-right arm
		{1.5, 1.5, false}, // the notch that was cut out
		{2.5, 0.5, false}, // fully outside
	}
	for _, c := range cases {
		if got := PointInRing(ring, c.lat, c.lon); got != c.want {
			t.Errorf("point (lon=%v,lat=%v): got inside=%v want %v", c.lon, c.lat, got, c.want)
		}
	}
}

func TestPointInRingDegenerateIsNeverInside(t *testing.T) {
	if PointInRing(Ring{{0, 0}, {0, 0}}, 0, 0) {
		t.Error("a two-point ring must contain nothing")
	}
}