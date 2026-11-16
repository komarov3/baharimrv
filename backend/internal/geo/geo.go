// Package geo holds the plot geometry shared by the API write path and the satellite
// analysis cells. The same area and centroid math is mirrored in s2_worker/aoi.py and
// the Android capture screen, so a boundary recorded on a phone yields the same hectares
// and the same analysis outline on every layer.
package geo

import (
	"encoding/json"
	"fmt"
	"math"
)

// MetresPerDegreeLat matches the convention used by the buffered-square fallback.
const MetresPerDegreeLat = 111320.0

// Ring is a closed linear ring of [lon, lat] pairs in EPSG:4326.
type Ring [][2]float64

type polygon struct {
	Type        string `json:"type"`
	Coordinates []Ring `json:"coordinates"`
}

// ParsePolygon validates a GeoJSON Polygon and returns its outer ring. Holes are
// rejected for now: a plot is one surveyed outline. The ring must be closed, carry at
// least three vertices plus a closing point, and stay inside lon/lat ranges.
func ParsePolygon(raw []byte) (Ring, error) {
	var p polygon
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, fmt.Errorf("boundary is not valid GeoJSON: %w", err)
	}
	if p.Type != "Polygon" {
		return nil, fmt.Errorf("boundary must be a GeoJSON Polygon, got %q", p.Type)
	}
	if len(p.Coordinates) != 1 {
		return nil, fmt.Errorf("boundary must have exactly one ring, got %d", len(p.Coordinates))
	}
	ring := p.Coordinates[0]
	if len(ring) < 4 {
		return nil, fmt.Errorf("boundary needs at least 3 vertices plus a closing point")
	}
	first, last := ring[0], ring[len(ring)-1]
	if first[0] != last[0] || first[1] != last[1] {
		return nil, fmt.Errorf("boundary ring is not closed")
	}
	for _, v := range ring {
		if math.IsNaN(v[0]) || math.IsNaN(v[1]) || math.IsInf(v[0], 0) || math.IsInf(v[1], 0) {
			return nil, fmt.Errorf("boundary has a non-finite coordinate")
		}
		if v[0] < -180 || v[0] > 180 || v[1] < -90 || v[1] > 90 {
			return nil, fmt.Errorf("boundary coordinate outside lon/lat ranges")
		}
	}
	return ring, nil
}

// AreaHectares is the polygon area in hectares via a local equirectangular projection
// and the shoelace formula. It is an approximation consistent with the square fallback
// and adequate for plots up to a few kilometres across.
func AreaHectares(ring Ring) float64 {
	x, y := project(ring)
	var sum float64
	for i := 0; i < len(x)-1; i++ {
		sum += x[i]*y[i+1] - x[i+1]*y[i]
	}
	return math.Abs(sum) / 2 / 10_000
}

// Centroid returns the area-weighted centroid as (lat, lon). A degenerate ring falls
// back to the vertex mean so a pin still lands on the recorded outline.
func Centroid(ring Ring) (lat, lon float64) {
	lat0, lon0 := mean(ring)
	x, y := projectAround(ring, lat0, lon0)
	var a, cx, cy float64
	for i := 0; i < len(x)-1; i++ {
		cross := x[i]*y[i+1] - x[i+1]*y[i]
		a += cross
		cx += (x[i] + x[i+1]) * cross
		cy += (y[i] + y[i+1]) * cross
	}
	if a == 0 {
		return lat0, lon0
	}
	a /= 2
	cx /= 6 * a
	cy /= 6 * a
	cosLat := math.Cos(lat0 * math.Pi / 180)
	if cosLat == 0 {
		cosLat = 1
	}
	return lat0 + cy/MetresPerDegreeLat, lon0 + cx/(MetresPerDegreeLat*cosLat)
}

func mean(ring Ring) (lat, lon float64) {
	n := len(ring) - 1
	if n < 1 {
		n = len(ring)
	}
	for i := 0; i < n; i++ {
		lon += ring[i][0]
		lat += ring[i][1]
	}
	return lat / float64(n), lon / float64(n)
}

func project(ring Ring) (x, y []float64) {
	lat0, lon0 := mean(ring)
	return projectAround(ring, lat0, lon0)
}

func projectAround(ring Ring, lat0, lon0 float64) (x, y []float64) {
	cosLat := math.Cos(lat0 * math.Pi / 180)
	if cosLat == 0 {
		cosLat = 1
	}
	x = make([]float64, len(ring))
	y = make([]float64, len(ring))
	for i, v := range ring {
		x[i] = (v[0] - lon0) * cosLat * MetresPerDegreeLat
		y[i] = (v[1] - lat0) * MetresPerDegreeLat
	}
	return x, y
}