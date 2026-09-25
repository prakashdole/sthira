package org.sthira.mobile.contracts

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * Standard Sthira v2 API envelope matching plan/trd.md line 26.
 */
@Serializable
data class ApiResponse<T>(
    @SerialName("data") val data: T? = null,
    @SerialName("error") val error: ApiError? = null,
    @SerialName("freshness") val freshness: String,
    @SerialName("request_id") val requestId: String,
    @SerialName("signature") val signature: String? = null
)

@Serializable
data class ApiError(
    @SerialName("code") val code: String,
    @SerialName("message") val message: String,
    @SerialName("details") val details: String? = null
)

/**
 * Authoritative destination choice matching /api/v3/guidance.
 */
@Serializable
data class DestinationChoice(
    @SerialName("facility_id") val facilityId: String,
    @SerialName("safe_zone_id") val safeZoneId: String,
    @SerialName("facility_name") val facilityName: String,
    @SerialName("capacity_known") val capacityKnown: Boolean,
    @SerialName("free") val freeCapacity: Int? = null,
    @SerialName("route_id") val routeId: String? = null,
    @SerialName("route_verified") val routeVerified: Boolean = false,
    @SerialName("distance_km") val distanceKm: Double? = null,
    @SerialName("duration_minutes") val durationMinutes: Int? = null
)

/**
 * Citizen session capability token matching Decision D24.
 */
@Serializable
data class CitizenSession(
    @SerialName("session_id") val sessionId: String,
    @SerialName("token") val token: String,
    @SerialName("principal") val principal: String = "CITIZEN",
    @SerialName("jurisdiction") val jurisdiction: String,
    @SerialName("expires_in") val expiresInSeconds: Int
)

/**
 * Stay allocation commitment matching /api/v3/stays.
 */
@Serializable
data class StayAllocation(
    @SerialName("stay_id") val stayId: String,
    @SerialName("reservation_id") val reservationId: String,
    @SerialName("facility_id") val facilityId: String,
    @SerialName("party_size") val partySize: Int,
    @SerialName("status") val status: String, // HELD | OCCUPIED | DEPARTED | CANCELLED
    @SerialName("hold_expires_at") val holdExpiresAt: String? = null,
    @SerialName("snapshot_version") val snapshotVersion: Int
)

/**
 * Offline package descriptor matching P5 offline protocol.
 */
@Serializable
data class OfflinePackageMetadata(
    @SerialName("package_id") val packageId: String,
    @SerialName("version") val version: Int,
    @SerialName("jurisdiction") val jurisdiction: String,
    @SerialName("issued_at") val issuedAt: String,
    @SerialName("expires_at") val expiresAt: String,
    @SerialName("content_sha256") val contentSha256: String,
    @SerialName("size_bytes") val sizeBytes: Long
)
