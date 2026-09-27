/**
 * Consent-based foreground journey tracking and arrival verification logic (R04).
 *
 * Implements deterministic proximity checks, sensor uncertainty handling,
 * and strict consent-based explicit arrival state transitions.
 * Pure logic module with zero DOM dependencies for complete testability.
 */

export type JourneyState =
  | 'NOT_STARTED'
  | 'TRACKING'
  | 'NEAR_DESTINATION'
  | 'ARRIVAL_REPORTED'
  | 'PAUSED'
  | 'LOCATION_UNAVAILABLE'
  | 'ROUTE_REVOKED';

export interface PositionReading {
  longitude: number;
  latitude: number;
  accuracyMeters: number;
  timestamp: number; // Epoch ms
}

export interface DestinationTarget {
  id: string;
  name: string;
  longitude?: number;
  latitude?: number;
}

export interface JourneyOptions {
  proximityThresholdMeters: number; // Maximum distance to consider citizen "near" destination
  maxAcceptableAccuracyMeters: number; // Maximum GPS accuracy radius before rejecting assertion
  maxPositionAgeMs: number; // Maximum age in milliseconds before position is considered stale
}

export const DEFAULT_JOURNEY_OPTIONS: JourneyOptions = {
  proximityThresholdMeters: 150, // Conservative exercise threshold (150 m)
  maxAcceptableAccuracyMeters: 100, // Reject assertions if uncertainty > 100 m
  maxPositionAgeMs: 30000, // 30 seconds freshness window
};

export interface ProximityEvaluation {
  distanceMeters: number;
  isNear: boolean;
  isStale: boolean;
  isAccurateEnough: boolean;
  reason: 'VERIFIED_NEAR' | 'OUTSIDE_RANGE' | 'LOW_ACCURACY' | 'STALE' | 'INVALID_COORDINATES' | 'UNAVAILABLE_COORDINATES';
}

/**
 * Calculates great-circle distance between two WGS84 coordinates in meters using the Haversine formula.
 */
export function computeDistanceMeters(lon1: number, lat1: number, lon2: number, lat2: number): number {
  if (
    !Number.isFinite(lon1) || !Number.isFinite(lat1) ||
    !Number.isFinite(lon2) || !Number.isFinite(lat2) ||
    lat1 < -90 || lat1 > 90 || lat2 < -90 || lat2 > 90 ||
    lon1 < -180 || lon1 > 180 || lon2 < -180 || lon2 > 180
  ) {
    return Number.NaN;
  }

  const R = 6371000; // Earth mean radius in meters
  const toRad = Math.PI / 180;
  const dLat = (lat2 - lat1) * toRad;
  const dLon = (lon2 - lon1) * toRad;

  const a =
    Math.sin(dLat / 2) * Math.sin(dLat / 2) +
    Math.cos(lat1 * toRad) * Math.cos(lat2 * toRad) *
    Math.sin(dLon / 2) * Math.sin(dLon / 2);

  const c = 2 * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
  return R * c;
}

/**
 * Evaluates citizen proximity to destination taking into account sensor uncertainty and staleness.
 * Missing or undefined destination coordinates produce UNAVAILABLE_COORDINATES (unavailable proximity).
 */
