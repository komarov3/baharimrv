package satellite

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"

	"github.com/baharilink/baharimrv/internal/models"
)

// The cell arithmetic mirrors s2_worker/aoi.py, and has to: a fixture recorded by the
// worker is replayed against cells built here, so a divergence would move the sample
// off the plot a manager registered without either side reporting a fault.
const (
	// A 30 m floor guarantees at least 3x3 of the 10 m bands for a sub-0.1 ha record.
	minCellSideMetres  = 30.0
	metresPerDegreeLat = 111320.0
)

type polygonGeometry struct {
	Type        string         `json:"type"`
	Coordinates [][][2]float64 `json:"coordinates"`
}

type multiPolygonGeometry struct {
	Type        string           `json:"type"`
	Coordinates [][][][2]float64 `json:"coordinates"`
}

// cellRing is one plot's square sample, a closed counter-clockwise ring of [lon, lat]
// pairs whose area equals the plot's hectares.
//
// The square assumes the recorded centroid lies inside the plot. Centroid error larger
// than half the side moves the sample off the trees, which is why gps_accuracy_m is
// stored beside it and why a surveyed boundary, not a better buffer, is the phase 6
// answer.
func cellRing(lat, lon, areaHectares float64) [][2]float64 {
	side := math.Sqrt(math.Max(areaHectares, 0) * 10_000)
	if side < minCellSideMetres {
		side = minCellSideMetres
	}
	dlat := side / 2 / metresPerDegreeLat
	cosLat := math.Cos(lat * math.Pi / 180)
	if cosLat == 0 {
		cosLat = 1
	}
	dlon := side / 2 / (metresPerDegreeLat * cosLat)
	west, east := lon-dlon, lon+dlon
	south, north := lat-dlat, lat+dlat
	return [][2]float64{{west, south}, {east, south}, {east, north}, {west, north}, {west, south}}
}

// aoiFromPlots builds the analysis cells, the geometry they cover and the framing bbox.
//
// The geometry is a MultiPolygon of the cells, not a boolean union of them: a pixel mask
// counts a shared pixel once, so overlapping cells cover exactly the union, and this is
// the shape s2_worker's aoi.search_geometry already builds. Neither side needs a
// geometry library to agree.
//
// The bbox is framing only. Averaging over it would pull in the farmland, settlement and
// open water between scattered plots, which is the defect this whole method replaces.
func aoiFromPlots(plots []models.Plot) AOI {
	if len(plots) == 0 {
		return AOI{}
	}
	aoi := AOI{PlotCount: len(plots), Cells: make([]Cell, 0, len(plots))}
	polygons := make([][][][2]float64, 0, len(plots))
	for index, plot := range plots {
		area := plotArea(plot.AreaHectares)
		ring := cellRing(plot.Latitude, plot.Longitude, area)
		raw, _ := json.Marshal(polygonGeometry{Type: "Polygon", Coordinates: [][][2]float64{ring}})
		aoi.Cells = append(aoi.Cells, Cell{PlotID: plot.ID, Geometry: raw, AreaHectares: area})
		polygons = append(polygons, [][][2]float64{ring})
		aoi.CentroidLat += plot.Latitude
		aoi.CentroidLon += plot.Longitude
		west, south, east, north := ring[0][0], ring[0][1], ring[1][0], ring[2][1]
		if index == 0 {
			aoi.MinLon, aoi.MinLat, aoi.MaxLon, aoi.MaxLat = west, south, east, north
			continue
		}
		aoi.MinLon = math.Min(aoi.MinLon, west)
		aoi.MinLat = math.Min(aoi.MinLat, south)
		aoi.MaxLon = math.Max(aoi.MaxLon, east)
		aoi.MaxLat = math.Max(aoi.MaxLat, north)
	}
	aoi.CentroidLat /= float64(len(plots))
	aoi.CentroidLon /= float64(len(plots))
	aoi.FramingBBox = []float64{aoi.MinLon, aoi.MinLat, aoi.MaxLon, aoi.MaxLat}
	aoi.AnalysisGeom, _ = json.Marshal(multiPolygonGeometry{Type: "MultiPolygon", Coordinates: polygons})
	return aoi
}

// plotArea reads the numeric string the plot API stores. An unreadable or non-finite
// value becomes zero, which the side floor turns into the smallest honest sample rather
// than a cell of invented size or a geometry no encoder can write.
func plotArea(value string) float64 {
	area, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(area) || math.IsInf(area, 0) || area < 0 {
		return 0
	}
	return area
}
