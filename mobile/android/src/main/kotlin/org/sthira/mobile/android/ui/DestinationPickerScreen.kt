package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
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
import org.sthira.mobile.contracts.PlaceCandidate
import org.sthira.mobile.offlinepkg.FacilityCard

@OptIn(ExperimentalLayoutApi::class)
@Composable
fun DestinationPickerScreen(
    facilities: List<FacilityCard>,
    ambiguousCandidates: List<PlaceCandidate>,
    isVoiceRecording: Boolean,
    isProcessing: Boolean,
    onVoiceClick: () -> Unit,
    onTextSearch: (String) -> Unit,
    onCandidateSelected: (PlaceCandidate) -> Unit,
    onFacilitySelected: (FacilityCard) -> Unit,
    modifier: Modifier = Modifier
) {
    var searchQuery by remember { mutableStateOf("") }

    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(16.dp)
    ) {
        Text(
            text = "Select Evacuation Destination",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold
        )

        Spacer(modifier = Modifier.height(12.dp))

        // Search bar with Voice Mic and Text Input
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically
        ) {
            OutlinedTextField(
                value = searchQuery,
                onValueChange = {
                    searchQuery = it
                    onTextSearch(it)
                },
                modifier = Modifier
                    .weight(1f)
                    .accessibleTarget(48),
                placeholder = { Text("Search safe zone or village...") },
                singleLine = true
            )

            Spacer(modifier = Modifier.width(8.dp))

            // Prominent Voice Mic button (IndicConformer ASR)
            IconButton(
                onClick = onVoiceClick,
                modifier = Modifier
                    .size(52.dp)
                    .background(
                        color = if (isVoiceRecording) HazardRed else SthiraBluePrimary,
                        shape = CircleShape
                    )
                    .semantics {
                        contentDescription = if (isVoiceRecording)
                            "Listening to voice input. Tap to stop."
                        else
                            "Tap to speak your destination or village in Malayalam, Hindi, or English."
                    }
            ) {
                if (isProcessing) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(24.dp),
                        color = Color.White,
                        strokeWidth = 2.dp
                    )
                } else {
                    Text(
                        text = if (isVoiceRecording) "🛑" else "🎙️",
                        fontSize = 20.sp,
                        color = Color.White
                    )
                }
            }
        }

        Spacer(modifier = Modifier.height(16.dp))

        // Ambiguous Place Candidate Chips (409 Conflict Resolution)
        if (ambiguousCandidates.isNotEmpty()) {
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = Color(0xFFFFF3E0)),
                shape = RoundedCornerShape(8.dp)
            ) {
                Column(modifier = Modifier.padding(14.dp)) {
                    Text(
                        text = "⚠️ Multiple Locations Found",
                        fontWeight = FontWeight.Bold,
                        fontSize = 14.sp,
                        color = WarningAmber
                    )
                    Text(
                        text = "Please touch your exact location to continue. (Auto-selection disabled for safety):",
                        fontSize = 12.sp,
                        color = Color(0xFF4A4A4A)
                    )

                    Spacer(modifier = Modifier.height(10.dp))

                    FlowRow(
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        verticalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        ambiguousCandidates.forEach { candidate ->
                            Box(
                                modifier = Modifier
                                    .accessibleTarget(48)
                                    .background(Color.White, RoundedCornerShape(20.dp))
                                    .border(1.5.dp, SthiraBluePrimary, RoundedCornerShape(20.dp))
                                    .clickable { onCandidateSelected(candidate) }
                                    .padding(horizontal = 14.dp, vertical = 8.dp)
                                    .semantics {
                                        contentDescription = "Select candidate: ${candidate.displayName}"
                                    },
                                contentAlignment = Alignment.Center
                            ) {
                                Text(
                                    text = candidate.displayName,
                                    fontSize = 13.sp,
                                    fontWeight = FontWeight.SemiBold,
                                    color = SthiraBluePrimary
                                )
                            }
                        }
                    }
                }
            }
            Spacer(modifier = Modifier.height(16.dp))
        }

        // Available Facilities List
        Text(
            text = "Designated Government Facilities (${facilities.size})",
            style = MaterialTheme.typography.titleMedium,
            fontWeight = FontWeight.Bold
        )

        Spacer(modifier = Modifier.height(8.dp))

        LazyColumn(
            verticalArrangement = Arrangement.spacedBy(10.dp),
            modifier = Modifier.fillMaxSize()
        ) {
            items(facilities) { facility ->
                FacilityRowCard(
                    facility = facility,
                    onSelect = { onFacilitySelected(facility) }
                )
            }
        }
    }
}

@Composable
private fun FacilityRowCard(
    facility: FacilityCard,
    onSelect: () -> Unit
) {
    val isFull = facility.capacityRemaining != null && facility.capacityRemaining <= 0

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .accessibleTarget(56),
        colors = CardDefaults.cardColors(
            containerColor = if (isFull) Color(0xFFFAFAFA) else MaterialTheme.colorScheme.surface
        ),
        shape = RoundedCornerShape(10.dp)
    ) {
        Column(modifier = Modifier.padding(14.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    text = facility.name,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.Bold,
                    modifier = Modifier.weight(1f)
                )
                CapacityIndicator(
                    remainingCapacity = facility.capacityRemaining,
                    totalCapacity = facility.capacityTotal
                )
            }

            Spacer(modifier = Modifier.height(6.dp))

            Text(
                text = "Safe Zone: ${facility.safeZoneId} • Location: (%.4f, %.4f)".format(
                    facility.latitude,
                    facility.longitude
                ),
                fontSize = 12.sp,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(10.dp))

            Button(
                onClick = onSelect,
                enabled = !isFull,
                modifier = Modifier
                    .fillMaxWidth()
                    .accessibleTarget(48),
                colors = ButtonDefaults.buttonColors(
                    containerColor = SthiraBluePrimary,
                    disabledContainerColor = Color(0xFFBDBDBD)
                ),
                shape = RoundedCornerShape(6.dp)
            ) {
                Text(
                    text = if (isFull) "Shelter Full — Select Another Facility" else "Select & Route Here →",
                    fontWeight = FontWeight.Bold
                )
            }
        }
    }
}
