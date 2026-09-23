# Sthira Mobile Architecture & Framework Foundation (Task M00)

**Governance**: PRD §4, TRD §3; Standing Engineering Rules; Decisions D03, D62; Open Decisions O02, O13  
**Architecture**: Kotlin Multiplatform (KMP) Shared Core + Platform Native UI (Jetpack Compose & SwiftUI)

---

## 1. Executive Summary & Technology Decision (O13)

Sthira mobile provides citizen emergency guidance on both **Android** and **iOS** from initial launch.

Following the comparative architectural evaluation in Decision D62, **Kotlin Multiplatform (KMP)** with native UI adapters has been selected:
- **Shared Core (`mobile/shared`)**: Encapsulates golden protocol types, range-resumable download engines, cryptographic card/manifest verification (Ed25519/SHA-256), untrusted device clock monotonic freshness guards, and offline queue state machines.
- **Android Target (`mobile/android`)**: Native Kotlin + Jetpack Compose with direct Android `AccessibilityNodeInfo` integration for TalkBack, Android Keystore encryption, and MapLibre Native Android SDK.
- **iOS Target (`mobile/ios`)**: Native Swift + SwiftUI with direct `UIAccessibilityElement` integration for VoiceOver, iOS Keychain Services encryption, and MapLibre Native iOS SDK.

### Why KMP + Native Platform UI?
1. **Low-Memory 3 GB Device Safety**: Directly compiles to native ARM64 binaries without the 80–120 MiB runtime engine overhead of Flutter/React Native. Fits comfortably within the strict 120 MiB peak RAM budget on 3 GB Android devices.
2. **First-Class Accessibility**: Emergency instructions must be flawlessly announced by TalkBack and VoiceOver. Platform-native UI avoids custom accessibility tree synthesis.
3. **Identical Cryptographic & Freshness Semantics**: Golden protocol validation, monotonic freshness, and range-resume logic are written once in Kotlin Common, eliminating cross-platform logic drift during disaster recovery.
4. **MapLibre Native Compatibility**: Direct, unbridged integration with MapLibre Native Android and iOS vector tile engines.

---

## 2. Target Platforms & Resource Budgets (O02)

| Dimension | Android Specification | iOS Specification |
| --- | --- | --- |
| **Minimum OS Version** | Android 10 (API Level 29) | iOS 16.0 |
| **Target Hardware Tier** | 3 GB RAM baseline (e.g. Redmi 9A, Galaxy A03s class) | iPhone 8 / iPhone SE (2nd Gen) / iPhone 11 |
| **Architecture** | `arm64-v8a`, `armeabi-v7a` | `arm64` |
| **Peak RAM Budget** | $\le 120\text{ MiB}$ active RSS | $\le 90\text{ MiB}$ active RSS |
| **App Binary Size** | $\le 25\text{ MiB}$ APK download | $\le 30\text{ MiB}$ IPA download |
| **Cold Startup Time** | $\le 1.5\text{ s}$ to interactive guidance | $\le 1.2\text{ s}$ to interactive guidance |
| **Regional Map Pack** | $\le 50\text{ MiB}$ compressed per district pack | $\le 50\text{ MiB}$ compressed per district pack |

---

## 3. Directory Layout

```
mobile/
├── README.md               # Architecture, build guide, and operational boundaries
├── shared/                 # Kotlin Multiplatform Shared Module
│   ├── build.gradle.kts    # KMP build definition (JVM/Android/iOS targets)
│   └── src/
│       └── commonMain/     # Shared contracts, crypto, offline, journey logic
│           └── kotlin/org/sthira/mobile/
│               ├── contracts/       # ScopedContext, DestinationChoice, ErrorTaxonomy
│               ├── freshness/       # Monotonic clock & stale-data guards
│               ├── journey/         # Foreground tracking & proximity mathematics
│               └── storage/         # Secure token storage expect/actual definitions
├── android/                # Android Application Module
│   ├── build.gradle.kts    # Android application build configuration
│   └── src/
│       └── main/
│           ├── AndroidManifest.xml  # Permissions (foreground only, no background GPS)
│           └── kotlin/org/sthira/mobile/android/
└── ios/                    # iOS Application Workspace
    ├── Configuration/
    │   └── Info.plist      # Privacy strings & accessibility declarations
    └── SthiraMobile/
```

---

## 4. Build Commands & Toolchain Requirements

### Android Build
```sh
# Requires JDK 17+ and Android SDK (Platform 35, Build-Tools 35.0.0)
cd mobile
./gradlew :android:assembleDebug
./gradlew :android:testDebugUnitTest
```

### iOS Build
```sh
# Requires Xcode 15+ with iOS 16+ SDK on macOS
cd mobile/ios
xcodebuild -scheme SthiraMobile -destination 'platform=iOS Simulator,name=iPhone SE (3rd generation)' build
xcodebuild -scheme SthiraMobile -destination 'platform=iOS Simulator,name=iPhone SE (3rd generation)' test
```

### Build Status on Host
- **macOS Build Tools**: Command Line Tools installed; full Xcode app and Java JDK are not provisioned on this environment (`BLOCKED_BUILD_TOOL / NOT_RUN`).
- **Physical Test Devices**: Physical 3 GB Android device and iPhone SE/8 require dedicated hardware bench (`BLOCKED_HARDWARE / NOT_RUN`).
