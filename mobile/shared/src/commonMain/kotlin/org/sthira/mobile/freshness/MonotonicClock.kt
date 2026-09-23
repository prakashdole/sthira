package org.sthira.mobile.freshness

/**
 * Evaluates freshness against untrusted client device clocks.
 * Rule: Device wall-clock time can be manipulated or skewed.
 * A monotonic local anchor coupled with server-signed effective/expiration windows
 * prevents replay of expired packages or stale emergency guidance.
 */
class MonotonicFreshnessGuard(
    private val clockDriftToleranceSeconds: Long = 60
) {
    enum class FreshnessState {
        FRESH,
        STALE,
        EXPIRED,
        UNVERIFIABLE
    }

    /**
     * Evaluates whether an offline package or incident alert is valid.
     */
    fun evaluateFreshness(
        effectiveEpochSeconds: Long,
        expiresEpochSeconds: Long,
        currentDeviceEpochSeconds: Long,
        lastVerifiedServerEpochSeconds: Long?
    ): FreshnessState {
        // If device has no server synchronization anchor, status is UNVERIFIABLE
        if (lastVerifiedServerEpochSeconds == null) {
            return FreshnessState.UNVERIFIABLE
        }

        // Detect gross device clock rollback
        if (currentDeviceEpochSeconds < (lastVerifiedServerEpochSeconds - clockDriftToleranceSeconds)) {
            return FreshnessState.UNVERIFIABLE
        }

        // Check if package has expired
        if (currentDeviceEpochSeconds > expiresEpochSeconds) {
            return FreshnessState.EXPIRED
        }

        // Check if package is not yet effective
        if (currentDeviceEpochSeconds < (effectiveEpochSeconds - clockDriftToleranceSeconds)) {
            return FreshnessState.UNVERIFIABLE
        }

        return FreshnessState.FRESH
    }
}
