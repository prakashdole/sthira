package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.sthira.mobile.android.accessibility.EmergencyDialler
import org.sthira.mobile.android.accessibility.accessibleTarget
import org.sthira.mobile.android.theme.HazardRed
import org.sthira.mobile.android.theme.SafeGreen
import org.sthira.mobile.android.theme.SthiraBluePrimary
import org.sthira.mobile.android.theme.SyntheticDemoBanner
import org.sthira.mobile.android.theme.WarningAmber
import org.sthira.mobile.contracts.FreshnessState

/**
 * SyntheticExerciseBanner: clearly labels exercise data to prevent citizens
 * from mistaking drills for real disaster evacuations.
 */
@Composable
fun SyntheticExerciseBanner(
    isExercise: Boolean,
    modifier: Modifier = Modifier
) {
    if (!isExercise) return

    Box(
        modifier = modifier
            .fillMaxWidth()
            .background(SyntheticDemoBanner)
            .padding(horizontal = 16.dp, vertical = 8.dp)
            .semantics {
                contentDescription = "Notice: Synthetic disaster drill exercise in progress. This is not a real evacuation order."
            },
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = "⚠️ SYNTHETIC EXERCISE — NOT A REAL DISASTER ORDER",
            color = Color.White,
            fontWeight = FontWeight.Bold,
            fontSize = 12.sp,
            letterSpacing = 0.5.sp
        )
    }
}

/**
 * FreshnessBadge: displays honest freshness states (FRESH, STALE, UNVERIFIABLE, EXPIRED)
 */
@Composable
fun FreshnessBadge(
    freshness: FreshnessState,
    sourceId: String,
    modifier: Modifier = Modifier
) {
    val (bgColor, textColor, label) = when (freshness) {
        FreshnessState.FRESH -> Triple(Color(0xFFE8F5E9), SafeGreen, "Verified Live")
        FreshnessState.STALE -> Triple(Color(0xFFFFF3E0), WarningAmber, "Stale Data (Needs Recheck)")
        FreshnessState.UNVERIFIABLE -> Triple(Color(0xFFFFEBEE), HazardRed, "Unverifiable (Clock Mismatch)")
        FreshnessState.EXPIRED -> Triple(Color(0xFFFFEBEE), HazardRed, "Expired Advisory")
        FreshnessState.WITHDRAWN -> Triple(Color(0xFFEEEEEE), Color(0xFF616161), "Withdrawn by Authority")
    }

    Box(
        modifier = modifier
            .background(bgColor, shape = RoundedCornerShape(4.dp))
            .border(1.dp, textColor, shape = RoundedCornerShape(4.dp))
            .padding(horizontal = 8.dp, vertical = 4.dp)
            .semantics { contentDescription = "Data status: $label from $sourceId" }
    ) {
        Text(
            text = "$label • $sourceId",
            color = textColor,
            fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold
        )
    }
}

/**
 * EmergencyCallCard: provides a prominent button to open the device dialler
 * with 112 / local disaster helpline.
 */
@Composable
fun EmergencyCallCard(
    officialContact: String = "112",
    modifier: Modifier = Modifier
) {
    val context = LocalContext.current

    Card(
        modifier = modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = Color(0xFFFFEBEE)),
        shape = RoundedCornerShape(8.dp)
    ) {
        Row(
            modifier = Modifier
                .padding(16.dp)
                .fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = "Immediate Danger?",
                    fontWeight = FontWeight.Bold,
                    fontSize = 16.sp,
                    color = HazardRed
                )
                Text(
                    text = "Contact official emergency services",
                    fontSize = 13.sp,
                    color = Color(0xFF4A4A4A)
                )
            }
            Spacer(modifier = Modifier.width(12.dp))
            Button(
                onClick = { EmergencyDialler.openDialler(context, officialContact) },
                colors = ButtonDefaults.buttonColors(containerColor = HazardRed),
                modifier = Modifier.accessibleTarget(48)
            ) {
                Text(
                    text = "Call $officialContact",
                    color = Color.White,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}

/**
 * CapacityIndicator: displays shelter availability honestly.
 * Unknown capacity is explicitly presented as unknown, never 0 or 100%.
 */
@Composable
fun CapacityIndicator(
    remainingCapacity: Int?,
    totalCapacity: Int?,
    modifier: Modifier = Modifier
) {
    when {
        remainingCapacity == null || totalCapacity == null -> {
            Box(
                modifier = modifier
                    .background(Color(0xFFEDE7F6), RoundedCornerShape(4.dp))
                    .padding(horizontal = 8.dp, vertical = 4.dp)
                    .semantics { contentDescription = "Capacity unconfirmed by local authority" }
            ) {
                Text(
                    text = "❓ Capacity: Unconfirmed",
                    color = Color(0xFF4A148C),
                    fontSize = 12.sp,
                    fontWeight = FontWeight.SemiBold
                )
            }
        }
        remainingCapacity <= 0 -> {
            Box(
                modifier = modifier
                    .background(Color(0xFFFFEBEE), RoundedCornerShape(4.dp))
                    .padding(horizontal = 8.dp, vertical = 4.dp)
                    .semantics { contentDescription = "Shelter at maximum capacity. 0 beds available." }
            ) {
                Text(
                    text = "⛔ Full (0/$totalCapacity Available)",
                    color = HazardRed,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }
        else -> {
            Box(
                modifier = modifier
                    .background(Color(0xFFE8F5E9), RoundedCornerShape(4.dp))
                    .padding(horizontal = 8.dp, vertical = 4.dp)
                    .semantics { contentDescription = "$remainingCapacity of $totalCapacity beds available" }
            ) {
                Text(
                    text = "✓ $remainingCapacity/$totalCapacity Available",
                    color = SafeGreen,
                    fontSize = 12.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}
