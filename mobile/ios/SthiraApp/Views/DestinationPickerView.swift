import SwiftUI

struct DestinationPickerView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel
    let onSelectFacility: (Facility) -> Void

    @State private var searchText = ""

    var body: some View {
        VStack(alignment: .leading, spacing: 14) {
            Text("Select Evacuation Destination")
                .font(.title2)
                .fontWeight(.bold)
                .padding(.horizontal, 16)
                .padding(.top, 8)

            // Search bar & Mic button
            HStack(spacing: 10) {
                HStack {
                    Image(systemName: "magnifyingglass")
                        .foregroundColor(.gray)

                    TextField("Search safe zone or village...", text: $searchText)
                        .onChange(of: searchText) { newValue in
                            viewModel.onTextSearch(newValue)
                        }

                    if !searchText.isEmpty {
                        Button(action: {
                            searchText = ""
                            viewModel.onTextSearch("")
                        }) {
                            Image(systemName: "xmark.circle.fill")
                                .foregroundColor(.gray)
                        }
                    }
                }
                .padding(10)
                .background(Color(white: 0.94))
                .cornerRadius(10)

                // Ephemeral Voice Mic Button (IndicConformer ASR)
                Button(action: {
                    viewModel.toggleVoiceInput()
                }) {
                    ZStack {
                        Circle()
                            .fill(viewModel.isVoiceRecording ? SthiraTheme.hazardRed : SthiraTheme.primaryBlue)
                            .frame(width: 48, height: 48)

                        if viewModel.isProcessing {
                            ProgressView()
                                .progressViewStyle(CircularProgressViewStyle(tint: .white))
                        } else {
                            Image(systemName: viewModel.isVoiceRecording ? "stop.fill" : "mic.fill")
                                .foregroundColor(.white)
                                .font(.system(size: 20))
                        }
                    }
                }
                .accessibleTarget(minPt: 48)
                .accessibilityLabel(viewModel.isVoiceRecording ? "Recording voice. Tap to stop." : "Tap to speak your destination")
            }
            .padding(.horizontal, 16)

            // Ambiguous Candidate Chips (409 Conflict Resolution)
            if !viewModel.ambiguousCandidates.isEmpty {
                VStack(alignment: .leading, spacing: 8) {
                    Text("⚠️ Multiple Locations Found")
                        .font(.system(size: 13, weight: .bold))
                        .foregroundColor(SthiraTheme.warningAmber)

                    Text("Please touch your exact location to continue. (Auto-selection disabled for safety):")
                        .font(.caption)
                        .foregroundColor(SthiraTheme.textSecondaryLight)

                    ScrollView(.horizontal, showsIndicators: false) {
                        HStack(spacing: 8) {
                            ForEach(viewModel.ambiguousCandidates) { cand in
                                Button(action: {
                                    viewModel.selectCandidate(cand)
                                }) {
                                    Text(cand.displayName)
                                        .font(.system(size: 13, weight: .semibold))
                                        .foregroundColor(SthiraTheme.primaryBlue)
                                        .padding(.horizontal, 14)
                                        .padding(.vertical, 8)
                                        .background(Color.white)
                                        .overlay(
                                            RoundedRectangle(cornerRadius: 16)
                                                .stroke(SthiraTheme.primaryBlue, lineWidth: 1.5)
                                        )
                                        .clipShape(RoundedRectangle(cornerRadius: 16))
                                }
                                .accessibleTarget(minPt: 44)
                                .accessibilityLabel("Select location candidate: \(cand.displayName)")
                            }
                        }
                        .padding(.vertical, 4)
                    }
                }
                .padding(12)
                .background(SthiraTheme.warningAmberSurface)
                .cornerRadius(10)
                .padding(.horizontal, 16)
            }

            // Facility List
            Text("Designated Government Facilities (\(viewModel.facilities.count))")
                .font(.headline)
                .padding(.horizontal, 16)

            ScrollView {
                LazyVStack(spacing: 12) {
                    ForEach(viewModel.facilities) { fac in
                        let isFull = fac.capacityRemaining != nil && fac.capacityRemaining! <= 0

                        VStack(alignment: .leading, spacing: 10) {
                            HStack {
                                Text(fac.name)
                                    .font(.system(size: 16, weight: .bold))
                                Spacer()
                                CapacityIndicator(
                                    remainingCapacity: fac.capacityRemaining,
                                    totalCapacity: fac.capacityTotal
                                )
                            }

                            Text("Safe Zone: \(fac.safeZoneId) • (\(String(format: "%.4f", fac.latitude)), \(String(format: "%.4f", fac.longitude)))")
                                .font(.caption)
                                .foregroundColor(SthiraTheme.textSecondaryLight)

                            Button(action: {
                                viewModel.selectFacility(fac)
                                onSelectFacility(fac)
                            }) {
                                HStack {
                                    Spacer()
                                    Text(isFull ? "Shelter Full — Select Another Facility" : "Select & Route Here →")
                                        .font(.system(size: 14, weight: .bold))
                                        .foregroundColor(.white)
                                    Spacer()
                                }
                                .padding(.vertical, 12)
                                .background(isFull ? Color(white: 0.6) : SthiraTheme.primaryBlue)
                                .cornerRadius(8)
                            }
                            .disabled(isFull)
                            .accessibleTarget(minPt: 44)
                            .accessibilityLabel(isFull ? "\(fac.name) is full" : "Select and route to \(fac.name)")
                        }
                        .padding(14)
                        .background(Color(white: 0.97))
                        .cornerRadius(12)
                        .padding(.horizontal, 16)
                    }
                }
            }
        }
    }
}
