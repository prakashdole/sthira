package org.sthira.mobile.offlinepkg

/**
 * Validates public incident cards and manifests against P5 contract rules.
 */
object CardValidator {

    sealed class ValidationResult {
        object Valid : ValidationResult()
        data class Invalid(val reason: String) : ValidationResult()
    }

    /**
     * Validates structural invariants of a PublicIncidentCard.
     */
    fun validateCard(card: PublicIncidentCard, manifestRevocations: RevocationBlock? = null): ValidationResult {
        if (card.schemaVersion != "3.0") {
            return ValidationResult.Invalid("Unsupported schema_version '${card.schemaVersion}'; expected '3.0'")
        }
        if (card.packageId.isBlank()) {
            return ValidationResult.Invalid("Package ID cannot be blank")
        }
        if (card.version <= 0) {
            return ValidationResult.Invalid("Package version must be a positive integer")
        }
        if (card.jurisdiction.isBlank()) {
            return ValidationResult.Invalid("Jurisdiction cannot be blank")
        }

        // Check if package is revoked by manifest
        if (manifestRevocations != null) {
            if (manifestRevocations.revokedPackages.contains(card.packageId)) {
                return ValidationResult.Invalid("Package '${card.packageId}' is explicitly revoked by active manifest")
            }
            val isSuperseded = manifestRevocations.supersededVersions.any {
                it.packageId == card.packageId && it.version >= card.version
            }
            if (isSuperseded) {
                return ValidationResult.Invalid("Package '${card.packageId}' v${card.version} has been superseded")
            }
        }

        // Check red zones geometry
        if (card.redZones.isEmpty()) {
            return ValidationResult.Invalid("Card must contain at least one red zone hazard boundary")
        }

        // Check safe zones
        if (card.safeZones.isEmpty()) {
            return ValidationResult.Invalid("Card must contain at least one safe zone destination")
        }

        // Check policy safe zone ordering references
        val safeZoneIds = card.safeZones.map { it.id }.toSet()
        for (orderedId in card.allocationPolicy.order) {
            if (!safeZoneIds.contains(orderedId)) {
                return ValidationResult.Invalid("Allocation policy orders unknown safe zone '$orderedId'")
            }
        }

        // Check facilities reference valid safe zones
        for (facility in card.facilities) {
            if (!safeZoneIds.contains(facility.safeZoneId)) {
                return ValidationResult.Invalid("Facility '${facility.id}' references unknown safe zone '${facility.safeZoneId}'")
            }
        }

        // Check routes connect valid safe zones and check cancelled routes
        for (route in card.approvedRoutes) {
            if (manifestRevocations != null && manifestRevocations.cancelledRoutes.contains(route.id)) {
                return ValidationResult.Invalid("Approved route '${route.id}' is listed as cancelled in manifest")
            }
            if (!safeZoneIds.contains(route.toSafeZoneId)) {
                return ValidationResult.Invalid("Route '${route.id}' leads to unknown safe zone '${route.toSafeZoneId}'")
            }
        }

        return ValidationResult.Valid
    }
}
