import SwiftUI

/// Banner identifying synthetic drill exercise data to prevent citizen panic.
struct SyntheticExerciseBanner: View {
    let isExercise: BooleanLiteralType

    var body: some View {
        if isExercise {
            HStack {
                Spacer()
                Text("⚠️ SYNTHETIC EXERCISE — NOT A REAL DISASTER ORDER")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundColor(.white)
                    .multilineTextAlignment(.center)
                Spacer()
            }
            .padding(.vertical, 8)
            .padding(.horizontal, 16)
            .background(SthiraTheme.syntheticDemoBanner)
            .accessibilityElement(children: .combine)
            .accessibilityLabel("Notice: Synthetic disaster drill exercise in progress. This is not a real evacuation order.")
        }
    }
}

/// Displays data freshness status honestly (FRESH, STALE, UNVERIFIABLE, EXPIRED).
struct FreshnessBadge: View {
    let freshness: String
    let sourceId: String

    var body: some View {
        let (bgColor, textColor, label) = config(for: freshness)

        HStack(spacing: 4) {
            Text("\(label) • \(sourceId)")
                .font(.system(size: 11, weight: .semibold))
                .foregroundColor(textColor)
        }
        .padding(.horizontal, 8)
        .padding(.vertical, 4)
        .background(bgColor)
        .overlay(
            RoundedRectangle(cornerRadius: 4)
                .stroke(textColor, lineWidth: 1)
        )
        .clipShape(RoundedRectangle(cornerRadius: 4))
        .accessibilityLabel("Data status: \(label) from source \(sourceId)")
    }

    private func config(for freshness: String) -> (Color, Color, String) {
        switch freshness {
        case "FRESH":
            return (SthiraTheme.safeGreenSurface, SthiraTheme.safeGreen, "Verified Live")
        case "STALE":
            return (SthiraTheme.warningAmberSurface, SthiraTheme.warningAmber, "Stale Data (Needs Recheck)")
        case "UNVERIFIABLE":
            return (SthiraTheme.hazardRedSurface, SthiraTheme.hazardRed, "Unverifiable (Clock Mismatch)")
        case "EXPIRED":
            return (SthiraTheme.hazardRedSurface, SthiraTheme.hazardRed, "Expired Advisory")
        default:
            return (Color(white: 0.9), Color(white: 0.4), "Withdrawn")
        }
    }
}

/// Prominent card offering instant emergency call to 112 / local disaster control room.
struct EmergencyCallCard: View {
    let officialNumber: String

    init(officialNumber: String = "112") {
        self.officialNumber = officialNumber
    }

    var body: some View {
        HStack {
            VStack(alignment: .leading, spacing: 2) {
                Text("Immediate Danger?")
                    .font(.system(size: 16, weight: .bold))
                    .foregroundColor(SthiraTheme.hazardRed)

                Text("Contact official disaster emergency services")
                    .font(.system(size: 12))
                    .foregroundColor(SthiraTheme.textSecondaryLight)
            }

            Spacer()

            Button(action: {
                EmergencyDialler.call112(number: officialNumber)
            }) {
                Text("Call \(officialNumber)")
                    .font(.system(size: 14, weight: .bold))
                    .foregroundColor(.white)
                    .padding(.horizontal, 14)
                    .padding(.vertical, 10)
                    .background(SthiraTheme.hazardRed)
                    .cornerRadius(8)
            }
            .accessibleTarget(minPt: 44)
            .accessibilityLabel("Call emergency services at \(officialNumber)")
        }
        .padding(14)
        .background(SthiraTheme.hazardRedSurface)
        .cornerRadius(10)
    }
}

/// Displays facility bed capacity truthfully without guessing.
struct CapacityIndicator: View {
    let remainingCapacity: Int?
    let totalCapacity: Int?

    var body: some View {
        if let rem = remainingCapacity, let tot = totalCapacity {
            if rem <= 0 {
                Text("⛔ Full (0/\(tot) Available)")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundColor(SthiraTheme.hazardRed)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(SthiraTheme.hazardRedSurface)
                    .cornerRadius(4)
                    .accessibilityLabel("Shelter at maximum capacity. 0 beds available.")
            } else {
                Text("✓ \(rem)/\(tot) Available")
                    .font(.system(size: 12, weight: .bold))
                    .foregroundColor(SthiraTheme.safeGreen)
                    .padding(.horizontal, 8)
                    .padding(.vertical, 4)
                    .background(SthiraTheme.safeGreenSurface)
                    .cornerRadius(4)
                    .accessibilityLabel("\(rem) of \(tot) beds available")
            }
        } else {
            Text("❓ Capacity: Unconfirmed")
                .font(.system(size: 12, weight: .semibold))
                .foregroundColor(Color(red: 81/255, green: 45/255, blue: 168/255))
                .padding(.horizontal, 8)
                .padding(.vertical, 4)
                .background(Color(red: 237/255, green: 231/255, blue: 246/255))
                .cornerRadius(4)
                .accessibilityLabel("Capacity status unconfirmed by authority")
        }
    }
}
