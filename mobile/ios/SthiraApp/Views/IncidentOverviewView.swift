import SwiftUI

struct IncidentOverviewView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel
    let onNavigateToDestinations: () -> Void

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 16) {
                SyntheticExerciseBanner(isExercise: viewModel.isSyntheticExercise)

                VStack(alignment: .leading, spacing: 16) {
                    EmergencyCallCard(officialNumber: "112")

                    if let inc = viewModel.incident {
                        // Official Advisory Card
                        VStack(alignment: .leading, spacing: 8) {
                            HStack {
                                Text("OFFICIAL DISASTER ADVISORY")
                                    .font(.system(size: 11, weight: .bold))
                                    .foregroundColor(SthiraTheme.primaryBlue)

                                Spacer()

                                FreshnessBadge(freshness: "FRESH", sourceId: inc.sourceId)
                            }

                            Text(inc.title)
                                .font(.title3)
                                .fontWeight(.bold)

                            Text("Jurisdiction: \(inc.jurisdiction) • Issued: \(inc.issuedAt)")
                                .font(.caption)
                                .foregroundColor(SthiraTheme.textSecondaryLight)

                            Divider()

                            Text(inc.description)
                                .font(.body)
                        }
                        .padding(16)
                        .background(Color(white: 0.97))
                        .cornerRadius(12)

                        // Red Zones
                        Text("Active Red Zones (\(inc.redZones.count))")
                            .font(.headline)
                            .padding(.top, 4)

                        ForEach(inc.redZones) { rz in
                            VStack(alignment: .leading, spacing: 6) {
                                HazardBadge(level: .criticalRedZone, label: rz.name)

                                Text(rz.hazardType.uppercased())
                                    .font(.system(size: 12, weight: .bold))
                                    .foregroundColor(SthiraTheme.hazardRed)

                                Text(rz.instructions)
                                    .font(.subheadline)
                            }
                            .padding(14)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(SthiraTheme.hazardRedSurface)
                            .cornerRadius(10)
                        }

                        // Safe Zones
                        Text("Designated Safe Zones (\(inc.safeZones.count))")
                            .font(.headline)
                            .padding(.top, 4)

                        ForEach(inc.safeZones) { sz in
                            VStack(alignment: .leading, spacing: 4) {
                                HazardBadge(level: .safeZone, label: sz.name)

                                Text("Member Facilities: \(sz.facilities.joined(separator: ", "))")
                                    .font(.caption)
                                    .foregroundColor(SthiraTheme.safeGreen)
                            }
                            .padding(14)
                            .frame(maxWidth: .infinity, alignment: .leading)
                            .background(SthiraTheme.safeGreenSurface)
                            .cornerRadius(10)
                        }

                        // CTA Button
                        Button(action: onNavigateToDestinations) {
                            HStack {
                                Spacer()
                                Text("Find Safe Evacuation Shelters →")
                                    .font(.system(size: 16, weight: .bold))
                                    .foregroundColor(.white)
                                Spacer()
                            }
                            .padding()
                            .background(SthiraTheme.primaryBlue)
                            .cornerRadius(10)
                        }
                        .accessibleTarget(minPt: 52)
                        .accessibilityLabel("Find Safe Evacuation Shelters")
                        .padding(.top, 8)
                    }
                }
                .padding(.horizontal, 16)
            }
        }
    }
}
