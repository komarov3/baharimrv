package ke.co.baharilink.baharimrv.ui.components

import androidx.compose.animation.core.animateFloat
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.ExpandMore
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LocalContentColor
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.rotate
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.PathEffect
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import java.io.IOException
import java.math.BigDecimal
import java.time.OffsetDateTime
import java.time.ZoneId
import java.time.format.DateTimeFormatter
import ke.co.baharilink.baharimrv.data.remote.AmountDto
import ke.co.baharilink.baharimrv.ui.theme.extendedColors
import retrofit2.HttpException

sealed interface LoadState<out T> {
    data object Loading : LoadState<Nothing>
    data class Success<T>(val data: T) : LoadState<T>
    data class Error(val message: String) : LoadState<Nothing>
}

@Composable
fun StatusChip(text: String, modifier: Modifier = Modifier, tone: ChipTone = ChipTone.NEUTRAL) {
    val colors = MaterialTheme.colorScheme
    val (container, border, content) = when (tone) {
        ChipTone.POSITIVE -> Triple(colors.primaryContainer, colors.primary, colors.onPrimaryContainer)
        ChipTone.WARNING -> Triple(colors.tertiaryContainer, colors.tertiary, colors.onTertiaryContainer)
        ChipTone.ERROR -> Triple(colors.errorContainer, colors.error, colors.onErrorContainer)
        ChipTone.NEUTRAL -> Triple(colors.surface, colors.outline, colors.onSurfaceVariant)
    }
    Surface(
        modifier = modifier,
        color = container,
        contentColor = content,
        shape = CircleShape,
        border = BorderStroke(1.dp, border)
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(horizontal = 10.dp, vertical = 5.dp),
        )
    }
}

enum class ChipTone { POSITIVE, WARNING, ERROR, NEUTRAL }

@Composable
fun stateTone(status: String): ChipTone = when (status.uppercase()) {
    "VERIFIED", "ANCHORED", "CONFIRMED", "RECONCILED", "ACTIVE", "ISSUED", "RETIRED", "STORED", "APPROVE" -> ChipTone.POSITIVE
    "FAILED", "REJECTED", "REVOKED", "DISPUTED", "VERIFICATION_FAILED", "SYNC_CONFLICT", "REJECT", "SUPERSEDED", "SUSPENDED" -> ChipTone.ERROR
    "PENDING", "UNVERIFIED", "ACHIEVED_UNVERIFIED", "SANDBOX", "PROOF_PENDING", "ON_DEVICE", "UPLOADING", "ANCHORING",
    "AWAITING_VERIFICATION", "SUBMITTED", "SYNC_FAILED", "REQUESTED", "IN_REVIEW", "CORRECTIONS_REQUESTED",
    "REQUEST_CORRECTIONS", "PROVISIONING", "FAILED_PROVISIONING", "PENDING_RECOVERY" -> ChipTone.WARNING
    else -> ChipTone.NEUTRAL
}

/**
 * Names the ledger a reference came from. A sandbox value is labelled sandbox; the
 * client never renders an integration mode it did not receive from the server.
 */
fun chainNetworkLabel(network: String?): String = when (network?.lowercase()) {
    "iota_testnet", "testnet" -> "IOTA testnet"
    "iota_mainnet", "mainnet" -> "IOTA mainnet"
    "iota" -> "IOTA"
    null, "" -> "Ledger mode unavailable"
    else -> "Sandbox ledger"
}

/** Renders an RFC3339 instant in the device's zone, or the raw value if it will not parse. */
fun formatInstant(value: String?): String {
    if (value.isNullOrBlank()) return "timestamp unavailable"
    val parsed = runCatching {
        OffsetDateTime.parse(value).toInstant().atZone(ZoneId.systemDefault())
            .format(DateTimeFormatter.ofPattern("d MMM yyyy • HH:mm"))
    }.getOrNull()
    return parsed ?: value
}

/** Decimal strings stay decimal: no float ever touches an amount or a quantity. */
fun formatAmount(amount: AmountDto?): String {
    if (amount == null) return "Unavailable"
    val value = runCatching { BigDecimal(amount.amount).stripTrailingZeros().toPlainString() }.getOrDefault(amount.amount)
    return "$value ${amount.currency}"
}

@Composable
fun SectionHeader(title: String, action: (@Composable () -> Unit)? = null) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(top = 8.dp, bottom = 8.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(title, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
        action?.invoke()
    }
}

@Composable
fun LedgerCard(
    modifier: Modifier = Modifier,
    contentPadding: Dp = 16.dp,
    content: @Composable ColumnScope.() -> Unit,
) {
    Card(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.medium,
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp),
    ) {
        Column(modifier = Modifier.padding(contentPadding), verticalArrangement = Arrangement.spacedBy(10.dp), content = content)
    }
}

