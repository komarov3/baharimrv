package ke.co.baharilink.baharimrv.ui.satellite

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent
import android.content.IntentFilter
import android.graphics.BitmapFactory
import android.graphics.Color
import android.graphics.drawable.BitmapDrawable
import android.graphics.drawable.Drawable
import androidx.core.content.ContextCompat
import ke.co.baharilink.baharimrv.R
import org.osmdroid.tileprovider.IRegisterReceiver
import org.osmdroid.tileprovider.MapTileProviderArray
import org.osmdroid.tileprovider.modules.MapTileModuleProviderBase
import org.osmdroid.tileprovider.tilesource.ITileSource
import org.osmdroid.tileprovider.tilesource.XYTileSource
import org.osmdroid.util.GeoPoint
import org.osmdroid.util.MapTileIndex
import org.osmdroid.views.MapView
import org.osmdroid.views.overlay.Marker
import org.osmdroid.views.overlay.TilesOverlay

/**
 * osmdroid tile module that pulls NDVI tiles through the app's authenticated
 * OkHttp client (via [NdviTileProvider.fetchTile]). osmdroid's stock downloader
 * cannot attach the session bearer, so only the Drawable wrapper is renderer
 * specific here; the byte fetch, bbox/zoom guards and the 410 supersede signal
 * all stay in the shared provider.
 *
 * Returning null (no NDVI for this tile) lets the OSM basemap show through. The
 * overlay's loading background is transparent so pending or empty tiles never
 * paint a grey grid over the map.
 */
internal class NdviTileModuleProvider(
    private val fetcher: NdviTileProvider,
    private var tileSource: ITileSource,
) : MapTileModuleProviderBase(DOWNLOAD_THREADS, MAX_QUEUE) {

    private val loader = object : TileLoader() {
        override fun loadTile(tileIndex: Long): Drawable? {
            val bytes = fetcher.fetchTile(
                MapTileIndex.getX(tileIndex),
                MapTileIndex.getY(tileIndex),
                MapTileIndex.getZoom(tileIndex),
            ) ?: return null
            val bitmap = BitmapFactory.decodeByteArray(bytes, 0, bytes.size) ?: return null
            return BitmapDrawable(null, bitmap)
        }
    }

    override fun getTileLoader(): TileLoader = loader
    override fun getName(): String = "BahariNDVI"
    override fun getThreadGroupName(): String = "BahariNDVI"
    override fun getUsesDataConnection(): Boolean = true
    override fun getMinimumZoomLevel(): Int = tileSource.minimumZoomLevel
    override fun getMaximumZoomLevel(): Int = tileSource.maximumZoomLevel
    override fun setTileSource(source: ITileSource) {
        tileSource = source
    }

    private companion object {
        const val DOWNLOAD_THREADS = 2
        const val MAX_QUEUE = 40
    }
}

/**
 * Builds the transparent NDVI [TilesOverlay] for one scene. Online-only: the
 * authenticated OkHttp client already disk-caches by URL (scene included), so
 * osmdroid keeps just an in-memory cache and a scene change can never serve a
 * stale tile left over from a previous scene.
 */
internal fun ndviTilesOverlay(context: Context, fetcher: NdviTileProvider): TilesOverlay {
    val tileSource = XYTileSource(
        "BahariNDVI",
        NdviTileProvider.MIN_ZOOM,
        NdviTileProvider.MAX_ZOOM,
        NdviTileProvider.TILE_SIZE,
        ".png",
        arrayOf(fetcher.baseUrl),
    )
    val receiver = object : IRegisterReceiver {
        override fun registerReceiver(receiver: BroadcastReceiver?, filter: IntentFilter?): Intent? = null
        override fun unregisterReceiver(receiver: BroadcastReceiver?) {}
        override fun destroy() {}
    }
    val provider = MapTileProviderArray(
        tileSource,
        receiver,
        arrayOf<MapTileModuleProviderBase>(NdviTileModuleProvider(fetcher, tileSource)),
    )
    return TilesOverlay(provider, context).apply {
        setLoadingBackgroundColor(Color.TRANSPARENT)
        setLoadingLineColor(Color.TRANSPARENT)
    }
}

/** A plot pin with its label and NDVI band; tapping opens the info window. */
internal fun ndviPlotMarker(mapView: MapView, marker: PlotMarker, snippet: String): Marker =
    Marker(mapView).apply {
        setIcon(ContextCompat.getDrawable(mapView.context, R.drawable.ic_satellite_plot_pin))
        setPosition(GeoPoint(marker.latitude, marker.longitude))
        setTitle(marker.label)
        setSnippet(snippet)
        setAnchor(Marker.ANCHOR_CENTER, Marker.ANCHOR_BOTTOM)
        setOnMarkerClickListener { clicked, _ ->
            clicked.showInfoWindow()
            true
        }
    }