package ke.co.baharilink.baharimrv.ui.satellite

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import kotlin.math.PI
import kotlin.math.atan
import kotlin.math.exp
import kotlin.math.pow

/**
 * Pure-function coverage for the geometry helper the tile provider uses to
 * drop tiles outside the project bbox. No Maps SDK, no network, no Android
 * framework — this runs on the JVM and runs in CI.
 */
class NdviTileProviderTest {
    private fun tileBBox(x: Int, y: Int, zoom: Int): DoubleArray {
        val n = 2.0.pow(zoom.toDouble())
        val minLon = x / n * 360.0 - 180.0
        val maxLon = (x + 1) / n * 360.0 - 180.0
        val maxLat = Math.toDegrees(2.0 * atan(exp(PI * (1.0 - 2.0 * y / n))) - PI / 2.0)
        val minLat = Math.toDegrees(2.0 * atan(exp(PI * (1.0 - 2.0 * (y + 1) / n))) - PI / 2.0)
        return doubleArrayOf(minLat, minLon, maxLat, maxLon)
    }

    private fun tileFor(lon: Double, lat: Double, zoom: Int): Pair<Int, Int> {
        val n = 2.0.pow(zoom.toDouble())
        val x = ((lon + 180.0) / 360.0 * n).toInt()
        val latRad = Math.toRadians(lat)
        val y = ((1.0 - kotlin.math.ln(kotlin.math.tan(latRad) + 1.0 / kotlin.math.cos(latRad)) / PI) / 2.0 * n).toInt()
        return x to y
    }

    @Test
    fun tileCentredOnJahaziProjectIntersectsItsBBox() {
        val (x, y) = tileFor(lon = 39.67, lat = -4.05, zoom = 14)
        val bbox = tileBBox(x, y, 14)
        // A 0.1 degree padding around the tile keeps the point inside.
        assertTrue(
            NdviTileProvider.tileIntersects(
                x = x, y = y, zoom = 14,
                minLat = bbox[0] - 0.01, minLon = bbox[1] - 0.01,
                maxLat = bbox[2] + 0.01, maxLon = bbox[3] + 0.01,
            ),
        )
    }

    @Test
    fun tileOnOtherSideOfWorldDoesNotIntersect() {
        val (x, y) = tileFor(lon = 39.67, lat = -4.05, zoom = 14)
        assertFalse(
            NdviTileProvider.tileIntersects(
                x = x, y = y, zoom = 14,
                minLat = 40.70, minLon = -74.02, maxLat = 40.80, maxLon = -73.90,
            ),
        )
    }

    @Test
    fun tileAndBBoxThatShareAnEdgeStillIntersect() {
        // Tile (0, 0, 1) spans lon [-180, 0] and lat [0, ~66.5]. A bbox whose
        // south edge equals the tile's equator and whose west edge equals the
        // tile's west edge still intersects on the closed interval.
        assertTrue(
            NdviTileProvider.tileIntersects(
                x = 0, y = 0, zoom = 1,
                minLat = 0.0, minLon = -180.0, maxLat = 30.0, maxLon = -90.0,
            ),
        )
    }

    @Test
    fun documentedZoomBoundsAreEightThroughSixteen() {
        assertEquals(8, NdviTileProvider.MIN_ZOOM)
        assertEquals(16, NdviTileProvider.MAX_ZOOM)
    }

    @Test
    fun tileIntersectsHandlesGlobalBBoxWithoutCrashing() {
        // A full-world bbox is what a project with no bbox guard effectively
        // uses: every tile intersects. The helper must stay bounded.
        assertTrue(
            NdviTileProvider.tileIntersects(
                x = 1234, y = 2345, zoom = 13,
                minLat = -85.0, minLon = -180.0, maxLat = 85.0, maxLon = 180.0,
            ),
        )
    }
}