/** Wireframe micro label (`ttl-xs`): the small bold caption that names a section. */
@Composable
fun MicroLabel(text: String, modifier: Modifier = Modifier) {
    Text(
        text,
        style = MaterialTheme.typography.labelSmall,
        color = MaterialTheme.colorScheme.onSurfaceVariant,
        modifier = modifier.padding(bottom = 6.dp),
    )
}

@Composable
fun ContextHeader(
    title: String,
    subtitle: String,
    leading: (@Composable () -> Unit)? = null,
    trailing: (@Composable () -> Unit)? = null,
) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        leading?.invoke()
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(title, style = MaterialTheme.typography.titleLarge)
            Text(subtitle, style = MaterialTheme.typography.bodySmall, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
        trailing?.invoke()
    }
}

@Composable
fun ExpandableSection(
    title: String,
    initiallyExpanded: Boolean = false,
    content: @Composable ColumnScope.() -> Unit,
) {
    var expanded by remember { mutableStateOf(initiallyExpanded) }
    val chevronRotation by androidx.compose.animation.core.animateFloatAsState(
        targetValue = if (expanded) 180f else 0f,
        animationSpec = androidx.compose.animation.core.tween(200),
        label = "expandChevron",
    )
    Column(Modifier.fillMaxWidth()) {
        TextButton(onClick = { expanded = !expanded }, modifier = Modifier.fillMaxWidth().height(48.dp)) {
            Text(title, modifier = Modifier.weight(1f), textAlign = TextAlign.Start)
            Icon(
                Icons.Default.ExpandMore,
                contentDescription = if (expanded) "Collapse $title" else "Expand $title",
                modifier = Modifier.rotate(chevronRotation),
            )
        }
        androidx.compose.animation.AnimatedVisibility(
            visible = expanded,
            enter = androidx.compose.animation.expandVertically(animationSpec = androidx.compose.animation.core.tween(200)) + androidx.compose.animation.fadeIn(),
            exit = androidx.compose.animation.shrinkVertically(animationSpec = androidx.compose.animation.core.tween(150)) + androidx.compose.animation.fadeOut(),
        ) {
            Column(verticalArrangement = Arrangement.spacedBy(8.dp), content = content)
        }
    }
}

fun Modifier.dashedBorder(color: Color, radius: Dp, width: Dp = 1.dp): Modifier = drawBehind {
    drawRoundRect(
        color = color,
        cornerRadius = CornerRadius(radius.toPx()),
        style = Stroke(
            width = width.toPx(),
            pathEffect = PathEffect.dashPathEffect(floatArrayOf(6.dp.toPx(), 4.dp.toPx())),
        ),
    )
}

@Composable
fun LoadingState(label: String = "Loading current data…") {
    Column(
        modifier = Modifier.fillMaxWidth().padding(32.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        CircularProgressIndicator(modifier = Modifier.size(40.dp))
        Text(label, style = MaterialTheme.typography.bodyMedium)
    }
}

@Composable
fun EmptyState(title: String, detail: String, actionLabel: String? = null, onAction: (() -> Unit)? = null) {
    LedgerCard {
        Column(
            modifier = Modifier.fillMaxWidth().padding(vertical = 8.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            Text(title, style = MaterialTheme.typography.titleMedium, textAlign = TextAlign.Center)
            Text(
                detail,
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
            )
            if (actionLabel != null && onAction != null) {
                OutlinedButton(onClick = onAction, modifier = Modifier.height(48.dp)) { Text(actionLabel) }
            }
        }
    }
}

@Composable
fun ErrorState(message: String, onRetry: (() -> Unit)? = null) {
    LedgerCard {
        Text("Couldn’t load current data", style = MaterialTheme.typography.titleMedium, color = MaterialTheme.colorScheme.error)
        Text(message, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
        if (onRetry != null) Button(onClick = onRetry, modifier = Modifier.height(48.dp)) { Text("Retry") }
    }
}

@Composable
fun OfflineBanner() {
    Surface(color = MaterialTheme.colorScheme.tertiaryContainer, contentColor = MaterialTheme.colorScheme.onTertiaryContainer) {
        Text(
            "Offline — field records stay on this device; server data may be out of date.",
            style = MaterialTheme.typography.bodySmall,
            modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 10.dp),
        )
    }
}

/**
 * Shimmering placeholder block for loading content. Pulse stays subtle and loops
 * at 900ms so a slow network reads as "working", not "stuck".
 */
@Composable
fun ShimmerBox(modifier: Modifier = Modifier, shape: androidx.compose.ui.graphics.Shape = MaterialTheme.shapes.small) {
    val transition = androidx.compose.animation.core.rememberInfiniteTransition(label = "shimmer")
    val alpha by transition.animateFloat(
        initialValue = 0.35f,
        targetValue = 0.9f,
        animationSpec = androidx.compose.animation.core.infiniteRepeatable(
            animation = androidx.compose.animation.core.tween(900, easing = androidx.compose.animation.core.LinearEasing),
            repeatMode = androidx.compose.animation.core.RepeatMode.Reverse,
        ),
        label = "shimmerAlpha",
    )
    Box(
        modifier = modifier
            .background(MaterialTheme.extendedColors.skeleton.copy(alpha = alpha), shape)
    )
}

/** Card-shaped skeleton matching LedgerCard geometry while real content loads. */
@Composable
fun SkeletonCard(modifier: Modifier = Modifier, lines: Int = 3) {
    LedgerCard(modifier = modifier) {
        ShimmerBox(Modifier.fillMaxWidth(0.4f).height(14.dp))
        repeat(lines) {
            ShimmerBox(Modifier.fillMaxWidth(if (it == lines - 1) 0.7f else 1f).height(12.dp))
        }
    }
}

@Composable
fun KeyValue(label: String, value: String, valueColor: Color = Color.Unspecified, monospace: Boolean = false) {
    Row(modifier = Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.SpaceBetween) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = LocalContentColor.current.copy(alpha = 0.75f))
        Spacer(modifier = Modifier.size(8.dp))
        Text(
            value,
            style = MaterialTheme.typography.bodyMedium,
            fontFamily = if (monospace) FontFamily.Monospace else FontFamily.Default,
            fontWeight = FontWeight.Medium,
            color = valueColor,
            textAlign = TextAlign.End,
        )
    }
}

