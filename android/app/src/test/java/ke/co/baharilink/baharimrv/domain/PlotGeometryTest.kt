package ke.co.baharilink.baharimrv.domain

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import kotlin.math.cos

class PlotGeometryTest {

    private fun squareRing(lat: Double, lon: Double, side: Double): PlotRing {
        val dlat = side / 2 / METRES_PER_DEGREE_LAT
        val dlon = side / 2 / (METRES_PER_DEGREE_LAT * cos(Math.toRadians(lat)))
        val west = lon - dlon
        val east = lon + dlon
        val south = lat - dlat
        val north = lat + dlat
        return listOf(west to south, east to south, east to north, west to north, west to south)
    }

    @Test
    fun areaOf100mSquareIsOneHectareAtAnyLatitude() {
        for (lat in listOf(0.0, -4.05, 51.5)) {
            assertEquals(1.0, ringAreaHectares(squareRing(lat, 39.67, 100.0)), 1e-6)
        }
    }

    @Test
    fun centroidLandsAtTheSquareCentre() {
        val (lat, lon) = ringCentroid(squareRing(-4.05, 39.67, 100.0))
        assertEquals(-4.05, lat, 1e-9)
        assertEquals(39.67, lon, 1e-9)
    }

    @Test
    fun closeRingRepeatsTheFirstVertex() {
        val open = listOf(0.0 to 0.0, 1.0 to 0.0, 1.0 to 1.0)
        val closed = closeRing(open)
        assertEquals(4, closed.size)
        assertEquals(closed.first(), closed.last())
        assertEquals(closed, closeRing(closed))
    }

    @Test
    fun validityMirrorsTheBackendRules() {
        assertTrue(isValidBoundary(squareRing(-4.05, 39.67, 100.0)))
        assertFalse(isValidBoundary(listOf(0.0 to 0.0, 1.0 to 0.0, 1.0 to 1.0)))
        assertFalse(isValidBoundary(listOf(0.0 to 0.0, 1.0 to 0.0, 1.0 to 1.0, 0.0 to 1.0)))
        assertFalse(isValidBoundary(listOf(0.0 to 0.0, 200.0 to 0.0, 200.0 to 1.0, 0.0 to 0.0)))
    }

    @Test
    fun boundaryPolygonIsAClosedGeoJsonRing() {
        val ring = squareRing(-4.05, 39.67, 100.0)
        val polygon = boundaryPolygon(ring)
        assertEquals("Polygon", polygon.type)
        assertEquals(1, polygon.coordinates.size)
        assertEquals(ring.size, polygon.coordinates[0].size)
        assertEquals(listOf(ring[0].first, ring[0].second), polygon.coordinates[0][0])
    }

    @Test
    fun pointInRingSquareMatchesBackend() {
        val ring = squareRing(-4.05, 39.67, 200.0)
        val inside = listOf(-4.05 to 39.67, -4.0505 to 39.6695, -4.0495 to 39.6705)
        val outside = listOf(-4.06 to 39.67, -4.05 to 39.69, 0.0 to 0.0, -4.04 to 39.67)
        for ((lat, lon) in inside) {
            assertTrue("($lat,$lon) must be inside", pointInRing(ring, lat, lon))
        }
        for ((lat, lon) in outside) {
            assertFalse("($lat,$lon) must be outside", pointInRing(ring, lat, lon))
        }
    }

    @Test
    fun pointInRingConcaveMatchesBackend() {
        // An L-shaped polygon: a 2x2 block with the top-right 1x1 removed.
        val ring: PlotRing = listOf(0.0 to 0.0, 2.0 to 0.0, 2.0 to 1.0, 1.0 to 1.0, 1.0 to 2.0, 0.0 to 2.0, 0.0 to 0.0)
        assertTrue(pointInRing(ring, 0.5, 0.5))   // lower-left arm
        assertTrue(pointInRing(ring, 1.5, 0.5))   // upper-left arm
        assertTrue(pointInRing(ring, 0.5, 1.5))   // lower-right arm
        assertFalse(pointInRing(ring, 1.5, 1.5))  // the notch that was cut out
        assertFalse(pointInRing(ring, 0.5, 2.5))  // fully outside
    }

    @Test
    fun pointInRingDegenerateIsNeverInside() {
        assertFalse(pointInRing(listOf(0.0 to 0.0, 0.0 to 0.0), 0.0, 0.0))
        assertFalse(pointInRing(emptyList(), 0.0, 0.0))
    }
}