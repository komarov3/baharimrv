package ke.co.baharilink.baharimrv.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp

// Extended colors beyond Material3 standard palette
@Immutable
data class ExtendedColorScheme(
    val cyan: Color,
    val onCyan: Color,
    val cyanContainer: Color,
    val onCyanContainer: Color,
    val violet: Color,
    val onViolet: Color,
    val violetContainer: Color,
    val onVioletContainer: Color,
    val slate: Color,
    val onSlate: Color,
    val navy: Color,
    val navyLight: Color,
    val navyCard: Color,
    val offlineYellow: Color,
    val paper: Color,
    val wire: Color,
    val skeleton: Color,
    val skeletonDark: Color,
)

val LightExtendedColors = ExtendedColorScheme(
    cyan = Color(0xFF0E7490),
    onCyan = Color.White,
    cyanContainer = Color(0xFFCFFAFE),
    onCyanContainer = Color(0xFF0C4A5E),
    violet = Color(0xFF7C3AED),
    onViolet = Color.White,
    violetContainer = Color(0xFFEDE9FE),
    onVioletContainer = Color(0xFF4C1D95),
    slate = Color(0xFF475569),
    onSlate = Color.White,
    navy = Color(0xFF0B1C2C),
    navyLight = Color(0xFF1F2B3C),
    navyCard = Color(0xFF12263B),
    offlineYellow = Color(0xFFFFD166),
    paper = Color(0xFFF7FAFC),
    wire = Color(0xFFB9C6D6),
    skeleton = Color(0xFFDFE7F0),
    skeletonDark = Color(0xFFC7D2E0),
)

val DarkExtendedColors = ExtendedColorScheme(
    cyan = Color(0xFF22D3EE),
    onCyan = Color(0xFF083344),
    cyanContainer = Color(0xFF155E75),
    onCyanContainer = Color(0xFFCFFAFE),
    violet = Color(0xFFA78BFA),
    onViolet = Color(0xFF3B0764),
    violetContainer = Color(0xFF5B21B6),
    onVioletContainer = Color(0xFFEDE9FE),
    slate = Color(0xFF94A3B8),
    onSlate = Color(0xFF1E293B),
    navy = Color(0xFF0F172A),
    navyLight = Color(0xFF1E293B),
    navyCard = Color(0xFF1E293B),
    offlineYellow = Color(0xFFFFD166),
    paper = Color(0xFF1E293B),
    wire = Color(0xFF475569),
    skeleton = Color(0xFF334155),
    skeletonDark = Color(0xFF1E293B),
)

val LocalExtendedColors = staticCompositionLocalOf { LightExtendedColors }

// Gradient definitions matching wireframe
object BahariGradients {
    val hero = Brush.linearGradient(
        0.0f to Color(0xFF082F3F),
        0.55f to Color(0xFF0E7490),
        1.0f to Color(0xFF155E75)
    )
    val blueIdCard = Brush.linearGradient(
        0.0f to Color(0xFF12263B),
        1.0f to Color(0xFF0E7490)
    )
    val walletCard = Brush.linearGradient(
        0.0f to Color(0xFF12263B),
        1.0f to Color(0xFF1D4ED8)
    )
    val splash = Brush.linearGradient(
        0.0f to Color(0xFF0E7490),
        1.0f to Color(0xFF082F3F)
    )
}

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
    background = Color(0xFFE9EEF4),
    onBackground = Color(0xFF152238),
    surface = Color.White,
    onSurface = Color(0xFF152238),
    surfaceVariant = Color(0xFFEAF0F5),
    onSurfaceVariant = Color(0xFF5B6B80),
    outline = Color(0xFFD8E0EA),
    outlineVariant = Color(0xFFE8EEF3),
    error = Color(0xFFDC2626),
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
    val extendedColors = if (isSystemInDarkTheme()) DarkExtendedColors else LightExtendedColors
    
    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors,
        typography = BahariTypography,
        shapes = Shapes(
            extraSmall = RoundedCornerShape(6.dp),
            small = RoundedCornerShape(8.dp),
            medium = RoundedCornerShape(10.dp),
            large = RoundedCornerShape(12.dp),
            extraLarge = RoundedCornerShape(14.dp),
        ),
    ) {
        androidx.compose.runtime.CompositionLocalProvider(
            LocalExtendedColors provides extendedColors,
            content = content
        )
    }
}

// Extension property to access extended colors
val MaterialTheme.extendedColors: ExtendedColorScheme
    @Composable
    get() = LocalExtendedColors.current
