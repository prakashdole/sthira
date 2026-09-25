import SwiftUI

struct RouteGuidanceView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel
    let onNavigateToStay: () -> Void

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 14) {
                if let fac = viewModel.selectedFacility {
                    Text("Evacuation Route Guidance")
                        .font(.title2)
                        .fontWeight(.bold)

                    Text("Destination: \(fac.name) (Safe Zone \(fac.safeZoneId))")
                        .font(.subheadline)
                        .foregroundColor(SthiraTheme.textSecondaryLight)

                    // Route Revocation Alert
                    if viewModel.isRouteRevoked {
                        VStack(alignment: .leading, spacing: 6) {
                            HazardBadge(level: .criticalRedZone, label: "ROUTE REVOKED BY AUTHORITY")
                            Text("This evacuation corridor has been declared unsafe (e.g. flooded road / landslide). Do not proceed along this route. Please return to destination selection.")
                                .font(.subheadline)
                                .foregroundColor(SthiraTheme.hazardRed)
                        }
                        .padding(14)
                        .frame(maxWidth: .infinity, alignment: .leading)
                        .background(SthiraTheme.hazardRedSurface)
                        .cornerRadius(10)
                    }

                    // Vector Map Container
                    ZStack {
                        RoundedRectangle(cornerRadius: 12)
                            .fill(Color(white: 0.9))
                            .frame(height: 180)

                        VStack(spacing: 4) {
                            Text("🗺️ Vector Map Display")
                                .font(.system(size: 15, weight: .bold))
                                .foregroundColor(Color(white: 0.2))

                            Text("Approved Government Corridor #\(viewModel.activeRoute?.id ?? "N/A")")
                                .font(.caption)
                                .foregroundColor(Color(white: 0.4))
                        }
                    }
                    .accessibilityLabel("Vector Map: Approved corridor to \(fac.name)")

                    // Accessible Non-Map Turn-by-Turn Text Instructions
                    Text("Turn-by-Turn Directions (Accessible Text)")
                        .font(.headline)
                        .padding(.top, 6)

                    if let route = viewModel.activeRoute {
                        VStack(spacing: 10) {
                            ForEach(Array(route.instructions.enumerated()), id: \.element.id) { index, step in
                                HStack(spacing: 12) {
                                    ZStack {
                                        Circle()
                                            .fill(SthiraTheme.primaryBlue)
                                            .frame(width: 28, height: 28)

                                        Text("\(index + 1)")
                                            .font(.system(size: 12, weight: .bold))
                                            .foregroundColor(.white)
                                    }

                                    VStack(alignment: .leading, spacing: 2) {
                                        Text(step.instruction)
                                            .font(.system(size: 14, weight: .medium))

                                        Text("\(step.distanceMeters)m • \(step.maneuver)")
                                            .font(.caption)
                                            .foregroundColor(SthiraTheme.textSecondaryLight)
                                    }

                                    Spacer()
                                }
                                .padding(.vertical, 4)
                            }
                        }
                        .padding(14)
                        .background(Color(white: 0.97))
                        .cornerRadius(12)
                    }

                    // Action Buttons
                    HStack(spacing: 12) {
                        Button(action: {
                            viewModel.startNavigation()
                        }) {
                            HStack {
                                Spacer()
                                Text("Start GPS Guidance")
                                    .font(.system(size: 15, weight: .bold))
                                    .foregroundColor(.white)
                                Spacer()
                            }
                            .padding()
                            .background(viewModel.isRouteRevoked ? Color.gray : SthiraTheme.primaryBlue)
                            .cornerRadius(10)
                        }
                        .disabled(viewModel.isRouteRevoked)
                        .accessibleTarget(minPt: 50)
                        .accessibilityLabel("Start GPS Guidance")

                        Button(action: onNavigateToStay) {
                            HStack {
                                Spacer()
                                Text("Reserve / Stay")
                                    .font(.system(size: 15, weight: .bold))
                                    .foregroundColor(.white)
                                Spacer()
                            }
                            .padding()
                            .background(Color(red: 46/255, green: 125/255, blue: 50/255))
                            .cornerRadius(10)
                        }
                        .accessibleTarget(minPt: 50)
                        .accessibilityLabel("Reserve Shelter Stay")
                    }
                    .padding(.top, 10)

                } else {
                    Text("Please select a safe destination first.")
                        .font(.subheadline)
                        .foregroundColor(SthiraTheme.textSecondaryLight)
                        .padding()
                }
            }
            .padding(16)
        }
    }
}
