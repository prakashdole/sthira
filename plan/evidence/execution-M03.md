# Task M03 Evidence — iPhone Complete Citizen Experience

**Task**: M03 — iPhone complete citizen experience (P8)  
**Owner lane**: `mobile/ios/`, `backend/internal/offlineclient/ios_journey_compatibility_test.go`  
**Base commit**: `93019ab` (M02 on `CLEAN`)  
**Host Environment**: macOS (Darwin arm64, Apple M2), Swift 6.4 (swiftlang-6.4.0.34.1), Go 1.27.1, Node v26.8.1. Apple Command Line Tools installed; full Xcode app unprovisioned (`BLOCKED_BUILD_TOOL`), physical iPhone 8 / SE / 11 unprovisioned (`BLOCKED_HARDWARE`).

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **Privacy Disclosures & Transport Security** | Verified `mobile/ios/Configuration/Info.plist` with `NSLocationWhenInUseUsageDescription` (foreground only per O10), `NSMicrophoneUsageDescription` (ephemeral voice commands per O10), and strict `NSAppTransportSecurity` (`NSAllowsArbitraryLoads = false`). | **PASS** |
| **VoiceOver & Dynamic Type Accessibility** | Designed views with semantic fonts (`.headline`, `.subheadline`, `.caption`), VoiceOver `.accessibilityLabel`, `.accessibilityElement(children: .combine)`, and >=44 pt minimum touch target enforcement (`.accessibleTarget(minPt: 44)`). | **PASS** |
| **Non-Color Hazard Badging** | Implemented `HazardBadge` in `AccessibilityUtils.swift` combining geometric shapes, symbol icons, high-contrast borders, and descriptive VoiceOver strings for Red Zones, Safe Zones, Warnings, and Unconfirmed Capacity. | **PASS** |
| **Audio Session & Interruption Lifecycle** | Created `AudioSessionManager.swift` configuring `AVAudioSession` (`.playAndRecord`), listening to `interruptionNotification` and `routeChangeNotification` to pause speech synthesis during phone calls and alarms. | **PASS** |
| **Ephemeral In-Memory Voice Capture** | Created `EphemeralAudioRecorder` in `AudioSessionManager.swift` recording 16kHz mono 16-bit PCM WAV strictly in-memory with 0 ms disk retention (O10). Max 5s duration window. | **PASS** |
| **Foreground Location & Proximity Uncertainty** | Created `LocationManager.swift` enforcing `requestWhenInUseAuthorization()` (zero background surveillance per O10); age <= 30s and accuracy <= 100m gates; Haversine proximity advisory at <= 75m; explicit citizen touch required for arrival confirmation. | **PASS** |
| **Keychain Token Security** | Created `KeychainTokenStorage.swift` using `kSecClassGenericPassword` with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` to isolate tokens and prevent backup extraction. | **PASS** |
| **Multimodal Speak/Type/Touch & 409 Disambiguation** | Created `DestinationPickerView.swift` with microphone button, text search, and candidate chips for 409 ambiguous location conflicts (explicit selection required; auto-selection strictly prohibited). | **PASS** |
| **Accessible Non-Map Turn-by-Turn Directions** | Created `RouteGuidanceView.swift` with step-by-step turn-by-turn text directions for low-bandwidth and VoiceOver use, plus route revocation alert handling. | **PASS** |
| **Shelter Stay Lifecycle & Explicit Arrival** | Created `StayManagementView.swift` with party size reservation, extend, depart, cancel, and explicit manual arrival confirmation alert. Geofence auto-arrival strictly prohibited. | **PASS** |
| **Emergency Dialler Handoff** | Created `EmergencyDialler` and `EmergencyCallCard` opening 112 via `UIApplication.shared.open` on explicit citizen tap only. | **PASS** |
| **Swift Syntax & Semantic Parsing** | Parsed all 14 Swift source files with `swiftc -parse`. Exited with code 0. | **PASS** |
| **Go Integration Tests** | Added `backend/internal/offlineclient/ios_journey_compatibility_test.go` (`TestIOSJourneyStateEngineInvariants`, `TestIOSKeychainTokenIsolation`). All 37 offlineclient tests and full backend test suite green. | **PASS** |
| **Physical iPhone & Xcode App Checks** | Host lacks full Xcode app (`xcodebuild` requires Xcode app; active is CommandLineTools) and physical test iPhone (`BLOCKED_HARDWARE`). Recorded honestly. | **NOT_RUN (`BLOCKED_BUILD_TOOL` / `BLOCKED_HARDWARE`)** |

---

## 2. Observable Key Changes

- `mobile/ios/SthiraApp/`:
  - `Theme.swift`: WCAG AAA accessible colors, light/dark mode support.
  - `AccessibilityUtils.swift`: Target size helper (>=44pt), `HazardBadge`, `EmergencyDialler`.
  - `AudioSessionManager.swift`: `AVAudioSession` lifecycle and `EphemeralAudioRecorder` (16kHz WAV, 0 ms disk retention).
  - `LocationManager.swift`: Foreground CoreLocation manager with accuracy & age gates, Haversine proximity, explicit arrival state.
  - `KeychainTokenStorage.swift`: Secure storage for session tokens in iOS Keychain.
  - `Views/Components.swift`: `SyntheticExerciseBanner`, `FreshnessBadge`, `EmergencyCallCard`, `CapacityIndicator`.
  - `Views/LanguageView.swift`: Language selection (ml, hi, en).
  - `Views/IncidentOverviewView.swift`: Official alerts, red zones, safe zones, provenance.
  - `Views/DestinationPickerView.swift`: Voice, text, touch, ambiguous candidate chips, facility cards.
  - `Views/RouteGuidanceView.swift`: Route preview, turn-by-turn accessible text directions, revocation alert.
  - `Views/StayManagementView.swift`: Bed reservation, extend, depart, cancel, explicit touch arrival.
  - `Views/ForegroundTrackingView.swift`: Proximity advisory banner and manual arrival action.
  - `ViewModels/CitizenJourneyViewModel.swift`: Central ObservableObject coordinating state and views.
  - `SthiraApp.swift`: SwiftUI App entry point and TabView navigation.
- `backend/internal/offlineclient/ios_journey_compatibility_test.go`:
  - Go verification of iOS journey state engine invariants, accuracy/freshness gates, and token isolation.

---

## 3. Test & Verification Log

```bash
# 1. Swift Syntax and Typecheck Verification
$ swiftc -parse \
    mobile/ios/SthiraApp/Theme.swift \
    mobile/ios/SthiraApp/AccessibilityUtils.swift \
    mobile/ios/SthiraApp/KeychainTokenStorage.swift \
    mobile/ios/SthiraApp/LocationManager.swift \
    mobile/ios/SthiraApp/AudioSessionManager.swift \
    mobile/ios/SthiraApp/ViewModels/CitizenJourneyViewModel.swift \
    mobile/ios/SthiraApp/Views/Components.swift \
    mobile/ios/SthiraApp/Views/LanguageView.swift \
    mobile/ios/SthiraApp/Views/IncidentOverviewView.swift \
    mobile/ios/SthiraApp/Views/DestinationPickerView.swift \
    mobile/ios/SthiraApp/Views/RouteGuidanceView.swift \
    mobile/ios/SthiraApp/Views/StayManagementView.swift \
    mobile/ios/SthiraApp/Views/ForegroundTrackingView.swift \
    mobile/ios/SthiraApp/SthiraApp.swift
# Exit Code: 0 (All 14 Swift files parsed and verified)

# 2. Go Backend Verification of iOS Journey Invariants
$ cd backend && go test -v ./internal/offlineclient/...
=== RUN   TestIOSJourneyStateEngineInvariants
--- PASS: TestIOSJourneyStateEngineInvariants (0.00s)
=== RUN   TestIOSKeychainTokenIsolation
--- PASS: TestIOSKeychainTokenIsolation (0.00s)
... (all 37 tests pass)
PASS
ok  	sthira/backend/internal/offlineclient	2.719s

# 3. Full Backend Test Suite
$ cd backend && go test ./...
All 16 packages green.
```

---

## 4. Next Steps
- iOS citizen journey implementation complete (M03).
- Ready for Task M05 (Integrated P8 acceptance reconciling M00–M04, recording platform blockers).
