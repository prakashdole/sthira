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
  longitude: number;
  latitude: number;
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
  reason: 'VERIFIED_NEAR' | 'OUTSIDE_RANGE' | 'LOW_ACCURACY' | 'STALE' | 'INVALID_COORDINATES';
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
 */
export function evaluateProximity(
  pos: PositionReading,
  dest: DestinationTarget,
  options: JourneyOptions = DEFAULT_JOURNEY_OPTIONS,
  currentTimeMs: number = Date.now()
): ProximityEvaluation {
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
    if (evaluation.reason !== 'INVALID_COORDINATES' && !evaluation.isStale) {
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
    // If the citizen moved back away, transition back to tracking
    if (!evaluation.isNear && evaluation.isAccurateEnough && !evaluation.isStale) {
      return 'TRACKING';
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
