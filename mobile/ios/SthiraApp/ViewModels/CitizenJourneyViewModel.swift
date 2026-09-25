import Foundation
import SwiftUI

struct PublicIncident: Identifiable {
    let id: String
    let jurisdiction: String
    let title: String
    let description: String
    let issuedAt: String
    let sourceId: String
    let redZones: [RedZone]
    let safeZones: [SafeZone]
}

struct RedZone: Identifiable {
    let id: String
    let name: String
    let hazardType: String
    let instructions: String
}

struct SafeZone: Identifiable {
    let id: String
    let name: String
    let facilities: [String]
}

struct Facility: Identifiable {
    let id: String
    let name: String
    let safeZoneId: String
    let latitude: Double
    let longitude: Double
    let capacityTotal: Int?
    let capacityRemaining: Int?
}

struct PlaceCandidate: Identifiable {
    let id: String
    let displayName: String
    let latitude: Double
    let longitude: Double
    let score: Double
}

struct InstructionStep: Identifiable {
    let id = UUID()
    let instruction: String
    let distanceMeters: Int
    let maneuver: String
}

struct RouteInfo: Identifiable {
    let id: String
    let safeZoneId: String
    let instructions: [InstructionStep]
}

struct Reservation: Identifiable {
    let id: String
    let facilityId: String
    let allocatedCapacity: Int
    var status: String
    var expiresAt: String
}

class CitizenJourneyViewModel: ObservableObject {
    @Published var selectedLanguage: String = "ml"
    @Published var isSyntheticExercise: Bool = true
    @Published var incident: PublicIncident?
    @Published var facilities: [Facility] = []
    @Published var ambiguousCandidates: [PlaceCandidate] = []
    @Published var selectedFacility: Facility?
    @Published var activeRoute: RouteInfo?
    @Published var isRouteRevoked: Bool = false
    @Published var activeReservation: Reservation?
    @Published var isOffline: Bool = false
    @Published var isVoiceRecording: Bool = false
    @Published var isProcessing: Bool = false
    @Published var statusMessage: String?

    let audioRecorder = EphemeralAudioRecorder()
    let locationManager = LocationManager.shared

    init() {
        loadInitialData()
    }

    private func loadInitialData() {
        self.incident = PublicIncident(
            id: "INC-KL-2024-WYND-01",
            jurisdiction: "IN-KL-WYND",
            title: "Wayanad Landslide & Flash Flood Evacuation Advisory",
            description: "Heavy rainfall triggered active landslides in Meppadi panchayat. Citizens in marked red zones must move immediately to designated safe facilities.",
            issuedAt: "2024-07-30T04:30:00Z",
            sourceId: "KSDMA-OPERATIONAL",
            redZones: [
                RedZone(
                    id: "RZ-MEPPADI-01",
                    name: "Meppadi Hill Slopes & River Basin",
                    hazardType: "Landslide / Flash Flood",
                    instructions: "Evacuate immediately via approved bypass corridor. Do not use river bridge."
                )
            ],
            safeZones: [
                SafeZone(
                    id: "SZ-VYTHIRI-01",
                    name: "Vythiri High Grounds Safe Zone",
                    facilities: ["FAC-VYTHIRI-CH-01", "FAC-MEPPADI-SCH-02"]
                )
            ]
        )

        self.facilities = [
            Facility(
                id: "FAC-VYTHIRI-CH-01",
                name: "Vythiri Community Hall Shelter",
                safeZoneId: "SZ-VYTHIRI-01",
                latitude: 11.5512,
                longitude: 76.0415,
                capacityTotal: 250,
                capacityRemaining: 78
            ),
            Facility(
                id: "FAC-MEPPADI-SCH-02",
                name: "St. Joseph High School Relief Camp",
                safeZoneId: "SZ-VYTHIRI-01",
                latitude: 11.5480,
                longitude: 76.1280,
                capacityTotal: 400,
                capacityRemaining: 120
            ),
            Facility(
                id: "FAC-KALPETTA-GOVT-03",
                name: "Kalpetta Town Relief Centre",
                safeZoneId: "SZ-VYTHIRI-01",
                latitude: 11.6100,
                longitude: 76.0820,
                capacityTotal: 150,
                capacityRemaining: 0 // Full shelter
            )
        ]
    }

    func selectLanguage(_ code: String) {
        self.selectedLanguage = code
    }

