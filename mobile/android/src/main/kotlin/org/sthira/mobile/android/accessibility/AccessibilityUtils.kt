package org.sthira.mobile.android.accessibility

import android.content.Context
import android.content.Intent
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.defaultMinSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CutCornerShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.Shape
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.sthira.mobile.android.theme.HazardRed
import org.sthira.mobile.android.theme.HazardRedSurface
import org.sthira.mobile.android.theme.SafeGreen
import org.sthira.mobile.android.theme.SafeGreenSurface
import org.sthira.mobile.android.theme.WarningAmber
import org.sthira.mobile.android.theme.WarningAmberSurface

/**
 * Standard min touch target per WCAG 2.1 Success Criterion 2.5.5 (Target Size)
 * and Android Accessibility guidelines (>= 48 dp).
 */
fun Modifier.accessibleTarget(minDp: Int = 48): Modifier =
    this.defaultMinSize(minWidth = minDp.dp, minHeight = minDp.dp)

enum class HazardLevel {
    CRITICAL_RED_ZONE,
    SAFE_ZONE,
    ADVISORY_WARNING,
    UNKNOWN_CAPACITY
}

/**
 * Non-color-only hazard badge: combines distinctive geometric shape, icon text,
 * contrasting background, and TalkBack semantics so color-blind citizens are never misled.
 */
@Composable
fun HazardBadge(
    level: HazardLevel,
    label: String,
    modifier: Modifier = Modifier
) {
    val (shape: Shape, bgColor: Color, borderColor: Color, textColor: Color, symbol: String, a11yPrefix: String) = when (level) {
        HazardLevel.CRITICAL_RED_ZONE -> Hexuple(
            CutCornerShape(8.dp), // Octagonal hazard shape
            HazardRedSurface,
            HazardRed,
            HazardRed,
            "⛔ [STOP / RED ZONE]",
            "Danger Red Zone: "
        )
        HazardLevel.SAFE_ZONE -> Hexuple(
            RoundedCornerShape(12.dp), // Soft pill shape for safety
            SafeGreenSurface,
            SafeGreen,
            SafeGreen,
            "🛡️ [SAFE ZONE]",
            "Safe Zone designated: "
        )
        HazardLevel.ADVISORY_WARNING -> Hexuple(
            RoundedCornerShape(4.dp), // Rounded rectangle
            WarningAmberSurface,
            WarningAmber,
            WarningAmber,
            "⚠️ [CAUTION]",
            "Warning Advisory: "
        )
        HazardLevel.UNKNOWN_CAPACITY -> Hexuple(
            RoundedCornerShape(4.dp),
            Color(0xFFEDE7F6),
            Color(0xFF512DA8),
            Color(0xFF512DA8),
            "❓ [UNCONFIRMED]",
            "Capacity Status Unknown: "
        )
    }

    Box(
        modifier = modifier
            .background(color = bgColor, shape = shape)
            .border(width = 1.5.dp, color = borderColor, shape = shape)
            .padding(horizontal = 10.dp, vertical = 6.dp)
            .semantics { contentDescription = "$a11yPrefix $label" },
        contentAlignment = Alignment.Center
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Text(
                text = symbol,
                fontWeight = FontWeight.Bold,
                fontSize = 12.sp,
                color = textColor
            )
            Spacer(modifier = Modifier.width(6.dp))
            Text(
                text = label,
                fontWeight = FontWeight.SemiBold,
                fontSize = 13.sp,
                color = textColor
            )
        }
    }
}

/**
 * Emergency Dialler Handoff: opens device dialler with official 112 / disaster control room
 * upon explicit citizen touch. Never makes automated calls without user action.
 */
object EmergencyDialler {
    fun openDialler(context: Context, officialNumber: String = "112") {
        val sanitized = officialNumber.filter { it.isDigit() || it == '+' }
        val intent = Intent(Intent.ACTION_DIAL).apply {
            data = Uri.parse("tel:$sanitized")
            flags = Intent.FLAG_ACTIVITY_NEW_TASK
        }
        context.startActivity(intent)
    }
}

private data class Hexuple<A, B, C, D, E, F>(
    val first: A,
    val second: B,
    val third: C,
    val fourth: D,
    val fifth: E,
    val sixth: F
)
