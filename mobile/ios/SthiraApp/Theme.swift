import SwiftUI

/// Accessible color palette adhering to WCAG 2.1 AAA contrast requirements.
struct SthiraTheme {
    static let primaryBlue = Color(red: 13/255, green: 71/255, blue: 161/255) // #0D47A1
    static let primaryBlueDark = Color(red: 0/255, green: 33/255, blue: 113/255) // #002171
    static let primaryBlueLight = Color(red: 84/255, green: 114/255, blue: 211/255) // #5472D3

    static let hazardRed = Color(red: 183/255, green: 28/255, blue: 28/255) // #B71C1C
    static let hazardRedSurface = Color(red: 255/255, green: 235/255, blue: 238/255) // #FFEBEE

    static let safeGreen = Color(red: 27/255, green: 94/255, blue: 32/255) // #1B5E20
    static let safeGreenSurface = Color(red: 232/255, green: 245/255, blue: 233/255) // #E8F5E9

    static let warningAmber = Color(red: 230/255, green: 81/255, blue: 0/255) // #E65100
    static let warningAmberSurface = Color(red: 255/255, green: 243/255, blue: 224/255) // #FFF3E0

    static let syntheticDemoBanner = Color(red: 255/255, green: 111/255, blue: 0/255) // #FF6F00

    static let surfaceLight = Color(red: 251/255, green: 251/255, blue: 251/255)
    static let backgroundLight = Color(red: 240/255, green: 242/255, blue: 245/255)
    static let textPrimaryLight = Color(red: 26/255, green: 26/255, blue: 26/255)
    static let textSecondaryLight = Color(red: 74/255, green: 74/255, blue: 74/255)
}
