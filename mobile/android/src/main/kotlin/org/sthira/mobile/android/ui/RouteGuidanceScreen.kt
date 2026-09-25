package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
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
import org.sthira.mobile.android.accessibility.HazardBadge
import org.sthira.mobile.android.accessibility.HazardLevel
import org.sthira.mobile.android.accessibility.accessibleTarget
import org.sthira.mobile.android.theme.HazardRed
import org.sthira.mobile.android.theme.SthiraBluePrimary
import org.sthira.mobile.contracts.InstructionStep
import org.sthira.mobile.offlinepkg.FacilityCard
import org.sthira.mobile.offlinepkg.RouteCard

@Composable
fun RouteGuidanceScreen(
    facility: FacilityCard,
    route: RouteCard?,
    instructions: List<InstructionStep>,
    isRouteRevoked: Boolean,
    onStartNavigation: () -> Unit,
    onManageStay: () -> Unit,
    modifier: Modifier = Modifier
) {
    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(16.dp)
    ) {
        Text(
            text = "Evacuation Route Guidance",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold
        )

        Spacer(modifier = Modifier.height(4.dp))

        Text(
            text = "Destination: ${facility.name} (Safe Zone ${facility.safeZoneId})",
            fontSize = 14.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        Spacer(modifier = Modifier.height(12.dp))

        // Route Revocation Alert
        if (isRouteRevoked) {
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = Color(0xFFFFEBEE)),
                shape = RoundedCornerShape(8.dp)
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    HazardBadge(
                        level = HazardLevel.CRITICAL_RED_ZONE,
                        label = "ROUTE REVOKED BY AUTHORITY"
                    )
                    Spacer(modifier = Modifier.height(6.dp))
                    Text(
                        text = "This evacuation corridor has been declared unsafe (e.g. flooded road / landslide). Do not proceed along this route. Please return to destination selection.",
                        color = HazardRed,
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold
                    )
                }
            }
            Spacer(modifier = Modifier.height(12.dp))
        }

        // Map View Container / Schematic Box
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .height(160.dp)
                .background(Color(0xFFE0E0E0), RoundedCornerShape(10.dp))
                .border(1.dp, Color(0xFFBDBDBD), RoundedCornerShape(10.dp))
                .semantics {
                    contentDescription = "Vector Map: Approved corridor from current position to ${facility.name}"
                },
            contentAlignment = Alignment.Center
        ) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                Text(
                    text = "🗺️ Vector Map Display",
                    fontWeight = FontWeight.Bold,
                    fontSize = 14.sp,
                    color = Color(0xFF424242)
                )
                Text(
                    text = "Approved Government Corridor #${route?.id ?: "N/A"}",
                    fontSize = 12.sp,
                    color = Color(0xFF616161)
                )
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        // Non-Map Turn-by-Turn Textual Directions (Accessible / Low Bandwidth)
        Text(
            text = "Turn-by-Turn Directions (Accessible Text)",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold
        )

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
        ) {
            itemsIndexed(instructions) { index, step ->
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 6.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Box(
                        modifier = Modifier
                            .size(28.dp)
                            .background(SthiraBluePrimary, CircleShape),
                        contentAlignment = Alignment.Center
                    ) {
                        Text(
                            text = "${index + 1}",
                            color = Color.White,
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold
                        )
                    }

                    Spacer(modifier = Modifier.width(12.dp))

                    Column {
                        Text(
                            text = step.instruction,
                            fontSize = 14.sp,
                            fontWeight = FontWeight.Medium
                        )
                        Text(
                            text = "${step.distanceMeters}m • ${step.maneuver}",
                            fontSize = 12.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }
        }

        Spacer(modifier = Modifier.height(12.dp))

        // Action Buttons
        Row(modifier = Modifier.fillMaxWidth()) {
            Button(
                onClick = onStartNavigation,
                enabled = !isRouteRevoked,
                modifier = Modifier
                    .weight(1f)
                    .accessibleTarget(50),
                colors = ButtonDefaults.buttonColors(containerColor = SthiraBluePrimary),
                shape = RoundedCornerShape(8.dp)
            ) {
                Text(text = "Start GPS Guidance", fontWeight = FontWeight.Bold)
            }

            Spacer(modifier = Modifier.width(8.dp))

            Button(
                onClick = onManageStay,
                modifier = Modifier
                    .weight(1f)
                    .accessibleTarget(50),
                colors = ButtonDefaults.buttonColors(containerColor = Color(0xFF2E7D32)),
                shape = RoundedCornerShape(8.dp)
            ) {
                Text(text = "Reserve / Check-In", fontWeight = FontWeight.Bold)
            }
        }
    }
}
