import Foundation
import CoreLocation
import SwiftUI

enum JourneyState: String {
    case notStarted = "NOT_STARTED"
    case tracking = "TRACKING"
    case nearDestination = "NEAR_DESTINATION"
    case arrivalReported = "ARRIVAL_REPORTED"
    case paused = "PAUSED"
    case locationUnavailable = "LOCATION_UNAVAILABLE"
    case routeRevoked = "ROUTE_REVOKED"
}

struct PositionReading {
    let latitude: Double
    let longitude: Double
    let accuracyMeters: Double
    let timestampEpochMs: Int64
}

/// LocationManager: opt-in foreground evacuation guidance adhering strictly to O10.
/// - In-use authorization only (no continuous background tracking).
/// - Enforces accuracy <= 100m and age <= 30s gates.
/// - Proximity alert at <= 75m; explicit touch arrival confirmation required.
class LocationManager: NSObject, ObservableObject, CLLocationManagerDelegate {
    static let shared = LocationManager()

    private let manager = CLLocationManager()

    @Published var currentState: JourneyState = .notStarted
    @Published var lastReading: PositionReading?
    @Published var distanceMeters: Double?
    @Published var isAuthorized: Bool = false

    private var targetLat: Double?
    private var targetLon: Double?

    override init() {
        super.init()
        manager.delegate = self
        manager.desiredAccuracy = kCLLocationAccuracyBest
        manager.allowsBackgroundLocationUpdates = false // O10: zero background surveillance
    }

    func requestPermission() {
        manager.requestWhenInUseAuthorization()
    }

    func startTracking(targetLatitude: Double, targetLongitude: Double) {
        self.targetLat = targetLatitude
        self.targetLon = targetLongitude
        self.currentState = .tracking
        manager.startUpdatingLocation()
    }

    func stopTracking() {
        manager.stopUpdatingLocation()
        if currentState != .arrivalReported {
            currentState = .paused
        }
    }

    func confirmExplicitArrival() {
        // Explicit citizen touch confirmation (O10)
        currentState = .arrivalReported
        manager.stopUpdatingLocation()
    }

    // MARK: - CLLocationManagerDelegate

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        switch manager.authorizationStatus {
        case .authorizedWhenInUse, .authorizedAlways:
            isAuthorized = true
        default:
            isAuthorized = false
            if currentState == .tracking {
                currentState = .locationUnavailable
            }
        }
    }

    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }

        // Gate 1: Check timestamp freshness (<= 30 seconds)
        let ageSeconds = abs(location.timestamp.timeIntervalSinceNow)
        if ageSeconds > 30.0 {
            // Stale GPS reading ignored
            return
        }

        // Gate 2: Check horizontal accuracy (<= 100 meters)
        if location.horizontalAccuracy < 0 || location.horizontalAccuracy > 100.0 {
            currentState = .locationUnavailable
            return
        }

        let reading = PositionReading(
            latitude: location.coordinate.latitude,
            longitude: location.coordinate.longitude,
            accuracyMeters: location.horizontalAccuracy,
            timestampEpochMs: Int64(location.timestamp.timeIntervalSince1970 * 1000)
        )
        self.lastReading = reading

        guard let tLat = targetLat, let tLon = targetLon else { return }
        let dist = calculateHaversineDistance(
            lat1: reading.latitude,
            lon1: reading.longitude,
            lat2: tLat,
            lon2: tLon
        )
        self.distanceMeters = dist

        // State Transition: Proximity advisory at <= 75m
        if currentState != .arrivalReported {
            if dist <= 75.0 {
                // Near destination advisory: prompts user to confirm arrival explicitly
                currentState = .nearDestination
            } else {
                currentState = .tracking
            }
        }
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        if currentState == .tracking {
            currentState = .locationUnavailable
        }
    }

    private func calculateHaversineDistance(lat1: Double, lon1: Double, lat2: Double, lon2: Double) -> Double {
        let earthRadiusMeters = 6371000.0
        let dLat = (lat2 - lat1) * (.pi / 180.0)
        let dLon = (lon2 - lon1) * (.pi / 180.0)
        let a = sin(dLat / 2.0) * sin(dLat / 2.0) +
                cos(lat1 * (.pi / 180.0)) * cos(lat2 * (.pi / 180.0)) *
                sin(dLon / 2.0) * sin(dLon / 2.0)
        let c = 2.0 * atan2(sqrt(a), sqrt(1.0 - a))
        return earthRadiusMeters * c
    }
}
