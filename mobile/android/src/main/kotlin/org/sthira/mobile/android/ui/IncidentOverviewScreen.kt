package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
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
import org.sthira.mobile.android.theme.SafeGreen
import org.sthira.mobile.android.theme.SthiraBluePrimary
import org.sthira.mobile.contracts.FreshnessState
import org.sthira.mobile.offlinepkg.PublicIncidentCard

@Composable
fun IncidentOverviewScreen(
    incident: PublicIncidentCard?,
    isSyntheticExercise: Boolean,
    onNavigateToDestinations: () -> Unit,
    modifier: Modifier = Modifier
) {
    val scrollState = rememberScrollState()

    Column(
        modifier = modifier
            .fillMaxSize()
            .verticalScroll(scrollState)
    ) {
        SyntheticExerciseBanner(isExercise = isSyntheticExercise)

        Column(modifier = Modifier.padding(16.dp)) {
            EmergencyCallCard(officialContact = "112")

            Spacer(modifier = Modifier.height(16.dp))

            if (incident == null) {
                Card(
                    modifier = Modifier.fillMaxWidth(),
                    colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceVariant)
                ) {
                    Column(modifier = Modifier.padding(20.dp)) {
                        Text(
                            text = "No Active Emergency Alerts",
                            style = MaterialTheme.typography.titleMedium,
                            fontWeight = FontWeight.Bold
                        )
                        Spacer(modifier = Modifier.height(6.dp))
                        Text(
                            text = "There are currently no active evacuation orders or disaster alerts for your registered jurisdiction.",
                            style = MaterialTheme.typography.bodyMedium
                        )
                    }
                }
                return@Column
            }

            // Official incident header
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
                shape = RoundedCornerShape(12.dp)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "OFFICIAL DISASTER ADVISORY",
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Bold,
                            color = SthiraBluePrimary,
                            modifier = Modifier.weight(1f)
                        )
                        FreshnessBadge(
                            freshness = FreshnessState.FRESH,
                            sourceId = incident.sourceId
                        )
                    }

                    Spacer(modifier = Modifier.height(8.dp))

                    Text(
                        text = incident.title,
                        style = MaterialTheme.typography.headlineSmall,
                        fontWeight = FontWeight.Bold,
                        color = MaterialTheme.colorScheme.onSurface
                    )

                    Spacer(modifier = Modifier.height(4.dp))

                    Text(
                        text = "Jurisdiction: ${incident.jurisdiction} • Issued: ${incident.issuedAt}",
                        fontSize = 12.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )

                    Spacer(modifier = Modifier.height(12.dp))

                    Text(
                        text = incident.description,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurface
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // Red Zones Section
            Text(
                text = "Active Red Zones (${incident.redZones.size})",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold
            )
            Spacer(modifier = Modifier.height(8.dp))

            incident.redZones.forEach { redZone ->
                Card(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 4.dp),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFFFFEBEE)),
                    shape = RoundedCornerShape(8.dp)
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            HazardBadge(
                                level = HazardLevel.CRITICAL_RED_ZONE,
                                label = redZone.name
                            )
                        }
                        Spacer(modifier = Modifier.height(6.dp))
                        Text(
                            text = redZone.hazardType.uppercase(),
                            fontSize = 12.sp,
                            fontWeight = FontWeight.Bold,
                            color = HazardRed
                        )
                        Text(
                            text = redZone.instructions,
                            fontSize = 13.sp,
                            color = MaterialTheme.colorScheme.onSurface
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // Designated Safe Zones Section
            Text(
                text = "Designated Safe Zones (${incident.safeZones.size})",
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.Bold
            )
            Spacer(modifier = Modifier.height(8.dp))

            incident.safeZones.forEach { safeZone ->
                Card(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 4.dp),
                    colors = CardDefaults.cardColors(containerColor = Color(0xFFE8F5E9)),
                    shape = RoundedCornerShape(8.dp)
                ) {
                    Column(modifier = Modifier.padding(14.dp)) {
                        HazardBadge(
                            level = HazardLevel.SAFE_ZONE,
                            label = safeZone.name
                        )
                        Spacer(modifier = Modifier.height(4.dp))
                        Text(
                            text = "Member Facilities: ${safeZone.facilities.joinToString(", ")}",
                            fontSize = 12.sp,
                            color = SafeGreen
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(24.dp))

            // Primary Call to Action
            Button(
                onClick = onNavigateToDestinations,
                modifier = Modifier
                    .fillMaxWidth()
                    .accessibleTarget(52),
                colors = ButtonDefaults.buttonColors(containerColor = SthiraBluePrimary),
                shape = RoundedCornerShape(8.dp)
            ) {
                Text(
                    text = "Find Evacuation Shelters & Routes →",
                    fontSize = 16.sp,
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}
