import SwiftUI

struct LanguageOption: Identifiable {
    let id: String
    let vernacularName: String
    let englishName: String
}

struct LanguageView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel

    let languages = [
        LanguageOption(id: "ml", vernacularName: "മലയാളം", englishName: "Malayalam (Kerala)"),
        LanguageOption(id: "hi", vernacularName: "हिन्दी", englishName: "Hindi (National / Regional)"),
        LanguageOption(id: "en", vernacularName: "English", englishName: "English (Official Fallback)")
    ]

    var body: some View {
        VStack(spacing: 20) {
            Text("Select Language / ഭാഷ തിരഞ്ഞെടുക്കുക")
                .font(.title2)
                .fontWeight(.bold)
                .multilineTextAlignment(.center)

            Text("Choose your preferred language for voice and emergency alerts")
                .font(.subheadline)
                .foregroundColor(SthiraTheme.textSecondaryLight)
                .multilineTextAlignment(.center)
                .padding(.horizontal)

            VStack(spacing: 14) {
                ForEach(languages) { lang in
                    let isSelected = viewModel.selectedLanguage == lang.id

                    Button(action: {
                        viewModel.selectLanguage(lang.id)
                    }) {
                        HStack {
                            Image(systemName: isSelected ? "largecircle.fill.circle" : "circle")
                                .foregroundColor(isSelected ? SthiraTheme.primaryBlue : .gray)
                                .font(.title3)

                            VStack(alignment: .leading, spacing: 2) {
                                Text(lang.vernacularName)
                                    .font(.headline)
                                    .foregroundColor(.primary)

                                Text(lang.englishName)
                                    .font(.caption)
                                    .foregroundColor(SthiraTheme.textSecondaryLight)
                            }
                            .padding(.leading, 8)

                            Spacer()
                        }
                        .padding()
                        .background(isSelected ? SthiraTheme.primaryBlueLight.opacity(0.15) : Color(white: 0.96))
                        .overlay(
                            RoundedRectangle(cornerRadius: 12)
                                .stroke(isSelected ? SthiraTheme.primaryBlue : Color(white: 0.85), lineWidth: isSelected ? 2 : 1)
                        )
                        .clipShape(RoundedRectangle(cornerRadius: 12))
                    }
                    .accessibleTarget(minPt: 54)
                    .accessibilityLabel("\(lang.vernacularName), \(lang.englishName). \(isSelected ? "Selected" : "Not selected")")
                }
            }
            .padding(.horizontal, 20)

            Spacer()
        }
        .padding(.top, 40)
    }
}