export function evaluateProximity(
  pos: PositionReading | null | undefined,
  dest: DestinationTarget | null | undefined,
  options: JourneyOptions = DEFAULT_JOURNEY_OPTIONS,
  currentTimeMs: number = Date.now()
): ProximityEvaluation {
  if (
    !dest ||
    typeof dest.longitude !== 'number' ||
    typeof dest.latitude !== 'number' ||
    !Number.isFinite(dest.longitude) ||
    !Number.isFinite(dest.latitude)
  ) {
    return {
      distanceMeters: Number.NaN,
      isNear: false,
      isStale: false,
      isAccurateEnough: false,
      reason: 'UNAVAILABLE_COORDINATES',
    };
  }

  if (
    !pos ||
    typeof pos.longitude !== 'number' ||
    typeof pos.latitude !== 'number' ||
    !Number.isFinite(pos.longitude) ||
    !Number.isFinite(pos.latitude)
  ) {
    return {
      distanceMeters: Number.NaN,
      isNear: false,
      isStale: false,
      isAccurateEnough: false,
      reason: 'INVALID_COORDINATES',
    };
  }

  const distance = computeDistanceMeters(pos.longitude, pos.latitude, dest.longitude, dest.latitude);

  if (Number.isNaN(distance)) {
    return {
      distanceMeters: Number.NaN,
      isNear: false,
      isStale: false,
      isAccurateEnough: false,
      reason: 'INVALID_COORDINATES',
    };
  }

  const ageMs = currentTimeMs - pos.timestamp;
  const isStale = ageMs > options.maxPositionAgeMs;
  const isAccurateEnough = Number.isFinite(pos.accuracyMeters) && pos.accuracyMeters > 0 && pos.accuracyMeters <= options.maxAcceptableAccuracyMeters;

  if (isStale) {
    return {
      distanceMeters: Math.round(distance),
      isNear: false,
      isStale: true,
      isAccurateEnough,
      reason: 'STALE',
    };
  }

  if (!isAccurateEnough) {
    return {
      distanceMeters: Math.round(distance),
      isNear: false,
      isStale: false,
      isAccurateEnough: false,
      reason: 'LOW_ACCURACY',
    };
  }

  // Account for position uncertainty region straddling the threshold:
  // If citizen distance is <= proximity threshold, verify proximity
  const isNear = distance <= options.proximityThresholdMeters;

  return {
    distanceMeters: Math.round(distance),
    isNear,
    isStale: false,
    isAccurateEnough: true,
    reason: isNear ? 'VERIFIED_NEAR' : 'OUTSIDE_RANGE',
  };
}

/**
 * Deterministically computes next journey state on receiving a position update.
 */
export function transitionOnPosition(
  currentState: JourneyState,
  evaluation: ProximityEvaluation
): JourneyState {
  // Terminal or inactive states never transition automatically from position updates
  if (
    currentState === 'NOT_STARTED' ||
    currentState === 'PAUSED' ||
    currentState === 'ARRIVAL_REPORTED' ||
    currentState === 'ROUTE_REVOKED'
  ) {
    return currentState;
  }

  if (currentState === 'LOCATION_UNAVAILABLE') {
    // If we recovered a valid reading
    if (
      evaluation.reason !== 'INVALID_COORDINATES' &&
      evaluation.reason !== 'UNAVAILABLE_COORDINATES' &&
      !evaluation.isStale
    ) {
      return evaluation.isNear ? 'NEAR_DESTINATION' : 'TRACKING';
    }
    return currentState;
  }

  if (currentState === 'TRACKING') {
    if (evaluation.isNear) {
      return 'NEAR_DESTINATION';
    }
    return 'TRACKING';
  }

  if (currentState === 'NEAR_DESTINATION') {
    // If citizen moved away, or coordinates became unavailable, transition back to tracking
    if (!evaluation.isNear) {
      if (evaluation.isAccurateEnough && !evaluation.isStale) {
        return 'TRACKING';
      }
      if (
        evaluation.reason === 'UNAVAILABLE_COORDINATES' ||
        evaluation.reason === 'INVALID_COORDINATES'
      ) {
        return 'TRACKING';
      }
    }
    return 'NEAR_DESTINATION';
  }

  return currentState;
}

/**
 * Handles explicit arrival confirmation action by citizen.
 * Strictly guarantees idempotency: duplicate confirmation attempts do not re-transition.
 */
export function transitionOnArrival(
  currentState: JourneyState
): { nextState: JourneyState; isNewTransition: boolean; error?: string } {
  if (currentState === 'ARRIVAL_REPORTED') {
    return {
      nextState: 'ARRIVAL_REPORTED',
      isNewTransition: false,
      error: 'ALREADY_REPORTED',
    };
  }

  if (currentState === 'ROUTE_REVOKED') {
    return {
      nextState: 'ROUTE_REVOKED',
      isNewTransition: false,
      error: 'ROUTE_REVOKED',
    };
  }

  return {
    nextState: 'ARRIVAL_REPORTED',
    isNewTransition: true,
  };
}

/**
 * Handles emergency evacuation route revocation.
 */
export function transitionOnRevocation(currentState: JourneyState): JourneyState {
  if (currentState === 'ARRIVAL_REPORTED') {
    // Citizen already arrived; revocation applies to en route citizens
    return currentState;
  }
  return 'ROUTE_REVOKED';
}

export interface ArrivalVerificationRequest {
  currentState: JourneyState;
  activeStayId: string | null | undefined;
  serverResponse?: {
    ok: boolean;
    status: number;
    error?: string;
  };
}

export interface ArrivalVerificationResult {
  allowed: boolean;
  nextState: JourneyState;
  isNewTransition: boolean;
  arrivalSuccess: boolean;
  errorMessage?: string;
  canRetry: boolean;
}

