package ke.co.baharilink.baharimrv.ui.components

import android.widget.Toast
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.compositionLocalOf
import androidx.compose.ui.platform.LocalContext

/** Synchronous session check, provided by AppRoot. Defaults to true outside the app tree. */
val LocalSessionActive = compositionLocalOf<() -> Boolean> { { true } }

/**
 * A toast that stays quiet once the local session is gone. Toasts outlive the
 * screen that queued them, so the error from the call that discovered a dead
 * refresh grant would otherwise paint over the sign-in screen.
 */
@Composable
fun SessionAwareToast(message: String?) {
    val context = LocalContext.current
    val sessionActive = LocalSessionActive.current
    LaunchedEffect(message) {
        if (message != null && sessionActive()) {
            Toast.makeText(context, message, Toast.LENGTH_LONG).show()
        }
    }
}