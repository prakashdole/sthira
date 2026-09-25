# Sthira v2 Release & Operational Runbook

**Document Version**: 2.0 (P9 Release-Candidate Specification)  
**Status**: APPROVED FOR INTERNAL/STAGING EXERCISE ONLY (P9 RC)  
**Target Architecture**: Go 1.27.1 Backend, PostgreSQL 18 + PostGIS 3.6, KMP Mobile Core (`mobile/shared`), Jetpack Compose (`mobile/android`), SwiftUI (`mobile/ios`), Vite/MapLibre Web (`frontend/v2`).

---

## 1. Reproducible Build Procedures

### A. Android Citizen App (`mobile/android`)
**Target Platforms**: Android 10+ (API 29+), tested against 3 GB RAM baseline (O02).  
**Prerequisites**: OpenJDK 17+, Android SDK 34, Android NDK 26+.  
*(Note per C00/C06: Gradle wrapper & root catalog setup are pending C06 native toolchain integration).*

```bash
# Set reproducible environment variables
export JAVA_HOME="/path/to/jdk-17"
export SOURCE_DATE_EPOCH=$(git log -1 --pretty=%ct)
export ANDROID_HOME="/path/to/android-sdk"

# Navigate to Android project
cd mobile/android

# Build reproducible release AAB / APK
./gradlew clean assembleRelease bundleRelease \
  --no-daemon \
  -Dorg.gradle.parallel=false \
  -Dorg.gradle.vfs.watch=false \
  -PstrictReproducible=true
```

Artifact outputs:
- Release APK: `mobile/android/build/outputs/apk/release/sthira-citizen-release-unsigned.apk`
- Release AAB: `mobile/android/build/outputs/bundle/release/sthira-citizen-release.aab`
- Budgets: APK <= 25 MiB, RSS at runtime <= 120 MiB.

### B. iOS Citizen App (`mobile/ios`)
**Target Platforms**: iOS 16.0+, tested against iPhone 8 / SE (2nd/3rd gen) / 11 baseline (O02).  
**Prerequisites**: macOS 14+, Xcode 16.0+, Swift 6.4.  
*(Note per C00/C06: Xcode project file and scheme configuration are pending C06; Swift 6.4 syntax verified via swiftc).*

```bash
# Navigate to iOS project directory
cd mobile/ios/SthiraApp

# Synthesize/parse verification (Command-line tools)
find . -name "*.swift" -exec swiftc -parse {} +

# Archive reproducible release build via xcodebuild
xcodebuild clean archive \
  -scheme SthiraApp \
  -configuration Release \
  -archivePath build/SthiraApp.xcarchive \
  CODE_SIGNING_ALLOWED=NO \
  ZERO_AR_DATE=1 \
  CLANG_ENABLE_MODULE_DEBUGGING=NO
```

Artifact output:
- Release Archive: `mobile/ios/SthiraApp/build/SthiraApp.xcarchive`
- Budgets: IPA <= 30 MiB, RSS at runtime <= 90 MiB.

### C. Backend API & Migration Service (`backend/`)
**Target Platforms**: Linux x86_64 / arm64, Distroless container base.  
**Prerequisites**: Go 1.27.1.

```bash
# Build production backend binary
cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/sthira-api ./cmd/sthira

# Build migration binary
cd ../deploy/go-backend/migrate
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o bin/sthmigrate ./cmd/sthmigrate
```

---

## 2. Dependency Inventory & BOM

| Component | Pinned Version / Runtime | License | Justification & Role |
| --- | --- | --- | --- |
| **Go Backend** | Go 1.27.1 | BSD-3-Clause | Product API host, atomic reservation engine, authority ledger |
| **PostgreSQL / PostGIS** | PostgreSQL 18.0 / PostGIS 3.6 | PostgreSQL / GPL-2.0 | Transactional persistence, spatial containment, idempotency log |
| **Kotlin Multiplatform** | Kotlin 2.1.0 | Apache-2.0 | Shared domain, offline package validator, durable queue (`mobile/shared`) |
| **Jetpack Compose** | Compose BOM 2024.12.01 | Apache-2.0 | Native Android UI, accessible talkback, dynamic font scaling |
| **SwiftUI** | Swift 6.4 / iOS 16.0+ SDK | Apple EULA / Apache-2.0 | Native iOS UI, VoiceOver compatibility, device-only Keychain |
| **MapLibre GL JS** | 5.1.0 | BSD-3-Clause | Web vector map renderer (zero proprietary analytics) |
| **MapLibre Native** | Android 11.5.0 / iOS 6.4.0 | BSD-3-Clause | Mobile offline vector tile renderer (`<=50 MiB` pack budget) |
| **Sarvam-30B MoE** | sarvamai/sarvam-30b (FP8) | Apache-2.0 | Middle reasoning model, constrained JSON action extraction (D59) |
| **IndicConformer** | AI4Bharat 600M-Multilingual | MIT | Indic speech recognition (16 kHz, streaming WAV) |
| **Indic Parler-TTS** | AI4Bharat Indic Parler-TTS | Apache-2.0 | Indic speech synthesis (pre-approved templates only) |

---

## 3. Private Signing & Key Management

### Android Release Signing (Local / Internal Scope Only)
1. **Keystore Generation** (isolated offline host):
   ```bash
   keytool -genkeypair -v \
     -keystore sthira-release.keystore \
     -alias sthira-citizen \
     -keyalg RSA -keysize 4096 \
     -validity 10000 \
     -storetype PKCS12
   ```