/**
 * Validates arrival confirmation strictly according to fail-closed R04 rules:
 * - Requires an active, verified stay reservation (no stay -> fail closed, no success claim)
 * - Server rejection or network/timeout failure never invents success
 * - Requires explicit server acknowledgement (res.ok) to transition to ARRIVAL_REPORTED
 * - Idempotent: once arrival is recorded, subsequent attempts return already reported
 */
export function evaluateArrivalConfirmation(
  request: ArrivalVerificationRequest
): ArrivalVerificationResult {
  if (request.currentState === 'ARRIVAL_REPORTED') {
    return {
      allowed: false,
      nextState: 'ARRIVAL_REPORTED',
      isNewTransition: false,
      arrivalSuccess: true,
      errorMessage: 'ALREADY_REPORTED',
      canRetry: false,
    };
  }

  if (request.currentState === 'ROUTE_REVOKED') {
    return {
      allowed: false,
      nextState: 'ROUTE_REVOKED',
      isNewTransition: false,
      arrivalSuccess: false,
      errorMessage: 'ROUTE_REVOKED',
      canRetry: false,
    };
  }

  if (!request.activeStayId) {
    return {
      allowed: false,
      nextState: request.currentState,
      isNewTransition: false,
      arrivalSuccess: false,
      errorMessage: 'No verified stay reservation found. Please select an authorized route and reserve a stay before confirming arrival.',
      canRetry: true,
    };
  }

  if (!request.serverResponse) {
    return {
      allowed: false,
      nextState: request.currentState,
      isNewTransition: false,
      arrivalSuccess: false,
      errorMessage: 'Network error or timeout while reporting arrival. Please try again.',
      canRetry: true,
    };
  }

  if (!request.serverResponse.ok) {
    return {
      allowed: false,
      nextState: request.currentState,
      isNewTransition: false,
      arrivalSuccess: false,
      errorMessage: request.serverResponse.error || `Server rejected arrival (${request.serverResponse.status})`,
      canRetry: true,
    };
  }

  return {
    allowed: true,
    nextState: 'ARRIVAL_REPORTED',
    isNewTransition: true,
    arrivalSuccess: true,
    canRetry: false,
  };
}

/**
 * Guard for in-flight voice/network responses:
 * Drops responses when:
 * 1. Document is in the background (hidden)
 * 2. Active request ID has moved past the response request ID (superseded by a newer request or cancelled)
 */
export function shouldDropResponse(activeRequestId: number, responseRequestId: number, isHidden: boolean): boolean {
  if (isHidden) return true;
  if (activeRequestId !== responseRequestId) return true;
  return false;
}

export interface GuidanceSnapshotValidation {
  isValid: boolean;
  dataVersion?: string;
  snapshotVersion?: number;
  errorMessage?: string;
}

/**
 * Validates guidance snapshot and package version from server response:
 * - source_status must be 'CURRENT'
 * - data_version must be non-empty and cannot be 'none'
 * - snapshot_version must be a positive integer
 */
export function validateGuidanceSnapshot(
  sourceStatus: string | undefined,
  dataVersion: string | undefined,
  snapshotVersion: unknown
): GuidanceSnapshotValidation {
  if (!sourceStatus || sourceStatus !== 'CURRENT') {
    return {
      isValid: false,
      errorMessage: `Source status is ${sourceStatus || 'UNAVAILABLE'}. Guidance unavailable.`,
    };
  }
  if (!dataVersion || dataVersion === 'none' || dataVersion.trim() === '') {
    return {
      isValid: false,
      errorMessage: 'Missing authoritative data version (data_version cannot be empty or "none").',
    };
  }
  if (typeof snapshotVersion !== 'number' || !Number.isInteger(snapshotVersion) || snapshotVersion <= 0) {
    return {
      isValid: false,
      errorMessage: 'Missing or non-positive snapshot_version in guidance payload.',
    };
  }
  return {
    isValid: true,
    dataVersion,
    snapshotVersion,
  };
}

export interface RawGuidanceDestination {
  facility_id: string;
  safe_zone_id: string;
  capacity_known: boolean;
  free: number | null;
  route_id?: string;
  route_verified: boolean;
}

export interface ResolvedDestinationChoice {
  facility_id: string;
  safe_zone_id: string;
  facility_name: string;
  capacity_known: boolean;
  free: number | null;
  route_id?: string;
  route_verified: boolean;
  coordinates?: [number, number];
  distance_km?: number;
  duration_minutes?: number;
  is_illustrative: boolean;
}