fun Throwable.readableMessage(): String = when (this) {
    is IOException -> "No server connection. Check connectivity and try again; local field records are preserved."
    is HttpException -> "Server request failed (HTTP ${code()}). No success state has been assumed."
    is IllegalArgumentException, is IllegalStateException -> message ?: "The request could not be completed"
    else -> message ?: "Unexpected error. Try again."
}

/**
 * Wireframe C4: Verified badge - green circle with white checkmark
 */
@Composable
fun VerifiedBadge(modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier.size(24.dp),
        shape = CircleShape,
        color = MaterialTheme.colorScheme.primary,
    ) {
        Box(contentAlignment = Alignment.Center) {
            Icon(
                imageVector = Icons.Default.Check,
                contentDescription = "Verified",
                modifier = Modifier.size(16.dp),
                tint = MaterialTheme.colorScheme.onPrimary,
            )
        }
    }
}

/**
 * Wireframe C3: Stage number circle for verification pipeline
 * Shows numbered circle, with optional checkmark overlay for completed stages
 */
@Composable
fun StageNumberCircle(
    number: String,
    isCompleted: Boolean = false,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier.size(36.dp),
        contentAlignment = Alignment.Center,
    ) {
        Surface(
            shape = CircleShape,
            color = if (isCompleted) MaterialTheme.colorScheme.primaryContainer else MaterialTheme.colorScheme.surfaceVariant,
            contentColor = if (isCompleted) MaterialTheme.colorScheme.onPrimaryContainer else MaterialTheme.colorScheme.onSurfaceVariant,
        ) {
            Box(modifier = Modifier.size(36.dp), contentAlignment = Alignment.Center) {
                if (isCompleted) {
                    Icon(
                        imageVector = Icons.Default.Check,
                        contentDescription = "Completed",
                        modifier = Modifier.size(20.dp),
                    )
                } else {
                    Text(text = number, style = MaterialTheme.typography.labelMedium, fontWeight = FontWeight.Bold)
                }
            }
        }
    }
}

/**
 * Wireframe C1: Wallet revenue stream row
 * Shows label and emerald-colored amount, right-aligned
 */
@Composable
fun WalletRevenueStream(
    label: String,
    amount: String,
    modifier: Modifier = Modifier,
    amountColor: Color = Color.Unspecified,
    amountWeight: FontWeight = FontWeight.Bold,
) {
    Row(
        modifier = modifier.fillMaxWidth(),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
            modifier = Modifier.weight(1f),
        )
        Text(
            text = amount,
            style = MaterialTheme.typography.titleMedium,
            fontWeight = amountWeight,
            color = amountColor,
            textAlign = TextAlign.End,
        )
    }
}

/**
 * Wireframe C4: Format price with emoji for marketplace listings
 */
fun formatPrice(amount: AmountDto?): String {
    if (amount == null) return "Price unavailable"
    val value = runCatching {
        BigDecimal(amount.amount).stripTrailingZeros().toPlainString()
    }.getOrDefault(amount.amount)
    return if (amount.currency.uppercase() == "USD") "$$value" else "$value ${amount.currency}"
}

/**
 * Wireframe C4: Navy background card for retirement certificates
 */
@Composable
fun NavyLedgerCard(modifier: Modifier = Modifier, content: @Composable ColumnScope.() -> Unit) {
    Card(
        modifier = modifier.fillMaxWidth(),
        shape = MaterialTheme.shapes.large,
        colors = CardDefaults.cardColors(
            containerColor = Color(0xFF12263B), // navy
            contentColor = Color(0xFFDFF2FB)  // light blue
        ),
        border = BorderStroke(1.dp, Color(0xFF2A4356)),
    ) {
        Column(modifier = Modifier.padding(16.dp), verticalArrangement = Arrangement.spacedBy(8.dp), content = content)
    }
}
