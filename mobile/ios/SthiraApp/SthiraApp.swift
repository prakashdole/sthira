import SwiftUI

@main
struct SthiraApp: App {
    @StateObject private var viewModel = CitizenJourneyViewModel()

    var body: some Scene {
        WindowGroup {
            ContentView(viewModel: viewModel)
        }
    }
}

struct ContentView: View {
    @ObservedObject var viewModel: CitizenJourneyViewModel
    @State private var selectedTab = 0

    var body: some View {
        ZStack(alignment: .bottom) {
            TabView(selection: $selectedTab) {
                IncidentOverviewView(viewModel: viewModel) {
                    selectedTab = 1
                }
                .tabItem {
                    Label("Alerts", systemImage: "exclamationmark.triangle.fill")
                }
                .tag(0)

                DestinationPickerView(viewModel: viewModel) { selectedFacility in
                    selectedTab = 2
                }
                .tabItem {
                    Label("Shelters", systemImage: "tent.fill")
                }
                .tag(1)

                RouteGuidanceView(viewModel: viewModel) {
                    selectedTab = 3
                }
                .tabItem {
                    Label("Guidance", systemImage: "map.fill")
                }
                .tag(2)

                StayManagementView(viewModel: viewModel)
                    .tabItem {
                        Label("My Stay", systemImage: "person.crop.circle.badge.checkmark")
                    }
                    .tag(3)

                LanguageView(viewModel: viewModel)
                    .tabItem {
                        Label("Language", systemImage: "globe")
                    }
                    .tag(4)
            }

            // Status message toast
            if let msg = viewModel.statusMessage {
                VStack {
                    Spacer()
                    Text(msg)
                        .font(.system(size: 13, weight: .semibold))
                        .foregroundColor(.white)
                        .padding(.horizontal, 16)
                        .padding(.vertical, 10)
                        .background(Color.black.opacity(0.85))
                        .cornerRadius(20)
                        .padding(.bottom, 60)
                        .transition(.move(edge: .bottom).combined(with: .opacity))
                }
                .animation(.easeInOut, value: msg)
            }
        }
        .accentColor(SthiraTheme.primaryBlue)
    }
}
