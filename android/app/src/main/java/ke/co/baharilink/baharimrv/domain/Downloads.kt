package ke.co.baharilink.baharimrv.domain

import android.content.Context
import java.io.File
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
import okhttp3.ResponseBody

/**
 * Streams a server response into app-scoped storage, which needs no runtime
 * permission, and returns a human-readable description of where it landed.
 */
suspend fun Context.saveDownload(folder: String, suggestedName: String, body: ResponseBody): String =
    withContext(Dispatchers.IO) {
        val root = File(this@saveDownload.getExternalFilesDir(null) ?: this@saveDownload.cacheDir, folder)
        root.mkdirs()
        val file = File(root, suggestedName.safeFileName())
        file.outputStream().use { out -> body.byteStream().use { it.copyTo(out) } }
        body.close()
        "${file.name} • ${file.length()} bytes • saved to this device's app storage"
    }

/** Server-supplied names are treated as untrusted input: no path separators survive. */
private fun String.safeFileName(): String =
    map { if (it.isLetterOrDigit() || it == '.' || it == '-' || it == '_') it else '_' }
        .joinToString("")
        .trim('_')
        .ifBlank { "download" }
