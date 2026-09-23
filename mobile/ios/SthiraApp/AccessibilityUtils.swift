import SwiftUI
import UIKit

// Minimum target size per WCAG 2.1 (>= 44pt on iOS)
extension View {
    func accessibleTarget(minPt: CGFloat = 44) -> some View {
        self.frame(minWidth: minPt, minHeight: minPt)
    }
}

enum HazardLevel {
    case criticalRedZone
    case safeZone
    case advisoryWarning
    case unknownCapacity
}

/// Non-color-only hazard badge: combines geometry/shape, icon, bold text, and VoiceOver traits.
struct HazardBadge: View {
    let level: HazardLevel
    let label: String

    var body: some View {
        let config = configuration(for: level)

        HStack(spacing: 6) {
            Text(config.symbol)
                .font(.system(size: 12, weight: .bold))
                .foregroundColor(config.textColor)

            Text(label)
                .font(.system(size: 13, weight: .semibold))
                .foregroundColor(config.textColor)
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 6)
        .background(config.bgColor)
        .overlay(
            RoundedRectangle(cornerRadius: config.cornerRadius)
                .stroke(config.borderColor, lineWidth: 1.5)
        )
        .clipShape(RoundedRectangle(cornerRadius: config.cornerRadius))
        .accessibilityElement(children: .combine)
        .accessibilityLabel("\(config.a11yPrefix) \(label)")
    }

    private func configuration(for level: HazardLevel) -> (
        bgColor: Color,
        borderColor: Color,
        textColor: Color,
        symbol: String,
        a11yPrefix: String,
        cornerRadius: CGFloat
    ) {
        switch level {
        case .criticalRedZone:
            return (
                SthiraTheme.hazardRedSurface,
                SthiraTheme.hazardRed,
                SthiraTheme.hazardRed,
                "⛔ [STOP / RED ZONE]",
                "Danger Red Zone:",
                4.0
            )
        case .safeZone:
            return (
                SthiraTheme.safeGreenSurface,
                SthiraTheme.safeGreen,
                SthiraTheme.safeGreen,
                "🛡️ [SAFE ZONE]",
                "Designated Safe Zone:",
                12.0
            )
        case .advisoryWarning:
            return (
                SthiraTheme.warningAmberSurface,
                SthiraTheme.warningAmber,
                SthiraTheme.warningAmber,
                "⚠️ [CAUTION]",
                "Warning Advisory:",
                6.0
            )
        case .unknownCapacity:
            return (
                Color(red: 237/255, green: 231/255, blue: 246/255),
                Color(red: 81/255, green: 45/255, blue: 168/255),
                Color(red: 81/255, green: 45/255, blue: 168/255),
                "❓ [UNCONFIRMED]",
                "Capacity Status Unknown:",
                6.0
            )
        }
    }
}

/// Safe emergency dialler handoff: triggers device telephone dialler with 112
/// upon explicit citizen touch. Never makes automatic calls without user action.
struct EmergencyDialler {
    static func call112(number: String = "112") {
        let cleaned = number.filter { $0.isNumber || $0 == "+" }
        guard let url = URL(string: "tel://\(cleaned)"),
              UIApplication.shared.canOpenURL(url) else {
            return
        }
        UIApplication.shared.open(url, options: [:], completionHandler: nil)
    }
}