/**
 * Maps raw server guidance destinations without inventing routes or using other facilities' coordinates.
 */
export function mapGuidanceDestinations(
  rawList: RawGuidanceDestination[],
  demoFacilityId: string,
  demoCoordinates?: [number, number],
  userPosition?: { longitude: number; latitude: number } | null
): ResolvedDestinationChoice[] {
  return rawList.map((d) => {
    // Only associate coordinates if facility matches known coordinates; do NOT use another facility's coordinates!
    const coords = d.facility_id === demoFacilityId ? demoCoordinates : undefined;
    const dist =
      coords && userPosition
        ? Math.round(computeDistanceMeters(userPosition.longitude, userPosition.latitude, coords[0], coords[1]) / 100) / 10
        : undefined;

    return {
      facility_id: d.facility_id,
      safe_zone_id: d.safe_zone_id,
      facility_name: d.facility_id === demoFacilityId ? `Demo Safe Facility (${d.safe_zone_id})` : `Facility ${d.facility_id}`,
      capacity_known: d.capacity_known,
      free: d.free,
      route_id: d.route_id, // Preserved as supplied by server; no demo fallback!
      route_verified: d.route_verified,
      coordinates: coords,
      distance_km: dist,
      duration_minutes: undefined,
      is_illustrative: false,
    };
  });
}

export interface ChoiceResolutionResult {
  resolved: boolean;
  destination: ResolvedDestinationChoice | null;
  errorMessage?: string;
}

/**
 * Resolves a selected facility against current verified guidance.
 * Never fabricates safe_zone_id or assigns a demo route.
 */
export function resolveChoiceAgainstGuidance(
  targetId: string,
  verifiedDestinations: ResolvedDestinationChoice[]
): ChoiceResolutionResult {
  const match = verifiedDestinations.find(
    (d) => (d.facility_id === targetId || d.safe_zone_id === targetId) && !d.is_illustrative
  );
  if (match) {
    return { resolved: true, destination: match };
  }
  return {
    resolved: false,
    destination: null,
    errorMessage: `Facility or safe zone "${targetId}" is not verified in current guidance snapshot. Please refresh guidance.`,
  };
}

export interface BuildReservationRequest {
  facilityId: string;
  packageId: string;
  routeId?: string | null;
  partySize: number;
  startDate: string;
  endDate: string;
  idempotencyKey: string;
  snapshotVersion: number | null | undefined;
  isIllustrative?: boolean;
}

export interface ReservationPayloadResult {
  canAllocate: boolean;
  errorMessage?: string;
  payload?: {
    facility_id: string;
    package_id: string;
    route_id?: string;
    party_size: number;
    start_date: string;
    end_date: string;
    idempotency_key: string;
    snapshot_version: number;
  };
}

/**
 * Builds and validates reservation submission payload bound strictly to guidance snapshot_version.
 * Fails allocation if snapshot_version is non-positive/missing or guidance is illustrative.
 * Preserves missing route identity (never supplies demo fallback).
 */
export function buildReservationPayload(req: BuildReservationRequest): ReservationPayloadResult {
  if (req.isIllustrative) {
    return {
      canAllocate: false,
      errorMessage: 'Cannot allocate reservation against illustrative preview. Guidance must be verified.',
    };
  }
  if (!req.snapshotVersion || req.snapshotVersion <= 0 || !Number.isInteger(req.snapshotVersion)) {
    return {
      canAllocate: false,
      errorMessage: 'Missing or non-positive snapshot version. Cannot allocate reservation.',
    };
  }
  if (!req.facilityId) {
    return {
      canAllocate: false,
      errorMessage: 'facility_id is required.',
    };
  }

  const payload: {
    facility_id: string;
    package_id: string;
    route_id?: string;
    party_size: number;
    start_date: string;
    end_date: string;
    idempotency_key: string;
    snapshot_version: number;
  } = {
    facility_id: req.facilityId,
    package_id: req.packageId,
    party_size: req.partySize,
    start_date: req.startDate,
    end_date: req.endDate,
    idempotency_key: req.idempotencyKey,
    snapshot_version: req.snapshotVersion,
  };

  // Preserve missing route identity: if server supplied none, retain that absence
  if (req.routeId && req.routeId.trim() !== '') {
    payload.route_id = req.routeId.trim();
  }

  return { canAllocate: true, payload };
}


