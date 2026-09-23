import SwiftUI

struct ForegroundTrackingView: View {
    @ObservedObject var locationManager = LocationManager.shared
    @ObservedObject var viewModel: CitizenJourneyViewModel

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            HStack {
                let (statusColor, statusLabel) = statusInfo(for: locationManager.currentState)

                Circle()
                    .fill(statusColor)
                    .frame(width: 12, height: 12)

                Text(statusLabel)
                    .font(.system(size: 14, weight: .bold))
                    .foregroundColor(statusColor)

                Spacer()
            }

            if let dist = locationManager.distanceMeters {
                let formatted = dist >= 1000 ? String(format: "%.2f km", dist / 1000.0) : "\(Int(dist)) meters"
                Text("Distance to Shelter: \(formatted)")
                    .font(.title3)
                    .fontWeight(.bold)
            }

            if let reading = locationManager.lastReading {
                Text(String(format: "GPS Accuracy: ±%.1fm", reading.accuracyMeters))
                    .font(.caption)
                    .foregroundColor(SthiraTheme.textSecondaryLight)
            }

            // Proximity Advisory (O10: Advisory only, never auto-confirms arrival)
            if locationManager.currentState == .nearDestination {
                VStack(alignment: .leading, spacing: 4) {
                    Text("📍 You are near your designated shelter!")
                        .font(.system(size: 14, weight: .bold))
                        .foregroundColor(SthiraTheme.safeGreen)

                    Text("Please tap 'Confirm Arrival' below once you have checked in with the reception manager.")
                        .font(.caption)
                        .foregroundColor(Color(red: 46/255, green: 125/255, blue: 50/255))
                }
                .padding(12)
                .frame(maxWidth: .infinity, alignment: .leading)
                .background(SthiraTheme.safeGreenSurface)
                .overlay(RoundedRectangle(cornerRadius: 8).stroke(SthiraTheme.safeGreen, lineWidth: 1))
                .cornerRadius(8)
            }

            HStack(spacing: 12) {
                if locationManager.currentState == .notStarted || locationManager.currentState == .paused {
                    Button(action: {
                        viewModel.startNavigation()
                    }) {
                        HStack {
                            Spacer()
                            Text("Start Tracking")
                                .font(.system(size: 14, weight: .bold))
                                .foregroundColor(.white)
                            Spacer()
                        }
                        .padding(.vertical, 12)
                        .background(SthiraTheme.primaryBlue)
                        .cornerRadius(8)
                    }
                    .accessibleTarget(minPt: 48)
                } else {
                    Button(action: {
                        viewModel.stopNavigation()
                    }) {
                        HStack {
                            Spacer()
                            Text("Stop Tracking")
                                .font(.system(size: 14, weight: .bold))
                                .foregroundColor(.primary)
                            Spacer()
                        }
                        .padding(.vertical, 12)
                        .overlay(RoundedRectangle(cornerRadius: 8).stroke(Color.gray, lineWidth: 1))
                    }
                    .accessibleTarget(minPt: 48)
                }

                if locationManager.currentState == .nearDestination || locationManager.currentState == .tracking {
                    Button(action: {
                        if let res = viewModel.activeReservation {
                            viewModel.confirmExplicitArrival(reservationId: res.id)
                        } else {
                            locationManager.confirmExplicitArrival()
                        }
                    }) {
                        HStack {
                            Spacer()
                            Text("Touch Arrived")
                                .font(.system(size: 14, weight: .bold))
                                .foregroundColor(.white)
                            Spacer()
                        }
                        .padding(.vertical, 12)
                        .background(SthiraTheme.safeGreen)
                        .cornerRadius(8)
                    }
                    .accessibleTarget(minPt: 48)
                }
            }
        }
        .padding(16)
        .background(Color(white: 0.97))
        .cornerRadius(12)
    }

    private func statusInfo(for state: JourneyState) -> (Color, String) {
        switch state {
        case .notStarted:
            return (Color.gray, "Navigation Inactive")
        case .tracking:
            return (SthiraTheme.primaryBlue, "Foreground Tracking Active")
        case .nearDestination:
            return (SthiraTheme.safeGreen, "Near Destination (Within 75m)")
        case .arrivalReported:
            return (SthiraTheme.safeGreen, "Arrival Confirmed by Citizen")
        case .paused:
            return (SthiraTheme.warningAmber, "Navigation Paused")
        case .locationUnavailable:
            return (SthiraTheme.hazardRed, "GPS Signal Weak / Denied")
        case .routeRevoked:
            return (SthiraTheme.hazardRed, "Corridor Revoked by Authority")
        }
    }
}
