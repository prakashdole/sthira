package org.sthira.mobile.android.ui

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
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
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
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
import org.sthira.mobile.contracts.ReservationResponse
import org.sthira.mobile.offlinepkg.FacilityCard

@Composable
fun StayManagementScreen(
    facility: FacilityCard,
    currentReservation: ReservationResponse?,
    isOffline: Boolean,
    isSubmitting: Boolean,
    onConfirmReservation: (peopleCount: Int) -> Unit,
    onExplicitArrival: (reservationId: String) -> Unit,
    onExtendStay: (reservationId: String) -> Unit,
    onDepartStay: (reservationId: String) -> Unit,
    onTransferStay: (reservationId: String, newFacilityId: String) -> Unit,
    onCancelStay: (reservationId: String) -> Unit,
    modifier: Modifier = Modifier
) {
    var partySize by remember { mutableStateOf(1) }
    var showArrivalDialog by remember { mutableStateOf(false) }
    var showDepartDialog by remember { mutableStateOf(false) }
    var showCancelDialog by remember { mutableStateOf(false) }

    val scrollState = rememberScrollState()

    Column(
        modifier = modifier
            .fillMaxSize()
            .padding(16.dp)
            .verticalScroll(scrollState)
    ) {
        Text(
            text = "Shelter Stay & Reservation",
            style = MaterialTheme.typography.headlineSmall,
            fontWeight = FontWeight.Bold
        )

        Spacer(modifier = Modifier.height(4.dp))

        Text(
            text = "Facility: ${facility.name} (Zone ${facility.safeZoneId})",
            fontSize = 14.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        Spacer(modifier = Modifier.height(16.dp))

        // Offline Notification Banner
        if (isOffline) {
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = Color(0xFFFFF3E0)),
                shape = RoundedCornerShape(8.dp)
            ) {
                Row(
                    modifier = Modifier.padding(12.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    Text(text = "📡", fontSize = 20.sp)
                    Spacer(modifier = Modifier.width(10.dp))
                    Text(
                        text = "Working Offline: Actions will be queued durably on device and synced when connectivity returns.",
                        fontSize = 12.sp,
                        color = Color(0xFFE65100),
                        fontWeight = FontWeight.Medium
                    )
                }
            }
            Spacer(modifier = Modifier.height(14.dp))
        }

        // Active Reservation State Card
        if (currentReservation != null) {
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
                            text = "ACTIVE RESERVATION",
                            fontSize = 11.sp,
                            fontWeight = FontWeight.Bold,
                            color = SthiraBluePrimary,
                            modifier = Modifier.weight(1f)
                        )
                        Box(
                            modifier = Modifier
                                .background(
                                    if (currentReservation.status == "ARRIVED") Color(0xFFE8F5E9) else Color(0xFFE3F2FD),
                                    RoundedCornerShape(4.dp)
                                )
                                .padding(horizontal = 8.dp, vertical = 4.dp)
                        ) {
                            Text(
                                text = currentReservation.status,
                                fontSize = 12.sp,
                                fontWeight = FontWeight.Bold,
                                color = if (currentReservation.status == "ARRIVED") SafeGreen else SthiraBluePrimary
                            )
                        }
                    }

                    Spacer(modifier = Modifier.height(10.dp))

                    Text(
                        text = "Reservation ID: ${currentReservation.reservationId}",
                        fontSize = 13.sp,
                        fontWeight = FontWeight.SemiBold
                    )
                    Text(
                        text = "Beds / Capacity: ${currentReservation.allocatedCapacity} persons",
                        fontSize = 13.sp
                    )
                    Text(
                        text = "Valid Until: ${currentReservation.expiresAt}",
                        fontSize = 13.sp,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )

                    Spacer(modifier = Modifier.height(16.dp))

                    // Explicit Touch Arrival Confirmation Button
                    if (currentReservation.status == "RESERVED") {
                        Button(
                            onClick = { showArrivalDialog = true },
                            modifier = Modifier
                                .fillMaxWidth()
                                .accessibleTarget(52),
                            colors = ButtonDefaults.buttonColors(containerColor = SafeGreen),
                            shape = RoundedCornerShape(8.dp)
                        ) {
                            Text(
                                text = "Confirm Arrival at Shelter (Touch Action)",
                                fontWeight = FontWeight.Bold,
                                fontSize = 15.sp
                            )
                        }
                        Text(
                            text = "Arrival requires explicit touch action upon physical arrival. Geofencing never auto-confirms.",
                            fontSize = 11.sp,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(top = 4.dp, bottom = 12.dp)
                        )
                    }

                    // Lifecycle Actions: Extend, Depart, Cancel
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp)
                    ) {
                        OutlinedButton(
                            onClick = { onExtendStay(currentReservation.reservationId) },
                            modifier = Modifier.weight(1f).accessibleTarget(48)
                        ) {
                            Text("Extend")
                        }
                        OutlinedButton(
                            onClick = { showDepartDialog = true },
                            modifier = Modifier.weight(1f).accessibleTarget(48)
                        ) {
                            Text("Depart")
                        }
                        OutlinedButton(
                            onClick = { showCancelDialog = true },
                            colors = ButtonDefaults.outlinedButtonColors(contentColor = HazardRed),
                            modifier = Modifier.weight(1f).accessibleTarget(48)
                        ) {
                            Text("Cancel")
                        }
                    }
                }
            }
        } else {
            // New Reservation Creation Card
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surface),
                shape = RoundedCornerShape(12.dp)
            ) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = "Reserve Shelter Capacity",
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.Bold
                    )

                    Spacer(modifier = Modifier.height(10.dp))

                    CapacityIndicator(
                        remainingCapacity = facility.capacityRemaining,
                        totalCapacity = facility.capacityTotal
                    )

                    Spacer(modifier = Modifier.height(14.dp))

                    Text(
                        text = "Number of Persons in Household / Party:",
                        fontSize = 13.sp,
                        fontWeight = FontWeight.Medium
                    )

                    Spacer(modifier = Modifier.height(8.dp))

                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        OutlinedButton(
                            onClick = { if (partySize > 1) partySize-- },
                            modifier = Modifier.accessibleTarget(48)
                        ) {
                            Text("-", fontSize = 18.sp, fontWeight = FontWeight.Bold)
                        }

                        Text(
                            text = "$partySize",
                            fontSize = 18.sp,
                            fontWeight = FontWeight.Bold
                        )

                        OutlinedButton(
                            onClick = { if (partySize < 10) partySize++ },
                            modifier = Modifier.accessibleTarget(48)
                        ) {
                            Text("+", fontSize = 18.sp, fontWeight = FontWeight.Bold)
                        }
                    }

                    Spacer(modifier = Modifier.height(18.dp))

                    Button(
                        onClick = { onConfirmReservation(partySize) },
                        enabled = !isSubmitting && (facility.capacityRemaining == null || facility.capacityRemaining > 0),
                        modifier = Modifier
                            .fillMaxWidth()
                            .accessibleTarget(52),
                        colors = ButtonDefaults.buttonColors(containerColor = SthiraBluePrimary),
                        shape = RoundedCornerShape(8.dp)
                    ) {
                        if (isSubmitting) {
                            CircularProgressIndicator(
                                modifier = Modifier.height(20.dp),
                                color = Color.White,
                                strokeWidth = 2.dp
                            )
                        } else {
                            Text(
                                text = "Confirm Shelter Reservation",
                                fontWeight = FontWeight.Bold,
                                fontSize = 15.sp
                            )
                        }
                    }
                }
            }
        }

        // Arrival Confirmation Modal Dialog
        if (showArrivalDialog && currentReservation != null) {
            AlertDialog(
                onDismissRequest = { showArrivalDialog = false },
                title = { Text("Confirm Physical Arrival") },
                text = {
                    Text("Have you physically arrived at ${facility.name} and presented yourself to the shelter manager? This updates official facility capacity.")
                },
                confirmButton = {
                    Button(
                        onClick = {
                            showArrivalDialog = false
                            onExplicitArrival(currentReservation.reservationId)
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = SafeGreen),
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Yes, I Have Arrived")
                    }
                },
                dismissButton = {
                    OutlinedButton(
                        onClick = { showArrivalDialog = false },
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Cancel")
                    }
                }
            )
        }

        // Departure Modal Dialog
        if (showDepartDialog && currentReservation != null) {
            AlertDialog(
                onDismissRequest = { showDepartDialog = false },
                title = { Text("Confirm Departure") },
                text = {
                    Text("Are you departing ${facility.name}? This will mark your stay as departed and free capacity for other evacuees.")
                },
                confirmButton = {
                    Button(
                        onClick = {
                            showDepartDialog = false
                            onDepartStay(currentReservation.reservationId)
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = SthiraBluePrimary),
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Confirm Departure")
                    }
                },
                dismissButton = {
                    OutlinedButton(
                        onClick = { showDepartDialog = false },
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Back")
                    }
                }
            )
        }

        // Cancellation Modal Dialog
        if (showCancelDialog && currentReservation != null) {
            AlertDialog(
                onDismissRequest = { showCancelDialog = false },
                title = { Text("Cancel Reservation") },
                text = {
                    Text("Cancel your reservation at ${facility.name}? Your reserved beds will be immediately released.")
                },
                confirmButton = {
                    Button(
                        onClick = {
                            showCancelDialog = false
                            onCancelStay(currentReservation.reservationId)
                        },
                        colors = ButtonDefaults.buttonColors(containerColor = HazardRed),
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Release Capacity & Cancel")
                    }
                },
                dismissButton = {
                    OutlinedButton(
                        onClick = { showCancelDialog = false },
                        modifier = Modifier.accessibleTarget(48)
                    ) {
                        Text("Keep Reservation")
                    }
                }
            )
        }
    }
}
