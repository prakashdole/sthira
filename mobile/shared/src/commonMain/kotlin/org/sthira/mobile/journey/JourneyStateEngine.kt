package org.sthira.mobile.journey

import kotlin.math.*

/**
 * Foreground journey states matching R04 and M04 requirements.
 */
enum class MobileJourneyState {
    NOT_STARTED,
    TRACKING,
    NEAR_DESTINATION,
    ARRIVAL_REPORTED,
    ROUTE_REVOKED,
    LOCATION_UNAVAILABLE,
    PAUSED
}

data class GeoCoordinates(
    val longitude: Double,
    val latitude: Double,
    val accuracyMeters: Double,
    val timestampMillis: Long
)

data class DestinationTarget(
    val id: String,
    val name: String,
    val longitude: Double,
    val latitude: Double
)

data class ProximityEvaluation(
    val distanceMeters: Double,
    val isNear: Boolean,
    val isAccurate: Boolean,
    val isFresh: Boolean,
    val statusText: String
)

object JourneyEngine {
    private const val EARTH_RADIUS_METERS = 6371000.0
    private const val MAX_ACCURACY_THRESHOLD_METERS = 100.0
    private const val MAX_POSITION_AGE_MILLIS = 30_000L
    private const val NEAR_DESTINATION_RADIUS_METERS = 150.0

    fun computeDistanceMeters(
        lon1: Double,
        lat1: Double,
        lon2: Double,
        lat2: Double
    ): Double {
        val dLat = Math.toRadians(lat2 - lat1)
        val dLon = Math.toRadians(lon2 - lon1)
        val lat1Rad = Math.toRadians(lat1)
        val lat2Rad = Math.toRadians(lat2)

        val a = sin(dLat / 2).pow(2.0) +
                sin(dLon / 2).pow(2.0) * cos(lat1Rad) * cos(lat2Rad)
        val c = 2 * atan2(sqrt(a), sqrt(1 - a))
        return EARTH_RADIUS_METERS * c
    }

    fun evaluateProximity(
        position: GeoCoordinates,
        destination: DestinationTarget,
        nowMillis: Long = System.currentTimeMillis()
    ): ProximityEvaluation {
        val ageMillis = nowMillis - position.timestampMillis
        val isFresh = ageMillis in 0..MAX_POSITION_AGE_MILLIS
        val isAccurate = position.accuracyMeters in 0.0..MAX_ACCURACY_THRESHOLD_METERS

        if (!isFresh) {
            return ProximityEvaluation(
                distanceMeters = Double.MAX_VALUE,
                isNear = false,
                isAccurate = isAccurate,
                isFresh = false,
                statusText = "Position reading is stale (>30s old)"
            )
        }

        if (!isAccurate) {
            return ProximityEvaluation(
                distanceMeters = Double.MAX_VALUE,
                isNear = false,
                isAccurate = false,
                isFresh = true,
                statusText = "GPS accuracy ±${position.accuracyMeters.roundToInt()}m exceeds 100m threshold"
            )
        }

        val distance = computeDistanceMeters(
            position.longitude,
            position.latitude,
            destination.longitude,
            destination.latitude
        )

        val isNear = distance <= NEAR_DESTINATION_RADIUS_METERS

        return ProximityEvaluation(
            distanceMeters = distance,
            isNear = isNear,
            isAccurate = true,
            isFresh = true,
            statusText = if (isNear) "Near destination (±${distance.roundToInt()}m)" else "En route (${(distance / 1000.0).roundTo(1)} km)"
        )
    }

    private fun Double.roundTo(decimals: Int): Double {
        var multiplier = 1.0
        repeat(decimals) { multiplier *= 10 }
        return kotlin.math.round(this * multiplier) / multiplier
    }
}
