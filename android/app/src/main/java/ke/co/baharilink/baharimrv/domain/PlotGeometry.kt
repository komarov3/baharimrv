package ke.co.baharilink.baharimrv.domain

import ke.co.baharilink.baharimrv.data.remote.GeoPolygonDto
import kotlin.math.abs
import kotlin.math.cos

/**
 * Plot geometry mirrored line for line in backend/internal/geo/geo.go and
 * s2_worker/aoi.py, so a boundary recorded on a phone yields the same hectares and
 * centroid on every layer. A ring is a closed list of (lon, lat) pairs in EPSG:4326.
 */
typealias PlotRing = List<Pair<Double, Double>>

const val METRES_PER_DEGREE_LAT = 111320.0

/** Ensures the ring is closed by repeating the first vertex when needed. */
fun closeRing(vertices: PlotRing): PlotRing {
    if (vertices.isEmpty()) return vertices
    return if (vertices.first() == vertices.last()) vertices else vertices + vertices.first()
}

/** Mirrors geo.ParsePolygon's ring rules: closed, >= 3 vertices, finite, in range. */
fun isValidBoundary(ring: PlotRing): Boolean {
    if (ring.size < 4 || ring.first() != ring.last()) return false
    return ring.all { (lon, lat) ->
        lon.isFinite() && lat.isFinite() && lon in -180.0..180.0 && lat in -90.0..90.0
    }
}

fun boundaryPolygon(ring: PlotRing): GeoPolygonDto =
    GeoPolygonDto(coordinates = listOf(ring.map { listOf(it.first, it.second) }))

/** Vertices of a stored boundary as a closed ring, or empty when it is absent or invalid. */
fun ringVertices(boundary: GeoPolygonDto?): PlotRing {
    val outer = boundary?.coordinates?.firstOrNull() ?: return emptyList()
    val ring = outer.mapNotNull { vertex ->
        if (vertex.size >= 2) vertex[0] to vertex[1] else null
    }
    return if (isValidBoundary(closeRing(ring))) closeRing(ring) else emptyList()
}

/** Polygon area in hectares via local equirectangular projection + shoelace. */
fun ringAreaHectares(ring: PlotRing): Double {
    val (lat0, lon0) = ringMean(ring)
    val (xs, ys) = project(ring, lat0, lon0)
    var sum = 0.0
    for (i in 0 until xs.size - 1) {
        sum += xs[i] * ys[i + 1] - xs[i + 1] * ys[i]
    }
    return abs(sum) / 2 / 10_000.0
}

/** Area-weighted centroid as (lat, lon); vertex mean for a degenerate ring. */
fun ringCentroid(ring: PlotRing): Pair<Double, Double> {
    val (lat0, lon0) = ringMean(ring)
    val (xs, ys) = project(ring, lat0, lon0)
    var a = 0.0
    var cx = 0.0
    var cy = 0.0
    for (i in 0 until xs.size - 1) {
        val cross = xs[i] * ys[i + 1] - xs[i + 1] * ys[i]
        a += cross
        cx += (xs[i] + xs[i + 1]) * cross
        cy += (ys[i] + ys[i + 1]) * cross
    }
    if (a == 0.0) return lat0 to lon0
    a /= 2
    cx /= 6 * a
    cy /= 6 * a
    val cosLat = cos(Math.toRadians(lat0)).let { if (it == 0.0) 1.0 else it }
    return (lat0 + cy / METRES_PER_DEGREE_LAT) to (lon0 + cx / (METRES_PER_DEGREE_LAT * cosLat))
}

/** Even-odd ray cast, a line-for-line mirror of geo.PointInRing. */
fun pointInRing(ring: PlotRing, lat: Double, lon: Double): Boolean {
    if (ring.size < 3) return false
    val cosLat = cos(Math.toRadians(lat)).let { if (it == 0.0) 1.0 else it }
    val xs = ring.map { (it.first - lon) * cosLat * METRES_PER_DEGREE_LAT }
    val ys = ring.map { (it.second - lat) * METRES_PER_DEGREE_LAT }
    var inside = false
    var i = 0
    var j = xs.size - 1
    while (i < xs.size) {
        val yi = ys[i]
        val yj = ys[j]
        if ((yi > 0) != (yj > 0)) {
            val xCross = xs[i] - yi * (xs[j] - xs[i]) / (yj - yi)
            if (xCross > 0) inside = !inside
        }
        j = i
        i++
    }
    return inside
}

private fun ringMean(ring: PlotRing): Pair<Double, Double> {
    val n = if (ring.size > 1) ring.size - 1 else ring.size
    var lon = 0.0
    var lat = 0.0
    for (i in 0 until n) {
        lon += ring[i].first
        lat += ring[i].second
    }
    return (lat / n) to (lon / n)
}

private fun project(ring: PlotRing, lat0: Double, lon0: Double): Pair<List<Double>, List<Double>> {
    val cosLat = cos(Math.toRadians(lat0)).let { if (it == 0.0) 1.0 else it }
    val xs = ring.map { (it.first - lon0) * cosLat * METRES_PER_DEGREE_LAT }
    val ys = ring.map { (it.second - lat0) * METRES_PER_DEGREE_LAT }
    return xs to ys
}