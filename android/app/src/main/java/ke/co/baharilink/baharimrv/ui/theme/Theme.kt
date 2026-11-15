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
    paper = Color(0xFFF7FAF8),
    wire = Color(0xFFB9C6BE),
    skeleton = Color(0xFFDFE7E2),
    skeletonDark = Color(0xFFC7D2CB),
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
    paper = Color(0xFF1A2420),
    wire = Color(0xFF4A5A52),
    skeleton = Color(0xFF2E3A34),
    skeletonDark = Color(0xFF222E28),
)

val LocalExtendedColors = staticCompositionLocalOf { LightExtendedColors }

// Gradient definitions: deep forest anchors with emerald and teal light.
object BahariGradients {
    val hero = Brush.linearGradient(
        0.0f to Color(0xFF022C22),
        0.55f to Color(0xFF065F46),
        1.0f to Color(0xFF0E7490)
    )
    val blueIdCard = Brush.linearGradient(
        0.0f to Color(0xFF0B1C2C),
        1.0f to Color(0xFF0E7490)
    )
    val walletCard = Brush.linearGradient(
        0.0f to Color(0xFF0B1C2C),
        1.0f to Color(0xFF1D4ED8)
    )
    val splash = Brush.linearGradient(
        0.0f to Color(0xFF065F46),
        1.0f to Color(0xFF022C22)
    )
    val emerald = Brush.linearGradient(
        0.0f to Color(0xFF047857),
        1.0f to Color(0xFF065F46)
    )
}

private val LightColors = lightColorScheme(
    primary = Color(0xFF047857),
    onPrimary = Color.White,
    primaryContainer = Color(0xFFD1FAE5),
    onPrimaryContainer = Color(0xFF064E3B),
    secondary = Color(0xFF2563EB),
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFDBEAFE),
    onSecondaryContainer = Color(0xFF1E40AF),
    tertiary = Color(0xFFB45309),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFFEF3C7),
    onTertiaryContainer = Color(0xFF78350F),
    background = Color(0xFFF2F6F3),
    onBackground = Color(0xFF14201B),
    surface = Color(0xFFFFFFFF),
    onSurface = Color(0xFF14201B),
    surfaceVariant = Color(0xFFE4EBE6),
    onSurfaceVariant = Color(0xFF4A5A52),
    outline = Color(0xFFCBD8D0),
    outlineVariant = Color(0xFFE2EAE5),
    error = Color(0xFFDC2626),
    onError = Color.White,
    errorContainer = Color(0xFFFEE2E2),
    onErrorContainer = Color(0xFF7F1D1D),
    surfaceContainerLowest = Color(0xFFFFFFFF),
    surfaceContainerLow = Color(0xFFF7FAF8),
    surfaceContainer = Color(0xFFEDF2EF),
    surfaceContainerHigh = Color(0xFFE4EBE7),
    surfaceContainerHighest = Color(0xFFDBE5DF),
    surfaceBright = Color(0xFFFFFFFF),
    surfaceDim = Color(0xFFD5DED8),
    inverseSurface = Color(0xFF14201B),
    inverseOnSurface = Color(0xFFECF3EF),
    inversePrimary = Color(0xFF6EE7B7),
)

private val DarkColors = darkColorScheme(
    primary = Color(0xFF6EE7B7),
    onPrimary = Color(0xFF022C22),
    primaryContainer = Color(0xFF065F46),
    onPrimaryContainer = Color(0xFFA7F3D0),
    secondary = Color(0xFF93C5FD),
    onSecondary = Color(0xFF172554),
    secondaryContainer = Color(0xFF1E40AF),
    onSecondaryContainer = Color(0xFFDBEAFE),
    tertiary = Color(0xFFFCD34D),
    onTertiary = Color(0xFF451A03),
    tertiaryContainer = Color(0xFF78350F),
    onTertiaryContainer = Color(0xFFFDE68A),
    background = Color(0xFF0C1210),
    onBackground = Color(0xFFE3EBE6),
    surface = Color(0xFF131B17),
    onSurface = Color(0xFFE3EBE6),
    surfaceVariant = Color(0xFF24302A),
    onSurfaceVariant = Color(0xFFA8B8AF),
    outline = Color(0xFF4A5A52),
    outlineVariant = Color(0xFF2E3A34),
    error = Color(0xFFFCA5A5),
    onError = Color(0xFF450A0A),
    errorContainer = Color(0xFF7F1D1D),
    onErrorContainer = Color(0xFFFECACA),
    surfaceContainerLowest = Color(0xFF080D0B),
    surfaceContainerLow = Color(0xFF0C1210),
    surfaceContainer = Color(0xFF131B17),
    surfaceContainerHigh = Color(0xFF1A2420),
    surfaceContainerHighest = Color(0xFF222E28),
    surfaceBright = Color(0xFF2A3630),
    surfaceDim = Color(0xFF080D0B),
    inverseSurface = Color(0xFFE3EBE6),
    inverseOnSurface = Color(0xFF131B17),
    inversePrimary = Color(0xFF047857),
)

@Composable
fun BahariMrvTheme(content: @Composable () -> Unit) {
    val extendedColors = if (isSystemInDarkTheme()) DarkExtendedColors else LightExtendedColors

    MaterialTheme(
        colorScheme = if (isSystemInDarkTheme()) DarkColors else LightColors,
        typography = BahariTypography,
        shapes = Shapes(
            extraSmall = RoundedCornerShape(8.dp),
            small = RoundedCornerShape(12.dp),
            medium = RoundedCornerShape(16.dp),
            large = RoundedCornerShape(20.dp),
            extraLarge = RoundedCornerShape(24.dp),
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