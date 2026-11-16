package mrv

import (
	"encoding/json"
	"math"
	"testing"
)

func boundaryJSON(lat, lon, side float64) json.RawMessage {
	dlat := side / 2 / 111320.0
	cosLat := math.Cos(lat * math.Pi / 180)
	dlon := side / 2 / (111320.0 * cosLat)
	w, e := lon-dlon, lon+dlon
	s, n := lat-dlat, lat+dlat
	ring := [][]float64{{w, s}, {e, s}, {e, n}, {w, n}, {w, s}}
	b, _ := json.Marshal(map[string]any{"type": "Polygon", "coordinates": []any{ring}})
	return b
}

func TestResolvePlotWithoutBoundaryUsesStatedArea(t *testing.T) {
	r, err := resolvePlot("P1", PlotInput{Code: "P1", Latitude: -4.05, Longitude: 39.67, AreaHectares: "2.5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.AreaHectares != "2.500000" || r.Latitude != -4.05 || r.Longitude != 39.67 {
		t.Fatalf("expected stated area and recorded point, got %+v", r)
	}
	if r.Boundary != nil {
		t.Fatalf("expected no boundary, got %s", r.Boundary)
	}
}

func TestResolvePlotWithBoundaryDerivesAreaAndCentroid(t *testing.T) {
	input := PlotInput{Code: "P1", Latitude: 0, Longitude: 0, AreaHectares: "9.9",
		Boundary: boundaryJSON(-4.05, 39.67, 100)}
	r, err := resolvePlot("P1", input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.AreaHectares != "1.000000" {
		t.Fatalf("expected derived 1 ha, got %s", r.AreaHectares)
	}
	if math.Abs(r.Latitude-(-4.05)) > 1e-7 || math.Abs(r.Longitude-39.67) > 1e-7 {
		t.Fatalf("expected centroid (-4.05, 39.67), got (%v, %v)", r.Latitude, r.Longitude)
	}
	if len(r.Boundary) == 0 {
		t.Fatal("expected boundary to be stored")
	}
}

func TestResolvePlotRejectsBadInput(t *testing.T) {
	cases := map[string]PlotInput{
		"unclosed boundary": {Code: "P1", AreaHectares: "1",
			Boundary: json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[1,0],[1,1],[0,1]]]}`)},
		"zero area boundary": {Code: "P1", AreaHectares: "1",
			Boundary: json.RawMessage(`{"type":"Polygon","coordinates":[[[0,0],[0,0],[0,0],[0,0]]]}`)},
		"negative accuracy": {Code: "P1", AreaHectares: "1", GpsAccuracyM: ptr(-1)},
		"out of range point": {Code: "P1", Latitude: 999, AreaHectares: "1"},
		"invalid stated area": {Code: "P1", AreaHectares: "abc"},
	}
	for name, input := range cases {
		if _, err := resolvePlot(name, input); err == nil {
			t.Errorf("%s: expected rejection", name)
		}
	}
}

func ptr(v float64) *float64 { return &v }