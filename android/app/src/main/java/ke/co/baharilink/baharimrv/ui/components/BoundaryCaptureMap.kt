package ke.co.baharilink.baharimrv.ui.components

import android.content.Context
import android.graphics.Color
import android.location.Location
import android.location.LocationManager
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import org.osmdroid.events.MapEventsReceiver
import org.osmdroid.tileprovider.tilesource.TileSourceFactory
import org.osmdroid.util.GeoPoint
import org.osmdroid.views.MapView
import org.osmdroid.views.overlay.MapEventsOverlay
import org.osmdroid.views.overlay.Marker
import org.osmdroid.views.overlay.Polygon

data class GpsFix(val latitude: Double, val longitude: Double, val accuracyM: Double?)

fun recentGpsFix(context: Context): GpsFix? {
    val manager = context.getSystemService(Context.LOCATION_SERVICE) as? LocationManager ?: return null
    for (provider in listOf(LocationManager.GPS_PROVIDER, LocationManager.NETWORK_PROVIDER)) {
        if (!manager.isProviderEnabled(provider)) continue
        val location: Location = runCatching { manager.getLastKnownLocation(provider) }.getOrNull() ?: continue
        val accuracy = if (location.hasAccuracy()) location.accuracy.toDouble() else null
        return GpsFix(location.latitude, location.longitude, accuracy)
    }
    return null
}

@Composable
fun BoundaryCaptureMap(
    vertices: List<Pair<Double, Double>>,
    tapEnabled: () -> Boolean,
    onVertex: (lon: Double, lat: Double) -> Unit,
    modifier: Modifier = Modifier,
) {
    val context = LocalContext.current
    val mapView = remember {
        MapView(context).apply {
            setTileSource(TileSourceFactory.MAPNIK)
            setMultiTouchControls(true)
            setBuiltInZoomControls(false)
            controller.setZoom(16.0)
            val start = recentGpsFix(context)
            controller.setCenter(if (start != null) GeoPoint(start.latitude, start.longitude) else GeoPoint(-4.05, 39.67))
            overlays.add(
                MapEventsOverlay(
                    object : MapEventsReceiver {
                        override fun singleTapConfirmedHelper(p: GeoPoint): Boolean {
                            if (tapEnabled()) {
                                onVertex(p.longitude, p.latitude)
                                return true
                            }
                            return false
                        }

                        override fun longPressHelper(p: GeoPoint): Boolean = false
                    },
                ),
            )
        }
    }
    DisposableEffect(mapView) {
        mapView.onResume()
        onDispose {
            mapView.onPause()
            mapView.onDetach()
        }
    }
    AndroidView(
        factory = { mapView },
        modifier = modifier,
        update = { mv ->
            while (mv.overlays.size > 1) mv.overlays.removeAt(mv.overlays.size - 1)
            if (vertices.size >= 2) {
                val polygon = Polygon().apply {
                    points = vertices.map { GeoPoint(it.second, it.first) }
                    fillColor = Color.argb(48, 0, 128, 0)
                    strokeColor = Color.GREEN
                    strokeWidth = 3f
                }
                mv.overlays.add(polygon)
            }
            vertices.forEach { vertex ->
                val marker = Marker(mv).apply {
                    position = GeoPoint(vertex.second, vertex.first)
                    setAnchor(Marker.ANCHOR_CENTER, Marker.ANCHOR_CENTER)
                }
                mv.overlays.add(marker)
            }
            mv.invalidate()
        },
    )
}
