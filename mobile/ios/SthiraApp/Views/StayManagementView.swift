import SwiftUI

struct StayManagementView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel

    @State private var partySize = 1
    @State private var showArrivalAlert = false
    @State private var showDepartAlert = false
    @State private var showCancelAlert = false

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                if let fac = viewModel.selectedFacility {
                    Text("Shelter Stay & Reservation")
                        .font(.title2)
                        .fontWeight(.bold)

                    Text("Facility: \(fac.name) (Zone \(fac.safeZoneId))")
                        .font(.subheadline)
                        .foregroundColor(SthiraTheme.textSecondaryLight)

                    // Offline Banner
                    if viewModel.isOffline {
                        HStack(spacing: 10) {
                            Text("📡")
                                .font(.title2)
                            Text("Working Offline: Actions will be queued durably on device and synced when connectivity returns.")
                                .font(.caption)
                                .foregroundColor(SthiraTheme.warningAmber)
                        }
                        .padding(12)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(SthiraTheme.warningAmberSurface)
                        .cornerRadius(8)
                    }

                    // Active Reservation
                    if let res = viewModel.activeReservation {
                        VStack(alignment: .leading, spacing: 12) {
                            HStack {
                                Text("ACTIVE RESERVATION")
                                    .font(.system(size: 11, weight: .bold))
                                    .foregroundColor(SthiraTheme.primaryBlue)

                                Spacer()

                                Text(res.status)
                                    .font(.system(size: 12, weight: .bold))
                                    .foregroundColor(res.status == "ARRIVED" ? SthiraTheme.safeGreen : SthiraTheme.primaryBlue)
                                    .padding(.horizontal, 8)
                                    .padding(.vertical, 4)
                                    .background(res.status == "ARRIVED" ? SthiraTheme.safeGreenSurface : SthiraTheme.primaryBlueLight.opacity(0.15))
                                    .cornerRadius(4)
                            }

                            Text("Reservation ID: \(res.id)")
                                .font(.subheadline)
                                .fontWeight(.semibold)

                            Text("Beds / Capacity: \(res.allocatedCapacity) persons")
                                .font(.subheadline)

                            Text("Valid Until: \(res.expiresAt)")
                                .font(.caption)
                                .foregroundColor(SthiraTheme.textSecondaryLight)

                            Divider()

                            // Explicit Touch Arrival Confirmation
                            if res.status == "RESERVED" {
                                Button(action: {
                                    showArrivalAlert = true
                                }) {
                                    HStack {
                                        Spacer()
                                        Text("Confirm Arrival at Shelter (Touch Action)")
                                            .font(.system(size: 15, weight: .bold))
                                            .foregroundColor(.white)
                                        Spacer()
                                    }
                                    .padding()
                                    .background(SthiraTheme.safeGreen)
                                    .cornerRadius(10)
                                }
                                .accessibleTarget(minPt: 52)
                                .accessibilityLabel("Confirm Arrival at Shelter")

                                Text("Arrival requires explicit touch action upon physical arrival. Geofencing never auto-confirms.")
                                    .font(.caption2)
                                    .foregroundColor(SthiraTheme.textSecondaryLight)
                            }

                            // Stay Lifecycle Actions: Extend, Depart, Cancel
                            HStack(spacing: 10) {
                                Button(action: {
                                    viewModel.extendStay(reservationId: res.id)
                                }) {
                                    Text("Extend")
                                        .font(.system(size: 13, weight: .semibold))
                                        .frame(maxWidth: .infinity)
                                        .padding(.vertical, 10)
                                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(Color.gray, lineWidth: 1))
                                }
                                .accessibleTarget(minPt: 44)

                                Button(action: {
                                    showDepartAlert = true
                                }) {
                                    Text("Depart")
                                        .font(.system(size: 13, weight: .semibold))
                                        .frame(maxWidth: .infinity)
                                        .padding(.vertical, 10)
                                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(Color.gray, lineWidth: 1))
                                }
                                .accessibleTarget(minPt: 44)

                                Button(action: {
                                    showCancelAlert = true
                                }) {
                                    Text("Cancel")
                                        .font(.system(size: 13, weight: .semibold))
                                        .foregroundColor(SthiraTheme.hazardRed)
                                        .frame(maxWidth: .infinity)
                                        .padding(.vertical, 10)
                                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(SthiraTheme.hazardRed, lineWidth: 1))
                                }
                                .accessibleTarget(minPt: 44)
                            }
                            .padding(.top, 4)
                        }
                        .padding(16)
                        .background(Color(white: 0.97))
                        .cornerRadius(12)

                    } else {
                        // New Reservation Form
                        VStack(alignment: .leading, spacing: 14) {
                            Text("Reserve Shelter Capacity")
                                .font(.headline)

                            CapacityIndicator(
                                remainingCapacity: fac.capacityRemaining,
                                totalCapacity: fac.capacityTotal
                            )

                            Text("Number of Persons in Household / Party:")
                                .font(.subheadline)
                                .fontWeight(.medium)

                            HStack(spacing: 16) {
                                Button(action: {
                                    if partySize > 1 { partySize -= 1 }
                                }) {
                                    Text("-")
                                        .font(.title2)
                                        .frame(width: 44, height: 44)
                                        .background(Color(white: 0.92))
                                        .cornerRadius(8)
                                }
                                .accessibleTarget(minPt: 44)

                                Text("\(partySize)")
                                    .font(.title2)
                                    .fontWeight(.bold)

                                Button(action: {
                                    if partySize < 10 { partySize += 1 }
                                }) {
                                    Text("+")
                                        .font(.title2)
                                        .frame(width: 44, height: 44)
                                        .background(Color(white: 0.92))
                                        .cornerRadius(8)
                                }
                                .accessibleTarget(minPt: 44)
                            }

                            Button(action: {
                                viewModel.reserveShelter(partySize: partySize)
                            }) {
                                HStack {
                                    Spacer()
                                    if viewModel.isProcessing {
                                        ProgressView()
                                            .progressViewStyle(CircularProgressViewStyle(tint: .white))
                                    } else {
                                        Text("Confirm Shelter Reservation")
                                            .font(.system(size: 16, weight: .bold))
                                            .foregroundColor(.white)
                                    }
                                    Spacer()
                                }
                                .padding()
                                .background(SthiraTheme.primaryBlue)
                                .cornerRadius(10)
                            }
                            .disabled(viewModel.isProcessing || (fac.capacityRemaining != nil && fac.capacityRemaining! <= 0))
                            .accessibleTarget(minPt: 52)
                        }
                        .padding(16)
                        .background(Color(white: 0.97))
                        .cornerRadius(12)
                    }

                } else {
                    Text("Please select a facility to manage shelter stay.")
                        .font(.subheadline)
                        .foregroundColor(SthiraTheme.textSecondaryLight)
                        .padding()
                }
            }
            .padding(16)
        }
        .alert(isPresented: $showArrivalAlert) {
            Alert(
                title: Text("Confirm Physical Arrival"),
                message: Text("Have you physically arrived at \(viewModel.selectedFacility?.name ?? "the shelter") and presented yourself to the shelter manager? This updates official capacity."),
                primaryButton: .default(Text("Yes, I Have Arrived")) {
                    if let res = viewModel.activeReservation {
                        viewModel.confirmExplicitArrival(reservationId: res.id)
                    }
                },
                secondaryButton: .cancel()
            )
        }
    }
}
