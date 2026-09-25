package org.sthira.mobile.offlinepkg

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/**
 * P5 offline package and card types matching backend/internal/offlinepkg/types.go.
 * All types are strictly public: zero citizen tokens, coordinates, or civilian PII.
 */

enum class FreshnessState {
    CURRENT,
    STALE,
    EXPIRED,
    REVOKED,
    UNVERIFIABLE
}

enum class ResourceFreshnessState {
    CURRENT,
    STALE,
    MISSING
}

enum class ResourceLicenseStatus {
    LICENSE_PENDING,
    LICENSE_ALLOWED,
    LICENSE_DENIED
}

enum class ResourceType {
    VECTOR_TILES,
    MAP_STYLE,
    MAP_SPRITE,
    MAP_GLYPHS,
    GAZETTEER,
    EMERGENCY_AUDIO
}

@Serializable
data class Signature(
    @SerialName("algorithm") val algorithm: String,
    @SerialName("key_id") val keyId: String,
    @SerialName("value") val value: String // Base64 RFC 4648
)

@Serializable
data class CriticalCardDescriptor(
    @SerialName("package_id") val packageId: String,
    @SerialName("version") val version: Int,
    @SerialName("uri") val uri: String,
    @SerialName("checksum_sha256") val checksumSha256: String,
    @SerialName("uncompressed_bytes") val uncompressedBytes: Long,
    @SerialName("compressed_bytes") val compressedBytes: Long,
    @SerialName("content_type") val contentType: String
)

@Serializable
data class ResourceDescriptor(
    @SerialName("resource_id") val resourceId: String,
    @SerialName("type") val type: ResourceType,
    @SerialName("uri") val uri: String,
    @SerialName("checksum_sha256") val checksumSha256: String,
    @SerialName("byte_size") val byteSize: Long,
    @SerialName("content_type") val contentType: String,
    @SerialName("required") val required: Boolean,
    @SerialName("attribution") val attribution: String
)

@Serializable
data class RevocationBlock(
    @SerialName("revoked_packages") val revokedPackages: List<String> = emptyList(),
    @SerialName("cancelled_routes") val cancelledRoutes: List<String> = emptyList(),
    @SerialName("superseded_versions") val supersededVersions: List<SupersededVersion> = emptyList()
)

@Serializable
data class SupersededVersion(
    @SerialName("package_id") val packageId: String,
    @SerialName("version") val version: Int
)

@Serializable
data class ManifestProvenance(
    @SerialName("authority") val authority: String,
    @SerialName("dataset_id") val datasetId: String,
    @SerialName("evidence_class") val evidenceClass: String
)

@Serializable
data class Manifest(
    @SerialName("schema_version") val schemaVersion: String,
    @SerialName("manifest_id") val manifestId: String,
    @SerialName("jurisdiction") val jurisdiction: String,
    @SerialName("revision") val revision: Int,
    @SerialName("generated_at") val generatedAt: String,
    @SerialName("valid_until") val validUntil: String,
    @SerialName("source_status") val sourceStatus: String,
    @SerialName("critical_card") val criticalCard: CriticalCardDescriptor,
    @SerialName("resources") val resources: List<ResourceDescriptor> = emptyList(),
    @SerialName("revocations") val revocations: RevocationBlock,
    @SerialName("provenance") val provenance: ManifestProvenance,
    @SerialName("checksum_sha256") val checksumSha256: String,
    @SerialName("signature") val signature: Signature? = null
)

@Serializable
data class PublicIncidentCard(
    @SerialName("schema_version") val schemaVersion: String,
    @SerialName("package_id") val packageId: String,
    @SerialName("version") val version: Int,
    @SerialName("jurisdiction") val jurisdiction: String,
    @SerialName("evidence_class") val evidenceClass: String,
    @SerialName("effective_at") val effectiveAt: String,
    @SerialName("expires_at") val expiresAt: String,
    @SerialName("alert") val alert: AlertCard,
    @SerialName("red_zones") val redZones: List<RedZoneCard> = emptyList(),
    @SerialName("safe_zones") val safeZones: List<SafeZoneCard> = emptyList(),
    @SerialName("approved_routes") val approvedRoutes: List<RouteCard> = emptyList(),
    @SerialName("facilities") val facilities: List<FacilityCard> = emptyList(),
    @SerialName("instructions") val instructions: List<InstructionCard> = emptyList(),
    @SerialName("emergency_contacts") val emergencyContacts: List<EmergencyContact> = emptyList(),
    @SerialName("allocation_policy") val allocationPolicy: PolicyCard,
    @SerialName("checksum_sha256") val checksumSha256: String,
    @SerialName("signature") val signature: Signature? = null
)

@Serializable
data class AlertCard(
    @SerialName("identifier") val identifier: String,
    @SerialName("sender") val sender: String,
    @SerialName("headline") val headline: String,
    @SerialName("severity") val severity: String,
    @SerialName("urgency") val urgency: String,
    @SerialName("certainty") val certainty: String,
    @SerialName("area_description") val areaDescription: String
)

@Serializable
data class RedZoneCard(
    @SerialName("id") val id: String,
    @SerialName("name") val name: String? = null,
    @SerialName("centroid") val centroid: List<Double>? = null
)

@Serializable
data class SafeZoneCard(
    @SerialName("id") val id: String,
    @SerialName("name") val name: String,
    @SerialName("role") val role: String,
    @SerialName("status") val status: String,
    @SerialName("capacity_mode") val capacityMode: String,
    @SerialName("total_capacity") val totalCapacity: Int? = null,
    @SerialName("location") val location: List<Double>? = null,
    @SerialName("services") val services: List<String> = emptyList()
)

@Serializable
data class RouteCard(
    @SerialName("id") val id: String,
    @SerialName("from_zone_id") val fromZoneId: String,
    @SerialName("to_safe_zone_id") val toSafeZoneId: String,
    @SerialName("mode") val mode: String,
    @SerialName("approval") val approval: String,
    @SerialName("verified_by") val verifiedBy: String? = null,
    @SerialName("verified_at") val verifiedAt: String? = null,
    @SerialName("valid_from") val validFrom: String? = null,
    @SerialName("valid_until") val validUntil: String? = null
)

@Serializable
data class FacilityCard(
    @SerialName("id") val id: String,
    @SerialName("safe_zone_id") val safeZoneId: String,
    @SerialName("name") val name: String,
    @SerialName("address") val address: String? = null,
    @SerialName("contact_phone") val contactPhone: String? = null
)

@Serializable
data class InstructionCard(
    @SerialName("id") val id: String,
    @SerialName("order") val order: Int,
    @SerialName("phase") val phase: String,
    @SerialName("text") val text: String,
    @SerialName("language") val language: String = "en"
)

@Serializable
data class EmergencyContact(
    @SerialName("label") val label: String,
    @SerialName("phone_number") val phoneNumber: String
)

@Serializable
data class PolicyCard(
    @SerialName("order") val order: List<String> = emptyList(),
    @SerialName("route_required") val routeRequired: Boolean = true,
    @SerialName("hold_duration_minutes") val holdDurationMinutes: Int = 120
)
