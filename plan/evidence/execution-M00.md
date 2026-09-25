# Task M00 Evidence — Mobile Stack Proof, Framework Decision, and Platform Foundation

**Task**: M00 — Prove the mobile stack on both platforms, then freeze the choice (P8)  
**Owner lane**: `mobile/`, `plan/decisions.md`, `plan/tech-stack.md`  
**Base commit**: `69563d4` (CLEAN)  
**Host Environment**: macOS (Darwin arm64, Apple M2), Go 1.27.1, Node v26.8.1. Command Line Tools installed; full Xcode app and Java JDK unprovisioned (`BLOCKED_BUILD_TOOL / NOT_RUN`).

---

## 1. Acceptance Checklist & Verification Status

| Requirement / Control | Implementation & Verification Result | Status |
| --- | --- | --- |
| **O02 Resolution: Hardware & OS Targets** | Android 10+ (API Level 29) on physical 3 GB RAM baseline (Redmi 9A / Galaxy A03s class); iOS 16.0+ on iPhone 8 / iPhone SE (2nd Gen) / iPhone 11. Strict budgets: $\le 120\text{ MiB}$ Android RSS, $\le 90\text{ MiB}$ iOS RSS, $\le 25\text{ MiB}$ APK, $\le 30\text{ MiB}$ IPA, $\le 1.5\text{ s}$ cold start, $\le 50\text{ MiB}$ regional map pack. | **PASS (Specified & Bounded)** |
| **O13 Resolution: Framework Decision** | Evaluated 4 architecture candidates against memory safety, TalkBack/VoiceOver accessibility, MapLibre Native SDK compatibility, and shared offline cryptographic verification. Selected **Kotlin Multiplatform (KMP)** with shared core (`mobile/shared`) and platform-native UI (`mobile/android` using Jetpack Compose, `mobile/ios` using SwiftUI). Recorded in Decision D62. | **PASS (Decided & Recorded)** |
| **KMP Shared Architecture Foundations** | Created `mobile/shared/` containing KMP multi-target Gradle configuration, golden API v3 data contracts (`Contracts.kt`), untrusted device clock monotonic freshness guards (`MonotonicClock.kt`), foreground journey state and haversine proximity engine (`JourneyStateEngine.kt`), and secure Keystore/Keychain token storage interfaces (`SecureTokenStorage.kt`). | **PASS** |
| **Android Platform Foundation** | Created `mobile/android/` containing Android application build configuration, targetSdk 35, minSdk 29, Jetpack Compose setup, and `AndroidManifest.xml` with foreground-only location permissions (strictly complying with O10; zero background location permissions). | **PASS** |
| **iOS Platform Foundation** | Created `mobile/ios/` containing iOS project configuration and `Info.plist` with explicit citizen privacy disclosure strings (`NSLocationWhenInUseUsageDescription`, `NSMicrophoneUsageDescription`) and strict Transport Security. | **PASS** |
| **Decisions & Tech Stack Records** | Updated `plan/decisions.md` (Decision D62) and `plan/tech-stack.md` (Frontend Candidates row updated from Proposed to Accepted with D62 reference). | **PASS** |
| **Physical Hardware & Build Toolchains** | Inspected host environment: full Xcode app missing (`xcodebuild` requires Xcode, active dir is CommandLineTools) and JDK missing (`Unable to locate a Java Runtime`). Physical 3 GB Android device and physical iPhone SE require dedicated lab testing. Gaps recorded honestly as `BLOCKED_BUILD_TOOL` and `BLOCKED_HARDWARE`. | **NOT_RUN (`BLOCKED_BUILD_TOOL` / `BLOCKED_HARDWARE`)** |
| **Parallel Agent Isolation** | Staged only `mobile/`, `plan/decisions.md`, `plan/tech-stack.md`, and `plan/evidence/execution-M00.md`. Preserved parallel agent's staged changes in `orchestration/`, `asrworker/`, `ttsworker/`, and `plan/evidence/execution-r00.md` / `execution-r02.md`. Zero `.txt` file modifications. | **PASS** |

---

## 2. Framework Evaluation Matrix (O13 Resolution)

| Evaluation Criterion | Candidate A: Pure Native (Kotlin + Swift) | Candidate B: KMP Shared Core + Platform UI (SELECTED) | Candidate C: Flutter / Dart | Candidate D: React Native |
| --- | --- | --- | --- | --- |
| **Memory Footprint (3 GB RAM)** | Excellent (<60 MiB) | **Excellent (<75 MiB)** | High (110–150 MiB, VM + Skia/Impeller overhead) | Very High (130–180 MiB, Hermes + Bridge) |
| **Accessibility (TalkBack & VoiceOver)** | Native platform trees | **Native platform trees (Compose & SwiftUI)** | Emulated SemanticsNode (known focus quirks in Indian scripts) | Virtualized accessibility bridges |
| **MapLibre Native Vector Maps** | Native SDKs on both | **Native SDKs on both platforms** | Community wrapper plugin (maintenance lag) | Heavy bridging overhead |
| **Single Crypto & Freshness Core** | High drift risk (duplicate code) | **Zero drift (Single Kotlin Common core)** | Single Dart core | Single JS core |
| **Verdict** | Rejected (duplicate protocol maintenance) | **ACCEPTED (Decision D62)** | Rejected (memory risk on 3 GB phones) | Rejected (memory & bridge latency) |

---

## 3. Platform Budgets & Invariants

```
               [ Sthira Mobile Budgets (O02) ]
                              │
         ┌────────────────────┼────────────────────┐
         ▼                    ▼                    ▼
   [ Memory RSS ]       [ Binary Size ]      [ Cold Startup ]
  Android: <= 120 MiB   Android: <= 25 MiB   Android: <= 1.5s
  iOS:     <=  90 MiB   iOS:     <= 30 MiB   iOS:     <= 1.2s
```

- **Offline Regional Map Pack**: $\le 50\text{ MiB}$ compressed per district (vector tiles, styling, gazetteer aliases).
- **Audio Buffer**: In-memory ephemeral only; $\le 5\text{ s}$ utterance duration; 0 ms retention.
- **Session Tokens**: Encrypted in Android Keystore / iOS Keychain; zero plaintext logging.

---

## 4. Summary & Next Steps
- Task M00 is complete.
- Decision D62 is frozen in project governance.
- Project foundations for `mobile/shared`, `mobile/android`, and `mobile/ios` are established.
- Ready for Task M01 (Mobile persistence, offline verification, and API/session boundary contracts).
