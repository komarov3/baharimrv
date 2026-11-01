package ke.co.baharilink.baharimrv.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import ke.co.baharilink.baharimrv.ui.theme.extendedColors

/**
 * BahariCard - Standard card matching wireframe specifications
 * - 12dp radius
 * - 1dp border (--line color)
 * - 9dp x 10dp padding
 * - 7dp internal gap
 */
@Composable
fun BahariCard(
    modifier: Modifier = Modifier,
    highlighted: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val borderColor = if (highlighted) {
        MaterialTheme.colorScheme.secondary
    } else {
        MaterialTheme.colorScheme.outline
    }
    
    Card(
        modifier = modifier
            .fillMaxWidth()
            .let {
                if (onClick != null) it.clickable(onClick = onClick) else it
            },
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        border = BorderStroke(
            width = 1.dp,
            color = borderColor
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Column(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 9.dp),
            verticalArrangement = Arrangement.spacedBy(7.dp),
            content = content
        )
    }
}

/**
 * BahariButton - Button variants matching wireframe
 * - 10dp radius (8dp for small)
 * - 11sp text
 * - 9dp x 10dp padding (6dp x 8dp for small)
 * - 7dp icon gap
 */
enum class BahariButtonVariant {
    PRIMARY,    // Blue (.btn.p)
    GREEN,      // Green (.btn.g)
    AMBER,      // Amber (.btn.a)
    VIOLET,     // Violet (.btn.v)
    GHOST,      // Ghost (.btn.ghost)
    OUTLINED    // Outlined (.btn)
}

@Composable
fun BahariButton(
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    variant: BahariButtonVariant = BahariButtonVariant.PRIMARY,
    small: Boolean = false,
    enabled: Boolean = true,
    content: @Composable RowScope.() -> Unit
) {
    val colors = MaterialTheme.colorScheme
    val extColors = MaterialTheme.extendedColors
    
    val (containerColor, contentColor, borderColor) = when (variant) {
        BahariButtonVariant.PRIMARY -> Triple(
            colors.secondary, colors.onSecondary, colors.secondary
        )
        BahariButtonVariant.GREEN -> Triple(
            colors.primary, colors.onPrimary, colors.primary
        )
        BahariButtonVariant.AMBER -> Triple(
            colors.tertiary, colors.onTertiary, colors.tertiary
        )
        BahariButtonVariant.VIOLET -> Triple(
            extColors.violet, extColors.onViolet, extColors.violet
        )
        BahariButtonVariant.GHOST -> Triple(
            Color.Transparent, colors.onSurfaceVariant, Color.Transparent
        )
        BahariButtonVariant.OUTLINED -> Triple(
            colors.surface, colors.onSurface, colors.outline
        )
    }
    
    Button(
        onClick = onClick,
        modifier = modifier.height(if (small) 36.dp else 48.dp),
        enabled = enabled,
        shape = RoundedCornerShape(if (small) 8.dp else 10.dp),
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = colors.surfaceVariant,
            disabledContentColor = colors.onSurfaceVariant
        ),
        border = if (variant == BahariButtonVariant.OUTLINED) {
            BorderStroke(1.dp, borderColor)
        } else null,
        contentPadding = PaddingValues(
            horizontal = if (small) 8.dp else 10.dp,
            vertical = if (small) 6.dp else 9.dp
        ),
        elevation = ButtonDefaults.buttonElevation(
            defaultElevation = 0.dp,
            pressedElevation = 0.dp
        )
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(7.dp),
            verticalAlignment = Alignment.CenterVertically,
            content = content
        )
    }
}

/**
 * BahariChip - Chip/pill component matching wireframe
 * - 999dp radius (fully rounded)
 * - 4dp x 9dp padding
 * - 9.5sp text
 * - 1dp border
 * - 5dp icon gap
 */
enum class ChipColor {
    NEUTRAL,  // Default white
    BLUE,     // Active blue
    GREEN,    // Green variant
    AMBER     // Amber variant
}

@Composable
fun BahariChip(
    text: String,
    modifier: Modifier = Modifier,
    selected: Boolean = false,
    color: ChipColor = ChipColor.NEUTRAL,
    leadingIcon: @Composable (() -> Unit)? = null,
    onClick: (() -> Unit)? = null
) {
    val colors = MaterialTheme.colorScheme
    
    val (background, border, textColor) = when {
        selected && color == ChipColor.BLUE -> Triple(
            Color(0xFFEAF1FF), Color(0xFFBFD4FF), colors.secondary
        )
        selected && color == ChipColor.GREEN -> Triple(
            Color(0xFFE7F8F1), Color(0xFFB7E8D4), colors.primary
        )
        selected && color == ChipColor.AMBER -> Triple(
            Color(0xFFFDF3E3), Color(0xFFF2DDB3), colors.tertiary
        )
        else -> Triple(
            colors.surface, colors.outline, colors.onSurfaceVariant
        )
    }
    
    Surface(
        modifier = modifier.let {
            if (onClick != null) it.clickable(onClick = onClick) else it
        },
        color = background,
        contentColor = textColor,
        shape = CircleShape,
        border = BorderStroke(1.dp, border)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 9.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(5.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            leadingIcon?.invoke()
            Text(
                text = text,
                style = MaterialTheme.typography.bodySmall.copy(
                    fontSize = 9.5.sp,
                    fontWeight = FontWeight.SemiBold
                )
            )
        }
    }
}

