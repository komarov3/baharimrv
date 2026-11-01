package ke.co.baharilink.baharimrv.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

private val LightColors = lightColorScheme(
    primary = Color(0xFF059669),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFE7F8F1),
    onPrimaryContainer = Color(0xFF065F46),
    secondary = Color(0xFF2563EB),
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFEAF1FF),
    onSecondaryContainer = Color(0xFF1D4ED8),
    tertiary = Color(0xFFD97706),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFFDF3E3),
    onTertiaryContainer = Color(0xFF7C2D12),
    background = Color(0xFFFDFEFE),
    onBackground = Color(0xFF152238),
    surface = Color.White,
    onSurface = Color(0xFF152238),
    surfaceVariant = Color(0xFFEAF0F5),
    onSurfaceVariant = Color(0xFF5B6B80),
    outline = Color(0xFFD8E0EA),
    outlineVariant = Color(0xFFE8EEF3),
    error = Color(0xFFBA1A1A),
    onError = Color.White,
    errorContainer = Color(0xFFFFDAD6),
    onErrorContainer = Color(0xFF410002),
    // Material 3 ships purple-tinted defaults for the elevation containers; left in place they
    // show through on nav bars, sheets and tonal buttons and clash with the ink-and-teal wireframe.
    surfaceContainerLowest = Color.White,
    surfaceContainerLow = Color(0xFFF7FAFC),
    surfaceContainer = Color(0xFFF1F6F9),
    surfaceContainerHigh = Color(0xFFEAF1F5),
    surfaceContainerHighest = Color(0xFFE3EBF1),
    surfaceBright = Color(0xFFFDFEFE),
    surfaceDim = Color(0xFFD5DDE4),
    inverseSurface = Color(0xFF152238),
    inverseOnSurface = Color(0xFFEAF0F5),
    inversePrimary = Color(0xFF71DBB9),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFF71DBB9),
    onPrimary = Color(0xFF00382C),
    primaryContainer = Color(0xFF075844),
    onPrimaryContainer = Color(0xFFB8F4DE),
    secondary = Color(0xFFAEC6FF),
    onSecondary = Color(0xFF102F6B),
    secondaryContainer = Color(0xFF273F74),
    onSecondaryContainer = Color(0xFFDCE7FF),
    tertiary = Color(0xFFF3B75F),
    onTertiary = Color(0xFF452B00),
    tertiaryContainer = Color(0xFF62420B),
    onTertiaryContainer = Color(0xFFFFE2B0),
    background = Color(0xFF101718),
    onBackground = Color(0xFFDEE4E5),
    surface = Color(0xFF151D1F),
    onSurface = Color(0xFFDEE4E5),
    surfaceVariant = Color(0xFF253033),
    onSurfaceVariant = Color(0xFFBCC8CB),
    outline = Color(0xFF536164),
    outlineVariant = Color(0xFF354144),
    error = Color(0xFFFFB4AB),
    onError = Color(0xFF690005),
    errorContainer = Color(0xFF93000A),
    onErrorContainer = Color(0xFFFFDAD6),
    surfaceContainerLowest = Color(0xFF080E0F),
    surfaceContainerLow = Color(0xFF101718),
    surfaceContainer = Color(0xFF151D1F),
    surfaceContainerHigh = Color(0xFF1F282A),
    surfaceContainerHighest = Color(0xFF2A3335),
    surfaceBright = Color(0xFF313A3C),
    surfaceDim = Color(0xFF080E0F),
    inverseSurface = Color(0xFFDEE4E5),
    inverseOnSurface = Color(0xFF1B2426),
    inversePrimary = Color(0xFF059669),
)

@Composable
fun BahariMrvTheme(content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors,
        typography = BahariTypography,
        shapes = Shapes(
            small = RoundedCornerShape(8.dp),
            medium = RoundedCornerShape(12.dp),
            large = RoundedCornerShape(18.dp),
        ),
        content = content,
    )
}
