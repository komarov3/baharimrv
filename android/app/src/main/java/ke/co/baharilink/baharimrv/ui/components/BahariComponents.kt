package ke.co.baharilink.baharimrv.ui.components

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import ke.co.baharilink.baharimrv.ui.theme.extendedColors

/**
 * BahariCard - standard content card: 16dp radius, hairline outlineVariant border,
 * 16dp padding, ripple when clickable.
 */
@Composable
fun BahariCard(
    modifier: Modifier = Modifier,
    highlighted: Boolean = false,
    onClick: (() -> Unit)? = null,
    content: @Composable ColumnScope.() -> Unit
) {
    val borderColor = if (highlighted) {
        MaterialTheme.colorScheme.primary
    } else {
        MaterialTheme.colorScheme.outlineVariant
    }

    Card(
        modifier = modifier
            .fillMaxWidth()
            .let {
                if (onClick != null) it.clickable(onClick = onClick) else it
            },
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surface
        ),
        border = BorderStroke(1.dp, borderColor),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Column(
            modifier = Modifier.padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(10.dp),
            content = content
        )
    }
}

enum class BahariButtonVariant {
    PRIMARY,
    GREEN,
    AMBER,
    VIOLET,
    GHOST,
    OUTLINED
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
        modifier = modifier.height(if (small) 40.dp else 48.dp),
        enabled = enabled,
        shape = MaterialTheme.shapes.small,
        colors = ButtonDefaults.buttonColors(
            containerColor = containerColor,
            contentColor = contentColor,
            disabledContainerColor = colors.surfaceVariant,
            disabledContentColor = colors.onSurfaceVariant
        ),
        border = if (variant == BahariButtonVariant.OUTLINED) {
            BorderStroke(1.dp, borderColor)
        } else null,
        contentPadding = PaddingValues(horizontal = 16.dp, vertical = 10.dp),
        elevation = ButtonDefaults.buttonElevation(
            defaultElevation = 0.dp,
            pressedElevation = 0.dp
        )
    ) {
        Row(
            horizontalArrangement = Arrangement.spacedBy(8.dp),
            verticalAlignment = Alignment.CenterVertically,
            content = content
        )
    }
}

enum class ChipColor {
    NEUTRAL,
    BLUE,
    GREEN,
    AMBER
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
            colors.secondaryContainer, colors.secondary, colors.onSecondaryContainer
        )
        selected && color == ChipColor.GREEN -> Triple(
            colors.primaryContainer, colors.primary, colors.onPrimaryContainer
        )
        selected && color == ChipColor.AMBER -> Triple(
            colors.tertiaryContainer, colors.tertiary, colors.onTertiaryContainer
        )
        else -> Triple(
            colors.surface, colors.outline, colors.onSurfaceVariant
        )
    }

    Surface(
        modifier = modifier
            .let { if (onClick != null) it.clickable(onClick = onClick) else it }
            .then(if (onClick != null) Modifier.heightIn(min = 32.dp) else Modifier),
        color = background,
        contentColor = textColor,
        shape = CircleShape,
        border = BorderStroke(1.dp, border)
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 12.dp, vertical = 6.dp),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            leadingIcon?.invoke()
            Text(
                text = text,
                style = MaterialTheme.typography.labelSmall,
                fontWeight = FontWeight.SemiBold
            )
        }
    }
}

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
        IconBackgroundColor.BLUE -> colorScheme.secondaryContainer to colorScheme.onSecondaryContainer
        IconBackgroundColor.GREEN -> colorScheme.primaryContainer to colorScheme.onPrimaryContainer
        IconBackgroundColor.AMBER -> colorScheme.tertiaryContainer to colorScheme.onTertiaryContainer
        IconBackgroundColor.VIOLET -> extColors.violetContainer to extColors.onVioletContainer
        IconBackgroundColor.CYAN -> extColors.cyanContainer to extColors.onCyanContainer
        IconBackgroundColor.DEFAULT -> colorScheme.surfaceVariant to colorScheme.onSurfaceVariant
    }

    Box(
        modifier = modifier
            .size(40.dp)
            .background(backgroundColor, MaterialTheme.shapes.small),
        contentAlignment = Alignment.Center
    ) {
        CompositionLocalProvider(LocalContentColor provides contentColor) {
            content()
        }
    }
}

@Composable
fun BahariGradientCard(
    modifier: Modifier = Modifier,
    gradient: Brush,
    contentColor: Color = Color.White,
    content: @Composable ColumnScope.() -> Unit
) {
    Surface(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        color = Color.Transparent,
        contentColor = contentColor
    ) {
        Box(
            modifier = Modifier
                .background(gradient)
                .padding(16.dp)
        ) {
            Column(
                verticalArrangement = Arrangement.spacedBy(10.dp),
                content = content
            )
        }
    }
}

@Composable
fun BahariStatusPill(
    text: String,
    modifier: Modifier = Modifier,
    isOffline: Boolean = false,
    leadingIcon: @Composable (() -> Unit)? = null
) {
    val colors = MaterialTheme.colorScheme
    val extColors = MaterialTheme.extendedColors
    val (bgColor, textColor) = if (isOffline) {
        extColors.navyLight to extColors.offlineYellow
    } else {
        colors.primaryContainer to colors.onPrimaryContainer
    }

    Surface(
        modifier = modifier,
        color = bgColor,
        contentColor = textColor,
        shape = CircleShape
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 4.dp),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            leadingIcon?.invoke()
            Text(
                text = text,
                style = MaterialTheme.typography.labelSmall,
                fontWeight = FontWeight.SemiBold
            )
        }
    }
}

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

    val color = if (selected) activeColor else colors.onSurfaceVariant

    Column(
        modifier = modifier
            .fillMaxHeight()
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(4.dp, Alignment.CenterVertically)
    ) {
        CompositionLocalProvider(LocalContentColor provides color) {
            icon()
        }
        Text(
            text = label,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.SemiBold,
            color = color
        )
    }
}

@Composable
fun BahariDashedBox(
    modifier: Modifier = Modifier,
    content: @Composable BoxScope.() -> Unit
) {
    Box(
        modifier = modifier
            .background(
                MaterialTheme.extendedColors.paper,
                MaterialTheme.shapes.medium
            )
            .dashedBorder(
                color = MaterialTheme.extendedColors.wire,
                radius = 16.dp,
                width = 1.5.dp,
            ),
        contentAlignment = Alignment.Center,
        content = content
    )
}