    func toggleVoiceInput() {
        if isVoiceRecording {
            isVoiceRecording = false
            isProcessing = true
            audioRecorder.stopRecording { [weak self] result in
                DispatchQueue.main.async {
                    self?.isProcessing = false
                    switch result {
                    case .success(let wavData):
                        self?.statusMessage = "Voice processed (\(wavData.count) bytes). Filtering safe shelters."
                    case .failure(let error):
                        self?.statusMessage = "Voice error: \(error.localizedDescription)"
                    }
                }
            }
        } else {
            isVoiceRecording = true
            isProcessing = false
            audioRecorder.startRecording { [weak self] result in
                DispatchQueue.main.async {
                    self?.isVoiceRecording = false
                    self?.isProcessing = false
                    switch result {
                    case .success(let wavData):
                        self?.statusMessage = "Voice processed (\(wavData.count) bytes). Filtering safe shelters."
                    case .failure(let error):
                        self?.statusMessage = "Voice error: \(error.localizedDescription)"
                    }
                }
            }
        }
    }

    func onTextSearch(_ query: String) {
        if query.lowercased().contains("meppadi") {
            // Ambiguous place 409 simulation: returns candidate chips
            self.ambiguousCandidates = [
                PlaceCandidate(id: "LOC-MEP-01", displayName: "Meppadi Village (Near Church)", latitude: 11.5501, longitude: 76.1211, score: 0.95),
                PlaceCandidate(id: "LOC-MEP-02", displayName: "Meppadi Junction (Bus Stand)", latitude: 11.5542, longitude: 76.1265, score: 0.91)
            ]
        } else {
            self.ambiguousCandidates = []
        }
    }

    func selectCandidate(_ candidate: PlaceCandidate) {
        self.ambiguousCandidates = []
        self.statusMessage = "Location selected: \(candidate.displayName)"
    }

    func selectFacility(_ facility: Facility) {
        self.selectedFacility = facility
        self.activeRoute = RouteInfo(
            id: "RTE-GOV-WYND-001",
            safeZoneId: facility.safeZoneId,
            instructions: [
                InstructionStep(instruction: "Head northwest on Meppadi Bypass Road", distanceMeters: 450, maneuver: "DEPART"),
                InstructionStep(instruction: "Turn left at Checkpoint Alpha onto High Ground Corridor", distanceMeters: 1200, maneuver: "TURN_LEFT"),
                InstructionStep(instruction: "Arrive at \(facility.name) on the right", distanceMeters: 150, maneuver: "ARRIVE")
            ]
        )
        self.isRouteRevoked = false
    }

    func reserveShelter(partySize: Int) {
        guard let fac = selectedFacility else { return }
        isProcessing = true

        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { [weak self] in
            let resId = "RES-" + UUID().uuidString.prefix(8).uppercased()
            self?.isProcessing = false
            self?.activeReservation = Reservation(
                id: resId,
                facilityId: fac.id,
                allocatedCapacity: partySize,
                status: "RESERVED",
                expiresAt: "2024-07-31T04:30:00Z"
            )
            self?.statusMessage = "Reservation confirmed for \(partySize) persons."
        }
    }

    func confirmExplicitArrival(reservationId: String) {
        // Explicit touch action: updates arrival state, stops tracking
        locationManager.confirmExplicitArrival()
        self.activeReservation?.status = "ARRIVED"
        self.statusMessage = "Arrival confirmed at shelter desk. Capacity updated."
    }

    func extendStay(reservationId: String) {
        self.activeReservation?.expiresAt = "2024-08-07T04:30:00Z"
        self.statusMessage = "Stay extension approved within policy limits."
    }

    func departStay(reservationId: String) {
        locationManager.stopTracking()
        self.activeReservation = nil
        self.selectedFacility = nil
        self.activeRoute = nil
        self.statusMessage = "Departed facility. Capacity released back to authority pool."
    }

    func cancelStay(reservationId: String) {
        locationManager.stopTracking()
        self.activeReservation = nil
        self.selectedFacility = nil
        self.statusMessage = "Reservation cancelled. Capacity released."
    }

    func startNavigation() {
        guard let fac = selectedFacility else { return }
        locationManager.requestPermission()
        locationManager.startTracking(targetLatitude: fac.latitude, targetLongitude: fac.longitude)
    }

    func stopNavigation() {
        locationManager.stopTracking()
    }
}