/**
 * BahariIconBackground - Colored icon container
 * - 34dp x 34dp
 * - 10dp radius
 * - Color-specific background
 */
enum class IconBackgroundColor {
    DEFAULT, BLUE, GREEN, AMBER, VIOLET, CYAN
}

@Composable
fun BahariIconBackground(
    modifier: Modifier = Modifier,
    color: IconBackgroundColor = IconBackgroundColor.DEFAULT,
    content: @Composable BoxScope.() -> Unit
) {
    val extColors = MaterialTheme.extendedColors
    val colorScheme = MaterialTheme.colorScheme
    
    val (backgroundColor, contentColor) = when (color) {
        IconBackgroundColor.BLUE -> Color(0xFFDBEAFE) to colorScheme.secondary
        IconBackgroundColor.GREEN -> Color(0xFFD1FAE5) to colorScheme.primary
        IconBackgroundColor.AMBER -> Color(0xFFFEF3C7) to colorScheme.tertiary
        IconBackgroundColor.VIOLET -> Color(0xFFEDE9FE) to extColors.violet
        IconBackgroundColor.CYAN -> Color(0xFFCFFAFE) to extColors.cyan
        IconBackgroundColor.DEFAULT -> Color(0xFFE8EEF6) to extColors.slate
    }
    
    Box(
        modifier = modifier
            .size(34.dp)
            .background(backgroundColor, RoundedCornerShape(10.dp)),
        contentAlignment = Alignment.Center
    ) {
        CompositionLocalProvider(LocalContentColor provides contentColor) {
            content()
        }
    }
}

/**
 * BahariGradientCard - Card with gradient background
 * Matches Blue ID card and wallet card styles
 */
@Composable
fun BahariGradientCard(
    modifier: Modifier = Modifier,
    gradient: Brush,
    contentColor: Color = Color.White,
    content: @Composable ColumnScope.() -> Unit
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = Color.Transparent,
        contentColor = contentColor
    ) {
        Box(
            modifier = Modifier
                .background(gradient)
                .padding(horizontal = 10.dp, vertical = 9.dp)
        ) {
            Column(
                verticalArrangement = Arrangement.spacedBy(10.dp),
                content = content
            )
        }
    }
}

/**
 * BahariStatusPill - Small status indicator pills
 * - 999dp radius
 * - 3dp x 8dp padding
 * - 9sp text
 */
@Composable
fun BahariStatusPill(
    text: String,
    modifier: Modifier = Modifier,
    isOffline: Boolean = false,
    leadingIcon: @Composable (() -> Unit)? = null
) {
    val extColors = MaterialTheme.extendedColors
    val (bgColor, textColor) = if (isOffline) {
        extColors.navyLight to extColors.offlineYellow
    } else {
        Color(0xFFE7F8F1) to MaterialTheme.colorScheme.primary
    }
    
    Surface(
        modifier = modifier,
        color = bgColor,
        contentColor = textColor,
        shape = CircleShape
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp),
            horizontalArrangement = Arrangement.spacedBy(5.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            leadingIcon?.invoke()
            Text(
                text = text,
                style = TextStyle(
                    fontSize = 9.sp,
                    fontWeight = FontWeight.Bold
                )
            )
        }
    }
}

/**
 * BahariBottomNav - Bottom navigation matching wireframe
 * - 54dp height
 * - 8.5sp text
 * - Module-specific active colors
 */
enum class ModuleColor {
    BLUE,    // Fisheries
    AMBER,   // Tourism
    GREEN,   // Conservation
    VIOLET   // Cross-module
}

@Composable
fun BahariBottomNavItem(
    selected: Boolean,
    onClick: () -> Unit,
    icon: @Composable () -> Unit,
    label: String,
    modifier: Modifier = Modifier,
    moduleColor: ModuleColor = ModuleColor.BLUE
) {
    val colors = MaterialTheme.colorScheme
    val extColors = MaterialTheme.extendedColors
    
    val activeColor = when (moduleColor) {
        ModuleColor.BLUE -> colors.secondary
        ModuleColor.AMBER -> colors.tertiary
        ModuleColor.GREEN -> colors.primary
        ModuleColor.VIOLET -> extColors.violet
    }
    
    val color = if (selected) activeColor else Color(0xFF9AA8BA)
    
    Column(
        modifier = modifier
            .fillMaxHeight()
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(3.dp, Alignment.CenterVertically)
    ) {
        CompositionLocalProvider(LocalContentColor provides color) {
            icon()
        }
        Text(
            text = label,
            style = TextStyle(
                fontSize = 8.5.sp,
                fontWeight = FontWeight.SemiBold,
                color = color
            )
        )
    }
}

/**
 * BahariDashedBox - Dashed border placeholder box
 * For photo/media placeholders
 */
@Composable
fun BahariDashedBox(
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit
) {
    Box(
        modifier = modifier
            .background(
                Color(0xFFF7FAFC),
                RoundedCornerShape(12.dp)
            )
            // Note: Dashed border requires custom drawing
            // This is a simplified version
            .border(
                width = 1.5.dp,
                color = MaterialTheme.extendedColors.wire,
                shape = RoundedCornerShape(12.dp)
            ),
        contentAlignment = Alignment.Center,
        content = content
    )
}
