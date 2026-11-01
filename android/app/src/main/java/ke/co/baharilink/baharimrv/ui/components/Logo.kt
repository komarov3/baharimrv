package ke.co.baharilink.baharimrv.ui.components

import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Path
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.graphics.drawscope.rotate
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

/**
 * BahariMRV Logo - Professional design representing:
 * - Blue ocean waves (conservation)
 * - Checkmark/verification symbol (MRV)
 * - Circular completeness (sustainable cycles)
 * - Green accent (nature/growth)
 */
@Composable
fun BahariMrvLogo(
    modifier: Modifier = Modifier,
    size: Dp = 48.dp,
    primaryColor: Color = Color(0xFF079B76), // Field Emerald
    secondaryColor: Color = Color(0xFF12263B), // Ledger Navy
    accentColor: Color = Color(0xFF4A9EDB), // Ocean Blue
) {
    Box(
        modifier = modifier.size(size),
        contentAlignment = Alignment.Center
    ) {
        Canvas(modifier = Modifier.matchParentSize()) {
            val canvasSize = this.size.minDimension
            val center = Offset(this.size.width / 2f, this.size.height / 2f)
            val radius = canvasSize * 0.4f
            
            // Ocean waves (three flowing curves)
            val waveStroke = Stroke(width = canvasSize * 0.06f, cap = StrokeCap.Round)
            
            // Top wave (accent blue)
            val wave1 = Path().apply {
                moveTo(center.x - radius * 0.8f, center.y - radius * 0.3f)
                cubicTo(
                    center.x - radius * 0.2f, center.y - radius * 0.5f,
                    center.x + radius * 0.2f, center.y - radius * 0.1f,
                    center.x + radius * 0.8f, center.y - radius * 0.3f
                )
            }
            drawPath(wave1, color = accentColor, style = waveStroke)
            
            // Middle wave (lighter emerald)
            val wave2 = Path().apply {
                moveTo(center.x - radius * 0.8f, center.y)
                cubicTo(
                    center.x - radius * 0.2f, center.y - radius * 0.2f,
                    center.x + radius * 0.2f, center.y + radius * 0.2f,
                    center.x + radius * 0.8f, center.y
                )
            }
            drawPath(wave2, color = primaryColor.copy(alpha = 0.7f), style = waveStroke)
            
            // Bottom wave (primary emerald)
            val wave3 = Path().apply {
                moveTo(center.x - radius * 0.8f, center.y + radius * 0.3f)
                cubicTo(
                    center.x - radius * 0.2f, center.y + radius * 0.1f,
                    center.x + radius * 0.2f, center.y + radius * 0.5f,
                    center.x + radius * 0.8f, center.y + radius * 0.3f
                )
            }
            drawPath(wave3, color = primaryColor, style = waveStroke)
            
            // Verification checkmark (navy) - positioned top right
            val checkStroke = Stroke(width = canvasSize * 0.08f, cap = StrokeCap.Round)
            val checkPath = Path().apply {
                moveTo(center.x + radius * 0.4f, center.y - radius * 0.5f)
                lineTo(center.x + radius * 0.65f, center.y - radius * 0.25f)
                lineTo(center.x + radius * 0.95f, center.y - radius * 0.7f)
            }
            drawPath(checkPath, color = secondaryColor, style = checkStroke)
        }
    }
}

/**
 * Simplified icon version for navigation bar and small displays
 */
@Composable
fun BahariMrvIcon(
    modifier: Modifier = Modifier,
    size: Dp = 24.dp,
    color: Color = MaterialTheme.colorScheme.primary
) {
    Box(
        modifier = modifier.size(size),
        contentAlignment = Alignment.Center
    ) {
        Canvas(modifier = Modifier.matchParentSize()) {
            val canvasSize = this.size.minDimension
            val center = Offset(this.size.width / 2f, this.size.height / 2f)
            
            // Single wave with checkmark overlay
            val stroke = Stroke(width = canvasSize * 0.15f, cap = StrokeCap.Round)
            
            // Wave
            val wave = Path().apply {
                moveTo(center.x - canvasSize * 0.4f, center.y)
                cubicTo(
                    center.x - canvasSize * 0.1f, center.y - canvasSize * 0.15f,
                    center.x + canvasSize * 0.1f, center.y + canvasSize * 0.15f,
                    center.x + canvasSize * 0.4f, center.y
                )
            }
            drawPath(wave, color = color, style = stroke)
            
            // Check
            val checkPath = Path().apply {
                moveTo(center.x - canvasSize * 0.15f, center.y)
                lineTo(center.x - canvasSize * 0.05f, center.y + canvasSize * 0.1f)
                lineTo(center.x + canvasSize * 0.15f, center.y - canvasSize * 0.15f)
            }
            drawPath(checkPath, color = color, style = Stroke(width = canvasSize * 0.12f, cap = StrokeCap.Round))
        }
    }
}
