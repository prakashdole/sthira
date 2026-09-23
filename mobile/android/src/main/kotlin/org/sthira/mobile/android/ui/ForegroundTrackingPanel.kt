package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import org.sthira.mobile.android.accessibility.accessibleTarget
import org.sthira.mobile.android.theme.HazardRed
import org.sthira.mobile.android.theme.SafeGreen
import org.sthira.mobile.android.theme.SthiraBluePrimary
import org.sthira.mobile.android.theme.WarningAmber
import org.sthira.mobile.contracts.JourneyState
import org.sthira.mobile.contracts.PositionReading

@Composable
fun ForegroundTrackingPanel(
    journeyState: JourneyState,
    distanceMeters: Double?,
    lastReading: PositionReading?,
    onStartTracking: () -> Unit,
    onStopTracking: () -> Unit,
    onExplicitArrival: () -> Unit,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier
            .fillMaxWidth()
            .padding(vertical = 8.dp),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
        shape = RoundedCornerShape(12.dp)
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                // Tracking Status Dot
                val (statusColor, statusLabel) = when (journeyState) {
                    JourneyState.NOT_STARTED -> Pair(Color(0xFF9E9E9E), "Navigation Inactive")
                    JourneyState.TRACKING -> Pair(SthiraBluePrimary, "Foreground Tracking Active")
                    JourneyState.NEAR_DESTINATION -> Pair(SafeGreen, "Near Destination (Within 75m)")
                    JourneyState.ARRIVAL_REPORTED -> Pair(SafeGreen, "Arrival Confirmed by Citizen")
                    JourneyState.PAUSED -> Pair(WarningAmber, "Navigation Paused")
                    JourneyState.LOCATION_UNAVAILABLE -> Pair(HazardRed, "GPS Signal Weak / Denied")
                    JourneyState.ROUTE_REVOKED -> Pair(HazardRed, "Corridor Revoked by Authority")
                }

                Box(
                    modifier = Modifier
                        .size(12.dp)
                        .background(statusColor, CircleShape)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = statusLabel,
                    fontWeight = FontWeight.Bold,
                    fontSize = 13.sp,
                    color = statusColor,
                    modifier = Modifier.weight(1f)
                )
            }

            Spacer(modifier = Modifier.height(10.dp))

            // Distance & Accuracy Display
            if (distanceMeters != null) {
                val formattedDist = if (distanceMeters >= 1000) {
                    "%.2f km".format(distanceMeters / 1000.0)
                } else {
                    "%d meters".format(distanceMeters.toInt())
                }

                Text(
                    text = "Distance to Destination: $formattedDist",
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold
                )
            }

            if (lastReading != null) {
                Text(
                    text = "GPS Accuracy: ±%.1fm • Timestamp: %d".format(
                        lastReading.accuracyMeters,
                        lastReading.timestampEpochMs
                    ),
                    fontSize = 12.sp,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            // Proximity Advisory (O10: Advisory only, never auto-arrives)
            if (journeyState == JourneyState.NEAR_DESTINATION) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(Color(0xFFE8F5E9), RoundedCornerShape(8.dp))
                        .border(1.dp, SafeGreen, RoundedCornerShape(8.dp))
                        .padding(12.dp)
                ) {
                    Column {
                        Text(
                            text = "📍 You are near your designated shelter!",
                            fontWeight = FontWeight.Bold,
                            fontSize = 14.sp,
                            color = SafeGreen
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text = "Please tap 'Confirm Arrival' below once you have checked in with the reception manager.",
                            fontSize = 12.sp,
                            color = Color(0xFF2E7D32)
                        )
                    }
                }
                Spacer(modifier = Modifier.height(12.dp))
            }

            // Controls
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                if (journeyState == JourneyState.NOT_STARTED || journeyState == JourneyState.PAUSED) {
                    Button(
                        onClick = onStartTracking,
                        modifier = Modifier
                            .weight(1f)
                            .accessibleTarget(48),
                        colors = ButtonDefaults.buttonColors(containerColor = SthiraBluePrimary)
                    ) {
                        Text("Start Tracking")
                    }
                } else {
                    OutlinedButton(
                        onClick = onStopTracking,
                        modifier = Modifier
                            .weight(1f)
                            .accessibleTarget(48)
                    ) {
                        Text("Stop Tracking")
                    }
                }

                if (journeyState == JourneyState.NEAR_DESTINATION || journeyState == JourneyState.TRACKING) {
                    Button(
                        onClick = onExplicitArrival,
                        modifier = Modifier
                            .weight(1f)
                            .accessibleTarget(48),
                        colors = ButtonDefaults.buttonColors(containerColor = SafeGreen)
                    ) {
                        Text("Touch Arrived")
                    }
                }
            }
        }
    }
}