2. **Environment Ingestion**:
   Signing parameters MUST be injected via environment variables in CI/CD runners; NEVER commit credentials or keystores to Git:
   - `STHIRA_ANDROID_KEYSTORE_PATH`
   - `STHIRA_ANDROID_KEYSTORE_PASSWORD`
   - `STHIRA_ANDROID_KEY_ALIAS`
   - `STHIRA_ANDROID_KEY_PASSWORD`
3. **APK Signing**:
   ```bash
   apksigner sign --ks "$STHIRA_ANDROID_KEYSTORE_PATH" \
     --ks-key-alias "$STHIRA_ANDROID_KEY_ALIAS" \
     --out sthira-citizen-signed.apk \
     mobile/android/build/outputs/apk/release/sthira-citizen-release-unsigned.apk
   ```

### iOS Release Signing (Internal Ad-Hoc / TestFlight)
1. Signing requires authorized Apple Developer Team ID and distribution certificates (O16).
2. Export archive with managed profile:
   ```bash
   xcodebuild -exportArchive \
     -archivePath build/SthiraApp.xcarchive \
     -exportPath build/ReleaseIPA \
     -exportOptionsPlist ExportOptions.plist
   ```
3. Never bypass Gate S: Signed builds are restricted to internal staging/drills.

---

## 4. Privacy, Security & Retention Disclosures

### A. Raw Audio Stream (Zero-Retention Policy)
- Audio capture is ephemeral: sampled at 16,000 Hz mono (16-bit PCM WAV) in memory only.
- Raw audio buffers have **0 ms disk retention**; buffers are held in memory during the streaming request and zeroed immediately after decoding.
- Audio is never written to disk, caches, temp directories, or logging facilities.

### B. Citizen Geolocation (Foreground Consent Only)
- **Zero Background Tracking**: Android manifest explicitly omits `ACCESS_BACKGROUND_LOCATION`. iOS `Info.plist` omits background location capability.
- Location tracking operates exclusively while the citizen has an active journey screen open in the foreground.
- Minimum accuracy gating (<= 100m) and freshness gating (<= 30s) discard noisy and stale signals.
- **Proximity Advisory**: Proximity to destination (<= 75m) sets `NEAR_DESTINATION` status for UI guidance only; arrival requires **explicit manual touch confirmation**. Geofencing auto-arrival is strictly prohibited (D16, O10).

### C. Capability Tokens & Session Management
- Citizen sessions are account-free capability tokens generated via `crypto/rand` (32 bytes).
- The server stores only the cryptographic SHA-256 digest of the token; plaintext tokens are never logged or stored in databases.
- Device storage isolates tokens:
  - iOS: Device-only Keychain with `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly` (excluded from iCloud backups).
  - Android: `EncryptedSharedPreferences` backed by Android Keystore (`android:allowBackup="false"` prevents ADB extraction).

### D. Push Notifications Privacy (O12)
- Push payloads must NEVER contain coordinates, street addresses, or polyline geometries.
- Payloads carry only incident identifiers, safe zone names, and event timestamps.
- When an alert arrives, the client fetches and validates operational data directly from `/api/v3`.

---

## 5. Support, On-Call & Incident Escalation SOP

### Severity Matrix

| Severity | Definition | Target Response | Target Resolution | Escalation Contact |
| --- | --- | --- | --- | --- |
| **SEV-1 (Critical)** | Capacity overbooking, unauthorized route guidance, audit chain breach, or data corruption | < 15 min | < 2 hours | Operations Lead + Security Lead + Shelter Authority |
| **SEV-2 (Major)** | Backend dependency outage (DB/Model), 503 fail-closed active, or ingestion delay > 15 min | < 30 min | < 4 hours | Backend On-Call Engineer |
| **SEV-3 (Minor)** | Individual client sync retry failure, non-critical localized map asset fetch failure | < 2 hours | < 24 hours | Mobile Support Team |

### Escalation Protocol
1. **SEV-1 Activation**:
   - Immediately switch active backend into read-only degraded mode or issue HTTP 503 fail-closed if authority invariants are breached.
   - Quarantine suspect source feeds via `/api/v3/operator/sources/{id}/quarantine` using two-operator verification (O14).
   - Invalidate downstream client caches by incrementing `source_version`.
2. **Emergency Contacts**:
   - Backend On-Call: `oncall-backend@internal.sthira.org`
   - Operations Center: `ops-lead@internal.sthira.org`
   - Disaster Management Authority Liaison: `disaster-liaison@internal.sthira.org`

---

## 6. Operational Rollback Runbook

### Principle of Safe Zero-Data-Loss Rollback
- Database migrations (`0001` through `0010`) are backward compatible. New columns (`source_id`, `template_sha256`, `operator_subject`) are additive.
- **NEVER roll back database migrations destructively** in production: rolling back migrations drops tables and destroys citizen stay records.
- **Binary Rollback**: In the event of a regression in backend code, roll back the container binary to the previous verified release candidate SHA. The previous binary will operate cleanly against the migrated database.

### Capacity & Reservation Rollback Rules
- If an erroneous batch of reservations occurred due to bad input data, **do not execute hard SQL deletes** on reservations.
- Operator corrections must be **downward-only** (D27, D30) via `POST /api/v3/operator/stays/{id}/correction`. Downward corrections release space safely back into inventory while preserving audit trails.
- Any emergency capacity reallocation must be executed as explicit atomic idempotent transactions.

### Rollback Execution Steps
```bash
# 1. Drain traffic on faulty container revision
docker-compose -f deploy/docker-compose.demo.yml stop api

# 2. Revert to prior certified release image SHA
export STHIRA_API_IMAGE="sthira/api:rc-verified-prev"

# 3. Launch certified image
docker-compose -f deploy/docker-compose.demo.yml up -d api

# 4. Verify health and readiness probe
curl -f http://127.0.0.1:8080/readyz || echo "Readiness probe failed!"
```
