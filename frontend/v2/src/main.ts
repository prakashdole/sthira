import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Map as MapLibreMap } from 'maplibre-gl';
import mapData from './mapData.json';
import { words, type Language } from './i18n';
import {
  validateVoiceResponse,
  executeMapActions,
  type MapAction as VoiceMapAction,
  type Panel,
} from './mapActions';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
import '@fontsource/noto-sans/800.css';
import '@fontsource/noto-sans-malayalam/400.css';
import '@fontsource/noto-sans-malayalam/700.css';
import '@fontsource/noto-sans-devanagari/400.css';
import '@fontsource/noto-sans-devanagari/700.css';
import {
  type JourneyState,
  type PositionReading,
  type DestinationTarget,
  type ProximityEvaluation,
  computeDistanceMeters,
  evaluateProximity,
  transitionOnPosition,
  transitionOnArrival,
  transitionOnRevocation,
  DEFAULT_JOURNEY_OPTIONS,
} from './journey';
import { triggerEmergencyDial } from './emergency';
import {
  type AudioMetadata,
  type VoiceResponseEnvelope,
  processVoiceEnvelope,
  isAudioValidForReplay,
  verifyAudioIntegrity,
  AudioPlaybackGuard,
  buildVoicePipelineRequest,
  evaluateReadinessState,
  shouldDropRecordedAudio,
} from './audioGuidance';

type RuntimeState = 'checking' | 'demo' | 'blocked' | 'offline';
type OnboardingStep = 'starting' | 'language' | 'location' | null;
type LocationStatus = 'idle' | 'checking' | 'ready' | 'unavailable';

type DestinationChoice = {
  facility_id: string;
  safe_zone_id: string;
  facility_name: string;
  capacity_known: boolean;
  free: number | null;
  route_id?: string;
  route_verified: boolean;
  distance_km?: number;
  duration_minutes?: number;
  is_illustrative?: boolean;
};

type AmbiguousCandidate = { place_id: string; place_kind: string };

const icons: Record<string, string> = {
  arrow: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M13 6l6 6-6 6"/></svg>',
  mic: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="3" width="6" height="12" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3M9 21h6"/></svg>',
  locate: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3"/></svg>',
  route: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="6" cy="18" r="2"/><circle cx="18" cy="6" r="2"/><path d="M8 18h3a3 3 0 0 0 3-3V9a3 3 0 0 1 3-3"/></svg>',
  phone: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 3h3l1.25 4.2-2.1 1.15a15 15 0 0 0 6.5 6.5l1.15-2.1L21 14v3c0 1.1-.9 2-2 2C11.27 19 5 12.73 5 5c0-1.1.9-2 2-2Z"/></svg>',
  volume: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M11 5 6 9H3v6h3l5 4V5ZM15 9a5 5 0 0 1 0 6M18 6a9 9 0 0 1 0 12"/></svg>',
  close: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>',
  info: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/></svg>',
};

// Bounded exercise configuration matching cmd/sthira-exercise
const EXERCISE_CONFIG = {
  jurisdiction: 'DEMO-EXERCISE',
  package_id: 'PKGDEMO-1',
  facility_id: 'FACDEMO-1',
  safe_zone_id: 'SZDEMO-1',
  route_id: 'RTDEMO-1',
  snapshot_version: 1,
};
const JURISDICTION = EXERCISE_CONFIG.jurisdiction;
const PACKAGE_ID = EXERCISE_CONFIG.package_id;

function getTodayYMD(): string {
  return new Date().toISOString().slice(0, 10);
}
function getTomorrowYMD(): string {
  return new Date(Date.now() + 86400_000).toISOString().slice(0, 10);
}

function savedLanguage(): Language {
  try {
    const saved = localStorage.getItem('sthira-language');
    return saved === 'ML' || saved === 'HI' || saved === 'EN' ? saved : 'EN';
  } catch {
    return 'EN';
  }
}

function savedOnboardingStep(): OnboardingStep {
  return 'starting';
}

function speechLanguageTag(lang: Language): string {
  switch (lang) {
    case 'ML': return 'ml-IN';
    case 'HI': return 'hi-IN';
    case 'EN': return 'en-IN';
  }
}

let language: Language = savedLanguage();
let onboardingStep: OnboardingStep = savedOnboardingStep();
let locationStatus: LocationStatus = 'idle';
let deviceLocation: [number, number] | null = null;
let runtime: RuntimeState = navigator.onLine ? 'checking' : 'offline';
let runtimeDetail: 'blocked' | 'responding' | 'disconnected' = 'disconnected';

let map: MapLibreMap | null = null;
let mapAnimationFrame: number | null = null;
let mapRenderVersion = 0;
let hasRendered = false;
let redZonesVisible = false;
let relocationZonesVisible = false;
let layersOpen = false;
let mapTilted = false;
let perspectiveCamera: { center: [number, number]; zoom: number } | null = null;
let pendingZoneFocus: 'RED' | 'RELOCATION' | null = null;

let voiceOpen = false;
let voiceListening = false;
let voiceTranscript = '';
let voiceFeedbackKey: Exclude<keyof typeof words.EN, 'suggestions'> = 'micPrivacy';
let mediaRecorder: MediaRecorder | null = null;
let mediaStream: MediaStream | null = null;
let recordingStartedAt = 0;
let recordingCancelled = false;
let recordingTimer: number | null = null;

let commandPending = false;
let commandError = '';
let commandResponse: string = words[language].voiceReady;
let commandSuggestions: string[] = [...words[language].voiceCommands];

let sessionToken: string | null = sessionStorage.getItem('sthira_session_token');
let sessionId: string = sessionStorage.getItem('sthira_session_id') || ('SES-' + Math.random().toString(36).slice(2, 10));
let activeReservationId: string | null = sessionStorage.getItem('sthira_reservation_id');
let activeStayId: string | null = sessionStorage.getItem('sthira_stay_id');

let routeStarted = false;
let directionsOpen = false;
let detailsOpen = false;
let assistanceOpen = false;
let arrivalOpen = false;
let audioOpen = false;
let islOpen = false;

let partySize = 1;
let arrivalSuccess = false;
let arrivalRecordedAt: string | null = null;
let arrivalPending = false;
let arrivalError = '';
let reservationPending = false;
let reservationError = '';

let isIllustrativePreview = true;
let guidanceStatus: 'PENDING' | 'LOADED' | 'EMPTY' | 'ERROR' = 'PENDING';
let guidanceFreshness = 'UNKNOWN';
let guidanceErrorMessage = '';
let currentDataVersion = 'v1';

let ambiguousPlaces: AmbiguousCandidate[] = [];
let availableDestinations: DestinationChoice[] = [
  {
    facility_id: EXERCISE_CONFIG.facility_id,
    safe_zone_id: EXERCISE_CONFIG.safe_zone_id,
    facility_name: 'Demo Safe Facility (SZDEMO-1)',
    capacity_known: false,
    free: null,
    route_id: EXERCISE_CONFIG.route_id,
    route_verified: false,
    distance_km: undefined,
    duration_minutes: undefined,
    is_illustrative: true,
  },
];
let selectedDestination: DestinationChoice | null = availableDestinations[0]!;

let journeyState: JourneyState = 'NOT_STARTED';
let currentPosition: PositionReading | null = null;
let lastProximityEval: ProximityEvaluation | null = null;
let geolocationWatchId: number | null = null;
let locationErrorMessage = '';

let activeRequestId = 0;
const audioGuard = new AudioPlaybackGuard();
let lastApprovedAudio: AudioMetadata | undefined = undefined;

let queuedVoiceActions: VoiceMapAction[] = [];

function authHeaders(): Record<string, string> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    'X-Request-ID': 'req-' + Math.random().toString(36).slice(2, 10),
  };
  if (sessionToken) {
    headers['Authorization'] = `Bearer ${sessionToken}`;
  }
  return headers;
}

function runtimeCopy(): string {
  const t = words[language];
  if (runtime === 'offline') return t.offline;
  if (runtime === 'blocked' && runtimeDetail === 'blocked') return t.blocked;
  if (runtimeDetail === 'disconnected') return t.localDisconnected;
  if (runtime === 'demo') return t.responding;
  return t.checking;
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]!);
}

function completeOnboarding() {
  onboardingStep = null;
  render();
}

function requestLocation() {
  if (!navigator.geolocation) {
    locationStatus = 'unavailable';
    render();
    return;
  }
  locationStatus = 'checking';
  render();
  navigator.geolocation.getCurrentPosition(
    (position) => {
      deviceLocation = [position.coords.longitude, position.coords.latitude];
      locationStatus = 'ready';
      render();
    },
    () => {
      locationStatus = 'unavailable';
      render();
    },
    { enableHighAccuracy: false, timeout: 8_000, maximumAge: 300_000 },
  );
}

function getDestinationTarget(): DestinationTarget {
  return {
    id: selectedDestination ? selectedDestination.facility_id : EXERCISE_CONFIG.facility_id,
    name: selectedDestination ? selectedDestination.facility_name : 'Safe Shelter',
    longitude: mapData.shelter[0],
    latitude: mapData.shelter[1],
  };
}

function destinationDistanceText(d: DestinationChoice | null): { kmText: string; durationText: string } {
  const t = words[language];
  if (!d) return { kmText: '—', durationText: '—' };
  if (d.is_illustrative) return { kmText: t.distance || '2.8 km', durationText: t.duration || '35 min' };
  if (currentPosition) {
    const distM = computeDistanceMeters(currentPosition.longitude, currentPosition.latitude, mapData.shelter[0], mapData.shelter[1]);
    const km = (distM / 1000).toFixed(1);
    const mins = Math.max(1, Math.round(distM / 80));
    return { kmText: `${km} km`, durationText: `${mins} min` };
  }
  return { kmText: t.distance || '2.8 km', durationText: t.duration || '35 min' };
}

function destinationCapacityText(): string {
  const t = words[language];
  if (!selectedDestination) return '';
  if (isIllustrativePreview) return t.destinationMeta;
  if (!selectedDestination.capacity_known) {
    return t.capacityUnknown;
  }
  if (selectedDestination.free === null || selectedDestination.free > 0) {
    return selectedDestination.free !== null ? `${selectedDestination.free} spaces free (${t.capacityAvailable})` : t.capacityAvailable;
  }
  return t.capacityFull;
}

function journeyBadgeClass(state: JourneyState): string {
  switch (state) {
    case 'TRACKING': return 'tracking';
    case 'NEAR_DESTINATION': return 'near';
    case 'ARRIVAL_REPORTED': return 'near';
    case 'ROUTE_REVOKED': return 'revoked';
    case 'PAUSED': return 'paused';
    case 'LOCATION_UNAVAILABLE': return 'unavailable';
    default: return 'paused';
  }
}

function journeyStateLabel(state: JourneyState, t: typeof words[Language]): string {
  switch (state) {
    case 'NOT_STARTED': return t.journeyTracking;
    case 'TRACKING': return t.trackingActive;
    case 'NEAR_DESTINATION': return t.trackingNear;
    case 'ARRIVAL_REPORTED': return t.arrivalRecorded;
    case 'PAUSED': return t.trackingPaused;
    case 'LOCATION_UNAVAILABLE': return t.locationUnavailable;
    case 'ROUTE_REVOKED': return t.routeRevokedNotice;
    default: return state;
  }
}

function journeyTrackingDetail(
  state: JourneyState,
  evalResult: ProximityEvaluation | null,
  pos: PositionReading | null,
  t: typeof words[Language]
): string {
  if (state === 'ARRIVAL_REPORTED') {
    return arrivalRecordedAt ? `Arrived at ${arrivalRecordedAt}` : 'Arrived safely';
  }
  if (state === 'LOCATION_UNAVAILABLE') {
    return t.locationUnavailable;
  }
  if (state === 'ROUTE_REVOKED') {
    return 'Route revoked';
  }
  if (!pos) {
    return state === 'NOT_STARTED' ? 'GPS idle' : 'Waiting for GPS fix...';
  }
  if (evalResult && evalResult.isNear) {
    return `${evalResult.distanceMeters}m from shelter (±${Math.round(pos.accuracyMeters)}m)`;
  }
  if (evalResult && !Number.isNaN(evalResult.distanceMeters)) {
    return `${(evalResult.distanceMeters / 1000).toFixed(1)} km to shelter (±${Math.round(pos.accuracyMeters)}m)`;
  }
  return `Accuracy ±${Math.round(pos.accuracyMeters)}m`;
}

function trackingButtonHtml(state: JourneyState, t: typeof words[Language]): string {
  if (state === 'ARRIVAL_REPORTED' || state === 'ROUTE_REVOKED') {
    return '';
  }
  if (state === 'TRACKING' || state === 'NEAR_DESTINATION') {
    return `<button class="secondary-action" style="font-size: 0.72rem; min-height: 2rem; padding: 0.2rem 0.6rem;" type="button" data-action="stop-tracking">${icons.locate} ${t.stopJourneyTracking}</button>`;
  }
  return `<button class="secondary-action" style="font-size: 0.72rem; min-height: 2rem; padding: 0.2rem 0.6rem;" type="button" data-action="start-tracking">${icons.locate} ${t.startJourneyTracking}</button>`;
}

async function reconcileStayState(): Promise<void> {
  const savedStayId = sessionStorage.getItem('sthira_stay_id');
  if (!savedStayId) return;

  try {
    const res = await fetch(`/api/v3/reservations/${encodeURIComponent(savedStayId)}`, {
      method: 'GET',
      headers: authHeaders(),
    });
    if (res.ok) {
      const body = (await res.json()) as { data?: { stay_id?: string; state?: string; facility_id?: string } };
      if (body.data) {
        activeStayId = body.data.stay_id || savedStayId;
        if (body.data.state === 'ARRIVED') {
          journeyState = 'ARRIVAL_REPORTED';
          arrivalSuccess = true;
          routeStarted = true;
        } else if (body.data.state === 'RESERVED') {
          routeStarted = true;
        } else if (body.data.state === 'CANCELLED' || body.data.state === 'REVOKED' || body.data.state === 'EXPIRED') {
          journeyState = 'ROUTE_REVOKED';
        }
        render();
      }
    } else if (res.status === 404 || res.status === 401 || res.status === 403) {
      sessionStorage.removeItem('sthira_stay_id');
      sessionStorage.removeItem('sthira_reservation_id');
      activeStayId = null;
      activeReservationId = null;
    }
  } catch {
    // Graceful offline fallback
  }
}

async function initSession(): Promise<void> {
  if (sessionToken) {
    await reconcileStayState();
    return;
  }
  try {
    const res = await fetch('/api/v3/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ label: 'web-citizen' }),
    });
    if (res.ok) {
      const data = (await res.json()) as { data?: { token?: string; session_id?: string } };
      if (data.data?.token) {
        sessionToken = data.data.token;
        sessionStorage.setItem('sthira_session_token', sessionToken);
      }
      if (data.data?.session_id) {
        sessionId = data.data.session_id;
        sessionStorage.setItem('sthira_session_id', sessionId);
      }
      await reconcileStayState();
    }
  } catch {
    // Public demo fallback
  }
}

async function checkRuntime(): Promise<void> {
  if (!navigator.onLine) {
    runtime = 'offline';
    runtimeDetail = 'disconnected';
    render();
    return;
  }
  try {
    const readyRes = await fetch('/health/ready');
    const data = await readyRes.json().catch(() => null);
    const evalState = evaluateReadinessState(readyRes.status, data);
    runtime = evalState.runtime;
    runtimeDetail = evalState.runtimeDetail;
  } catch {
    // A failed connection must NOT imply service readiness.
    const evalState = evaluateReadinessState(null, null);
    runtime = evalState.runtime;
    runtimeDetail = evalState.runtimeDetail;
  }
  render();
}

async function queryGuidanceDestinations(): Promise<void> {
  guidanceStatus = 'PENDING';
  guidanceErrorMessage = '';
  try {
    const res = await fetch('/api/v3/guidance/query', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        jurisdiction: JURISDICTION,
        package_id: PACKAGE_ID,
        party_size: partySize,
        start_date: getTodayYMD(),
        end_date: getTomorrowYMD(),
      }),
    });
    if (res.ok) {
      const envelope = (await res.json()) as {
        status?: string;
        data_version?: string;
        source_status?: string;
        data?: {
          destinations?: Array<{
            facility_id: string;
            safe_zone_id: string;
            capacity_known: boolean;
            free: number | null;
            route_id?: string;
            route_verified: boolean;
          }>;
          package_id?: string;
          jurisdiction?: string;
        };
      };
      if (!envelope.source_status || envelope.source_status !== 'CURRENT') {
        guidanceFreshness = envelope.source_status || 'UNAVAILABLE';
        guidanceStatus = 'ERROR';
        guidanceErrorMessage = `Source status is ${guidanceFreshness}. Guidance unavailable.`;
        currentDataVersion = 'UNAVAILABLE';
        audioGuard.invalidate();
        lastApprovedAudio = undefined;
        availableDestinations = [];
        selectedDestination = null;
        isIllustrativePreview = false;
        render();
        return;
      }
      guidanceFreshness = 'CURRENT';
      const nextDataVersion = envelope.data_version || 'v1';
      if (nextDataVersion !== currentDataVersion) {
        audioGuard.invalidate();
        lastApprovedAudio = undefined;
      }
      currentDataVersion = nextDataVersion;
      const dests = envelope.data?.destinations;
      if (dests && dests.length > 0) {
        availableDestinations = dests.map((d) => ({
          facility_id: d.facility_id,
          safe_zone_id: d.safe_zone_id,
          facility_name: d.facility_id === EXERCISE_CONFIG.facility_id ? 'Demo Safe Facility (SZDEMO-1)' : `Shelter ${d.facility_id}`,
          capacity_known: d.capacity_known,
          free: d.free,
          route_id: d.route_id || EXERCISE_CONFIG.route_id,
          route_verified: d.route_verified,
          distance_km: currentPosition ? Math.round(computeDistanceMeters(currentPosition.longitude, currentPosition.latitude, mapData.shelter[0], mapData.shelter[1]) / 100) / 10 : undefined,
          duration_minutes: undefined,
          is_illustrative: false,
        }));
        selectedDestination = availableDestinations[0]!;
        isIllustrativePreview = false;
        guidanceStatus = 'LOADED';
      } else {
        availableDestinations = [];
        selectedDestination = null;
        isIllustrativePreview = false;
        guidanceStatus = 'EMPTY';
      }
    } else {
      isIllustrativePreview = false;
      guidanceStatus = 'ERROR';
      guidanceFreshness = 'UNAVAILABLE';
      currentDataVersion = 'UNAVAILABLE';
      audioGuard.invalidate();
      lastApprovedAudio = undefined;
      availableDestinations = [];
      selectedDestination = null;
      guidanceErrorMessage = `Authority returned HTTP ${res.status}. Guidance unavailable.`;
    }
  } catch {
    isIllustrativePreview = false;
    guidanceStatus = 'ERROR';
    guidanceFreshness = 'UNAVAILABLE';
    currentDataVersion = 'UNAVAILABLE';
    audioGuard.invalidate();
    lastApprovedAudio = undefined;
    availableDestinations = [];
    selectedDestination = null;
    guidanceErrorMessage = 'Network error: could not connect to guidance service.';
  }
  render();
}

async function selectCandidatePlace(candId: string) {
  ambiguousPlaces = [];
  voiceTranscript = `Selected: ${candId}`;
  commandResponse = `Resolving ${candId}...`;
  render();
  await resolvePlace(candId);
}

async function resolvePlace(query: string): Promise<void> {
  try {
    const res = await fetch('/api/v3/places/resolve', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({ jurisdiction: JURISDICTION, query }),
    });
    if (res.status === 409) {
      const errData = (await res.json()) as { errors?: Array<{ code: string; message: string; details?: { candidates?: AmbiguousCandidate[] } }> };
      const candidates = errData.errors?.[0]?.details?.candidates;
      if (candidates && candidates.length > 0) {
        ambiguousPlaces = candidates;
        commandResponse = `Multiple locations match "${query}". Please choose one:`;
        render();
        return;
      }
    }
    if (res.ok) {
      ambiguousPlaces = [];
      const data = (await res.json()) as { data?: { place_id: string; place_kind: string } };
      if (data.data?.place_id) {
        const pId = data.data.place_id;
        commandResponse = `Resolved location: ${pId} (${data.data.place_kind || 'place'})`;
        await queryGuidanceDestinations();
        render();
      }
    } else if (res.status === 404) {
      ambiguousPlaces = [];
      commandResponse = `Location "${query}" not found in jurisdiction ${JURISDICTION}.`;
      render();
    }
  } catch {
    commandResponse = `Place lookup failed for "${query}".`;
    render();
  }
}

function isAudioValidForReplayCheck(audio?: AudioMetadata): boolean {
  return isAudioValidForReplay(audio, guidanceFreshness, currentDataVersion, speechLanguageTag(language));
}

async function verifyAndPlayAudio(audioInfo: AudioMetadata): Promise<{ success: boolean; autoplayBlocked: boolean; error?: string }> {
  const currentGen = audioGuard.currentGeneration;
  const currentFreshness = guidanceFreshness;
  const currentDataVer = currentDataVersion;
  const currentLang = speechLanguageTag(language);

  if (!isAudioValidForReplayCheck(audioInfo)) {
    return {
      success: false,
      autoplayBlocked: false,
      error: 'Audio guidance is invalid, expired, or does not match active language and version',
    };
  }

  const integrity = await verifyAudioIntegrity(audioInfo);
  if (!integrity.success) {
    return {
      success: false,
      autoplayBlocked: false,
      error: integrity.error || 'Audio integrity verification failed',
    };
  }

  if (
    audioGuard.currentGeneration !== currentGen ||
    guidanceFreshness !== currentFreshness ||
    currentDataVersion !== currentDataVer ||
    speechLanguageTag(language) !== currentLang ||
    guidanceFreshness !== 'CURRENT'
  ) {
    return {
      success: false,
      autoplayBlocked: false,
      error: 'Audio playback cancelled: UI context changed during verification',
    };
  }

  return new Promise((resolve) => {
    try {
      const mime = audioInfo.content_type || 'audio/wav';
      const audio = new Audio(`data:${mime};base64,${audioInfo.audio_b64}`);
      audio
        .play()
        .then(() => {
          if (
            audioGuard.currentGeneration !== currentGen ||
            guidanceFreshness !== currentFreshness ||
            currentDataVersion !== currentDataVer ||
            speechLanguageTag(language) !== currentLang ||
            guidanceFreshness !== 'CURRENT'
          ) {
            audio.pause();
            resolve({ success: false, autoplayBlocked: false, error: 'Context changed during playback start' });
            return;
          }
          audioGuard.invalidate();
          resolve({ success: true, autoplayBlocked: false });
        })
        .catch((err: Error) => {
          if (err.name === 'NotAllowedError') {
            if (
              audioGuard.currentGeneration === currentGen &&
              guidanceFreshness === currentFreshness &&
              currentDataVersion === currentDataVer &&
              speechLanguageTag(language) === currentLang &&
              guidanceFreshness === 'CURRENT'
            ) {
              audioGuard.setPending({
                audio,
                metadata: audioInfo,
                expectedLanguage: currentLang,
                expectedDataVersion: currentDataVer,
              });
              render();
              resolve({ success: false, autoplayBlocked: true, error: 'Autoplay blocked by browser policy. Tap to play.' });
            } else {
              resolve({ success: false, autoplayBlocked: false, error: 'Context changed before autoplay could be queued' });
            }
          } else {
            resolve({ success: false, autoplayBlocked: false, error: err.message });
          }
        });
    } catch (e: any) {
      resolve({ success: false, autoplayBlocked: false, error: e?.message || 'Audio playback error' });
    }
  });
}

async function startRouteReservation() {
  if (!selectedDestination || isIllustrativePreview) {
    reservationError = 'Cannot reserve route for illustrative preview. Waiting for authorized operational guidance.';
    render();
    return;
  }

  routeStarted = true;
  directionsOpen = true;
  reservationPending = true;
  reservationError = '';
  render();
  focusRoute();

  try {
    await initSession();
    const pendingKey = sessionStorage.getItem('pending_reservation_idem') || ('idem-' + Math.random().toString(36).slice(2, 10));
    sessionStorage.setItem('pending_reservation_idem', pendingKey);

    const res = await fetch('/api/v3/reservations', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        facility_id: selectedDestination.facility_id,
        package_id: PACKAGE_ID,
        route_id: selectedDestination.route_id || EXERCISE_CONFIG.route_id,
        party_size: partySize,
        start_date: getTodayYMD(),
        end_date: getTomorrowYMD(),
        idempotency_key: pendingKey,
        snapshot_version: EXERCISE_CONFIG.snapshot_version,
      }),
    });
    if (res.ok) {
      sessionStorage.removeItem('pending_reservation_idem');
      const data = (await res.json()) as { data?: { reservation_id?: string; stay_id?: string } };
      if (data.data?.reservation_id) {
        activeReservationId = data.data.reservation_id;
        sessionStorage.setItem('sthira_reservation_id', activeReservationId);
      }
      if (data.data?.stay_id) {
        activeStayId = data.data.stay_id;
        sessionStorage.setItem('sthira_stay_id', activeStayId);
      }
      reservationPending = false;
      reservationError = '';
      render();
    } else {
      const errJson = await res.json().catch(() => null);
      reservationError = errJson?.error?.message || errJson?.message || `Reservation failed (${res.status})`;
      reservationPending = false;
      render();
    }
  } catch {
    reservationError = 'Network error while requesting reservation.';
    reservationPending = false;
    render();
  }
}

async function confirmArrival() {
  const transition = transitionOnArrival(journeyState);
  if (!transition.isNewTransition) {
    arrivalOpen = false;
    render();
    return;
  }

  arrivalPending = true;
  arrivalError = '';
  render();

  if (activeStayId) {
    const idemKey = sessionStorage.getItem('pending_arrival_idem') || ('arrive-' + Math.random().toString(36).slice(2, 10));
    sessionStorage.setItem('pending_arrival_idem', idemKey);

    try {
      const res = await fetch(`/api/v3/reservations/${encodeURIComponent(activeStayId)}/events`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          type: 'ARRIVE',
          idempotency_key: idemKey,
        }),
      });

      if (res.ok) {
        sessionStorage.removeItem('pending_arrival_idem');
        journeyState = 'ARRIVAL_REPORTED';
        arrivalSuccess = true;
        arrivalRecordedAt = new Date().toLocaleTimeString();
        arrivalPending = false;
        arrivalError = '';
        stopTracking();
        render();
      } else {
        const errJson = await res.json().catch(() => null);
        arrivalError = errJson?.error?.message || errJson?.message || `Server rejected arrival (${res.status})`;
        arrivalPending = false;
        render();
      }
    } catch {
      arrivalError = 'Network error while reporting arrival. Please try again.';
      arrivalPending = false;
      render();
    }
  } else {
    arrivalError = 'No verified stay reservation found. Please select an authorized route first.';
    arrivalPending = false;
    render();
  }
}

function startTracking() {
  locationErrorMessage = '';
  if (!navigator.geolocation) {
    journeyState = 'LOCATION_UNAVAILABLE';
    locationErrorMessage = words[language].locationUnavailable;
    render();
    return;
  }

  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }

  journeyState = 'TRACKING';
  render();

  geolocationWatchId = navigator.geolocation.watchPosition(
    (pos) => {
      if (document.hidden || journeyState === 'PAUSED' || journeyState === 'NOT_STARTED') {
        return;
      }
      const reading: PositionReading = {
        longitude: pos.coords.longitude,
        latitude: pos.coords.latitude,
        accuracyMeters: pos.coords.accuracy,
        timestamp: pos.timestamp || Date.now(),
      };
      applyPositionUpdate(reading);
    },
    (err) => {
      handleGeolocationError(err);
    },
    { enableHighAccuracy: true, timeout: 10000, maximumAge: 5000 }
  );
}

function stopTracking() {
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  if (journeyState !== 'ARRIVAL_REPORTED' && journeyState !== 'ROUTE_REVOKED') {
    journeyState = 'PAUSED';
  }
  render();
}

function handleGeolocationError(err: GeolocationPositionError) {
  journeyState = 'LOCATION_UNAVAILABLE';
  if (err.code === 1) {
    locationErrorMessage = words[language].locationDenied;
  } else {
    locationErrorMessage = words[language].locationUnavailable;
  }
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  render();
}

function applyPositionUpdate(reading: PositionReading) {
  currentPosition = reading;
  deviceLocation = [reading.longitude, reading.latitude];

  const evalResult = evaluateProximity(reading, getDestinationTarget(), DEFAULT_JOURNEY_OPTIONS);
  lastProximityEval = evalResult;

  const nextState = transitionOnPosition(journeyState, evalResult);
  journeyState = nextState;

  if (map && map.getSource('places')) {
    const placesGeoJSON = {
      type: 'FeatureCollection',
      features: [
        { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.user }, properties: { label: words[language].userMapLabel, kind: 'user' } },
        { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.shelter }, properties: { label: words[language].shelterMapLabel, kind: 'shelter' } },
        { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.hospital }, properties: { label: words[language].hospitalMapLabel, kind: 'hospital' } },
        ...(deviceLocation ? [{ type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: deviceLocation }, properties: { label: words[language].deviceMapLabel, kind: 'device' } }] : []),
      ],
    };
    (map.getSource('places') as any).setData(placesGeoJSON);
  }

  render();
}

function revokeRoute() {
  journeyState = transitionOnRevocation(journeyState);
  audioGuard.invalidate();
  lastApprovedAudio = undefined;
  guidanceFreshness = 'REVOKED';
  currentDataVersion = 'REVOKED';
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  render();
}

async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function cancelRecording() {
  recordingCancelled = true;
  if (recordingTimer !== null) {
    clearTimeout(recordingTimer);
    recordingTimer = null;
  }
  if (mediaStream) {
    mediaStream.getTracks().forEach((track) => track.stop());
    mediaStream = null;
  }
  if (mediaRecorder) {
    try {
      if (mediaRecorder.state === 'recording') mediaRecorder.stop();
    } catch {}
    mediaRecorder = null;
  }
  voiceListening = false;
}

async function toggleLocalRecording() {
  if (voiceListening && mediaRecorder) {
    recordingCancelled = false;
    if (recordingTimer !== null) {
      clearTimeout(recordingTimer);
      recordingTimer = null;
    }
    mediaRecorder.stop();
    return;
  }

  recordingCancelled = false;
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') ? 'audio/webm;codecs=opus' : 'audio/webm';
    const chunks: Blob[] = [];
    mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
    mediaRecorder.ondataavailable = (event) => {
      if (event.data.size) chunks.push(event.data);
    };
    mediaRecorder.onstop = async () => {
      if (shouldDropRecordedAudio(recordingCancelled, document.hidden)) {
        return;
      }
      const recording = new Blob(chunks, { type: mimeType });
      mediaStream?.getTracks().forEach((track) => track.stop());
      mediaStream = null;
      mediaRecorder = null;
      voiceListening = false;
      const b64 = await blobToBase64(recording);
      void sendVoiceOrText({ kind: 'audio', body_b64: b64, content_type: mimeType });
    };
    recordingStartedAt = Date.now();
    voiceListening = true;
    voiceFeedbackKey = 'recording';
    render();
    mediaRecorder.start();
    recordingTimer = window.setTimeout(() => {
      if (mediaRecorder?.state === 'recording') {
        mediaRecorder.stop();
      }
    }, 20_000);
  } catch {
    voiceListening = false;
    voiceFeedbackKey = 'micStopped';
    render();
  }
}

function applyQueuedVoiceActions() {
  const actions = queuedVoiceActions;
  queuedVoiceActions = [];
  for (const action of actions) {
    if (action.type === 'FOCUS_FEATURE' || action.type === 'HIGHLIGHT_FEATURE') {
      if (action.target_id === 'RZ-DEMO-01' || action.target_id === 'RZDEMO-1') map?.fitBounds(mapData.hazardBounds as [[number, number], [number, number]], { padding: 80, duration: motionDuration() });
      if (action.target_id === 'SZ-DEMO-01' || action.target_id === 'SZDEMO-1' || action.target_id === 'FACDEMO-1' || action.target_id === 'FAC-DEMO-01' || action.target_id === 'PLACE-DEMO-1' || action.target_id === 'PLACE-DEMO-2') map?.easeTo({ center: mapData.shelter as [number, number], zoom: 15, duration: motionDuration() });
      if (action.target_id === 'MY-LOCATION-DEMO') recenterMap();
      if (action.target_id === 'ROUTE-DEMO-01' || action.target_id === 'RTDEMO-1') focusRoute();
    }
    if (action.type === 'FIT_FEATURES' || action.type === 'SHOW_ROUTE') focusRoute();
    if (action.type === 'ZOOM') map?.zoomTo(map.getZoom() + (action.direction === 'IN' ? 1 : -1), { duration: motionDuration() });
    if (action.type === 'PAN') {
      const [lng, lat] = map?.getCenter().toArray() || mapData.user;
      const offsets = { NORTH: [0, 0.01], SOUTH: [0, -0.01], EAST: [0.01, 0], WEST: [-0.01, 0] } as const;
      const [dx, dy] = offsets[action.direction];
      map?.easeTo({ center: [lng + dx, lat + dy], duration: motionDuration() });
    }
    if (action.type === 'RECENTER') recenterMap();
  }
  if (pendingZoneFocus === 'RED') map?.fitBounds(mapData.hazardBounds as [[number, number], [number, number]], { padding: 70, duration: motionDuration() });
  if (pendingZoneFocus === 'RELOCATION') map?.fitBounds(mapData.relocationBounds as [[number, number], [number, number]], { padding: 70, duration: motionDuration() });
  pendingZoneFocus = null;
}

function applyVoiceActions(actions: VoiceMapAction[]) {
  queuedVoiceActions = actions;
  for (const action of actions) {
    if (action.type === 'SET_LAYER_VISIBILITY') {
      if (action.layer === 'RED_ZONES') redZonesVisible = action.visible;
      if (action.layer === 'SAFE_ZONES') relocationZonesVisible = action.visible;
      if (action.layer === 'ROUTES') routeStarted = action.visible;
    }
    if (action.type === 'OPEN_PANEL') {
      if (action.panel === 'ALERT_DETAILS') detailsOpen = true;
      if (action.panel === 'ROUTE_GUIDANCE' || action.panel === 'ROUTE_STEPS') directionsOpen = true;
      if (action.panel === 'EMERGENCY_CALL_CONFIRMATION') assistanceOpen = true;
      if (action.panel === 'ARRIVAL_CONFIRMATION') arrivalOpen = true;
    }
    if (action.type === 'SET_LANGUAGE') {
      language = action.language === 'ml-IN' ? 'ML' : action.language === 'hi-IN' ? 'HI' : 'EN';
      audioGuard.invalidate();
      lastApprovedAudio = undefined;
    }
  }
}

async function sendVoiceOrText(input: { kind: 'audio'; body_b64: string; content_type: string } | { kind: 'transcript'; text: string }) {
  const reqId = ++activeRequestId;
  audioGuard.invalidate();
  lastApprovedAudio = undefined;
  commandPending = true;
  commandError = '';
  voiceFeedbackKey = 'checkingBackend';
  if (input.kind === 'transcript') {
    voiceTranscript = input.text;
  } else {
    voiceTranscript = language === 'HI' ? 'आवाज़ इनपुट' : language === 'ML' ? 'വോയ്സ് ഇൻപുട്ട്' : 'Voice input';
  }
  render();

  try {
    const pipelineReq = buildVoicePipelineRequest(
      input,
      speechLanguageTag(language),
      JURISDICTION,
      'req-' + Math.random().toString(36).slice(2, 10)
    );

    const res = await fetch('/api/v3/voice/process', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify(pipelineReq),
    });

    if (reqId !== activeRequestId || document.hidden) {
      return;
    }

    if (!res.ok) {
      if (res.status === 503 || res.status === 504) {
        if (input.kind === 'transcript' && input.text.trim()) {
          commandPending = false;
          render();
          await resolvePlace(input.text.trim());
          return;
        }
        throw new Error('MODEL_UNAVAILABLE');
      }
      throw new Error(`Voice pipeline returned ${res.status}`);
    }

    const envelope = (await res.json()) as VoiceResponseEnvelope;
    if (reqId !== activeRequestId || document.hidden) return;

    const outcome = processVoiceEnvelope(envelope);

    if (outcome.kind === 'ERROR') {
      commandPending = false;
      commandError = words[language].assistantUnavailable;
      voiceFeedbackKey = 'backendUnavailable';
      render();
      return;
    }

    if (outcome.kind === 'CLARIFY') {
      commandPending = false;
      if (outcome.clarification_ids.length > 0) {
        ambiguousPlaces = outcome.clarification_ids.map((id) => ({ place_id: id, place_kind: 'candidate' }));
      }
      if (outcome.template_text) {
        commandResponse = outcome.template_text;
        voiceFeedbackKey = 'responseReady';
      } else {
        commandResponse = words[language].responseReady;
        voiceFeedbackKey = 'micPrivacy';
      }
      render();
      return;
    }

    // outcome.kind === 'OK'
    if (outcome.proposal) {
      if (map) {
        executeMapActions(
          map,
          outcome.proposal,
          motionDuration() === 0,
          (panel) => {
            if (panel === 'ROUTE_GUIDANCE' || panel === 'ROUTE_STEPS') directionsOpen = true;
            if (panel === 'ALERT_DETAILS') detailsOpen = true;
            if (panel === 'EMERGENCY_CALL_CONFIRMATION') assistanceOpen = true;
            if (panel === 'ARRIVAL_CONFIRMATION') arrivalOpen = true;
            render();
          },
          (newLang) => {
            if (newLang === 'ml-IN') language = 'ML';
            if (newLang === 'hi-IN') language = 'HI';
            if (newLang === 'en-IN') language = 'EN';
            audioGuard.invalidate();
            lastApprovedAudio = undefined;
            render();
          },
          (candidates) => {
            ambiguousPlaces = candidates.map((id) => ({ place_id: id, place_kind: 'candidate' }));
            render();
          }
        );
      } else {
        const validated = validateVoiceResponse(outcome.proposal);
        if (validated && validated.actions) {
          applyVoiceActions(validated.actions);
        }
      }
    }

    if (outcome.template_text) {
      commandResponse = outcome.template_text;
      voiceFeedbackKey = 'responseReady';
      if (outcome.audio) {
        lastApprovedAudio = outcome.audio;
        const playRes = await verifyAndPlayAudio(outcome.audio);
        if (!playRes.success && !playRes.autoplayBlocked) {
          commandError = `Audio verification notice: ${playRes.error}`;
        }
      } else if (outcome.guidanceSpeechExpected) {
        commandError = 'Audio verification notice: Audio integrity metadata missing or invalid';
      }
    } else if (outcome.captionUnavailable) {
      commandError = words[language].assistantUnavailable;
      voiceFeedbackKey = 'backendUnavailable';
    } else {
      commandResponse = words[language].responseReady;
      voiceFeedbackKey = 'responseReady';
    }

    commandPending = false;
    render();
  } catch {
    if (reqId !== activeRequestId) return;
    commandPending = false;
    commandError = words[language].commandUnavailable;
    voiceFeedbackKey = 'backendUnavailable';
    render();
  }
}

function renderOnboarding() {
  const t = words[language];
  document.documentElement.lang = language === 'ML' ? 'ml' : language === 'HI' ? 'hi' : 'en';
  const startingStep = onboardingStep === 'starting';
  const languageStep = onboardingStep === 'language';
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <main class="onboarding" aria-labelledby="onboarding-title">
      <section class="onboarding-card">
        <a class="brand onboarding-brand" href="/" aria-label="${t.brandHome}"><span class="brand-mark">സ്</span><span><strong>Sthira</strong><small>${t.tagline}</small></span></a>
        <div class="onboarding-copy">
          <span>${startingStep ? t.startingVoice : t.onboardingKicker}</span>
          <h1 id="onboarding-title">${startingStep ? t.startingVoice : languageStep ? t.onboardingLanguageTitle : t.onboardingLocationTitle}</h1>
          <p>${startingStep ? t.groundingLine : languageStep ? t.onboardingLanguageBody : t.onboardingLocationBody}</p>
        </div>
        ${startingStep ? `<button class="onboarding-primary onboarding-primary--voice" type="button" data-action="onboarding-start">${icons.mic}<span>${t.beginVoice}</span></button>` : languageStep ? `
          <div class="language-options" role="group" aria-label="${t.chooseLanguage}">
            ${(['EN', 'ML', 'HI'] as Language[]).map((code) => `<button class="${language === code ? 'is-active' : ''}" type="button" data-onboarding-language="${code}" aria-pressed="${language === code}"><strong>${code}</strong><span>${code === 'EN' ? 'English' : code === 'ML' ? 'മലയാളം' : 'हिन्दी'}</span></button>`).join('')}
          </div>
          <button class="onboarding-primary" type="button" data-action="onboarding-language-next">${t.onboardingContinue}${icons.arrow}</button>
        ` : `
          <div class="location-state location-state--${locationStatus}">
            ${locationStatus === 'checking' ? `<span>${t.locationChecking}</span>` : locationStatus === 'ready' ? `<strong>${t.locationReady}</strong>` : locationStatus === 'unavailable' ? `<strong>${t.locationUnavailable}</strong>` : `<span>${t.privacyNote}</span>`}
          </div>
          ${locationStatus === 'ready' || locationStatus === 'unavailable' ? `<button class="onboarding-primary" type="button" data-action="onboarding-complete">${t.startGuidance}${icons.arrow}</button>` : `<button class="onboarding-primary" type="button" data-action="location-request">${icons.locate}${t.useLocation}</button>`}
          ${locationStatus !== 'ready' ? `<button class="onboarding-secondary" type="button" data-action="onboarding-complete">${t.continueWithoutLocation}</button>` : ''}
        `}
      </section>
    </main>`;
  document.querySelectorAll<HTMLButtonElement>('[data-onboarding-language]').forEach((button) => button.addEventListener('click', () => {
    language = button.dataset.onboardingLanguage as Language;
    try { localStorage.setItem('sthira-language', language); } catch { /* Continue without storage. */ }
    renderOnboarding();
  }));
  document.querySelector<HTMLButtonElement>('[data-action="onboarding-start"]')?.addEventListener('click', () => { onboardingStep = 'language'; renderOnboarding(); });
  document.querySelector<HTMLButtonElement>('[data-action="onboarding-language-next"]')?.addEventListener('click', () => { onboardingStep = 'location'; renderOnboarding(); });
  document.querySelector<HTMLButtonElement>('[data-action="location-request"]')?.addEventListener('click', requestLocation);
  document.querySelectorAll<HTMLButtonElement>('[data-action="onboarding-complete"]').forEach((button) => button.addEventListener('click', completeOnboarding));
}

function render() {
  const t = words[language];
  if (mapAnimationFrame !== null) cancelAnimationFrame(mapAnimationFrame);
  mapAnimationFrame = null;
  map?.remove();
  mapRenderVersion += 1;
  if (onboardingStep) { renderOnboarding(); return; }
  document.documentElement.lang = language === 'ML' ? 'ml' : language === 'HI' ? 'hi' : 'en';
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell ${hasRendered ? '' : 'is-entering'} ${voiceOpen ? 'voice-is-open' : ''} ${directionsOpen ? 'route-is-open' : ''}">
      <header class="topbar">
        <a class="brand" href="/" aria-label="${t.brandHome}"><span class="brand-mark">സ്</span><span><strong>Sthira</strong><small>${t.tagline}</small></span></a>
        <div class="system-state system-state--${runtime}"><i></i><span>${runtimeCopy()}</span></div>
        <div class="top-actions">
          <button class="voice-launch ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-open" aria-expanded="${voiceOpen}">${icons.mic}<span>${t.voice}</span></button>
          <div class="language-switcher" role="group" aria-label="${t.chooseLanguage}">${(['EN', 'ML', 'HI'] as Language[]).map((code) => `<button class="${language === code ? 'is-active' : ''}" type="button" data-language="${code}" aria-pressed="${language === code}">${code}</button>`).join('')}</div>
        </div>
      </header>
      <main class="map-workspace">
        <section class="guidance-panel" data-testid="emergency-card" aria-labelledby="alert-title">
          <div class="authority-line"><span>${t.exerciseAlert}</span><button type="button" data-action="details">${icons.info} ${t.sourceDetails}</button></div>
          <div class="severity"><i aria-hidden="true">!</i><span>${t.severeWarning}</span><time>${t.updated}</time></div>
          <h1 id="alert-title">${t.leave}</h1><p class="lede">${t.summary}</p>

          ${journeyState === 'ROUTE_REVOKED' ? `
            <div class="warning-banner" role="alert">
              <strong>${t.routeRevokedNotice}</strong>
            </div>
          ` : ''}

          ${journeyState === 'NEAR_DESTINATION' ? `
            <div class="near-destination-advisory" role="region" aria-label="${t.nearDestinationPrompt}">
              <p>${icons.locate} ${t.nearDestinationPrompt}</p>
              <button class="primary-action is-success" type="button" data-action="arrival-open">
                ${t.confirmArrivalPrompt}
              </button>
            </div>
          ` : ''}

          ${ambiguousPlaces.length > 0 ? `
            <div class="ambiguous-places-panel" role="region" aria-label="${t.ambiguousPlacesTitle}">
              <p><strong>${t.ambiguousPlacesTitle}</strong></p>
              <div class="candidate-buttons" style="display: flex; gap: 8px; flex-wrap: wrap;">
                ${ambiguousPlaces.map((c) =>
                  `<button class="secondary-action" type="button" data-candidate-id="${escapeHtml(c.place_id)}">${escapeHtml(c.place_id)} (${escapeHtml(c.place_kind)})</button>`
                ).join('')}
              </div>
            </div>
          ` : ''}

          ${guidanceStatus === 'ERROR' ? `
            <div class="warning-banner" role="alert">
              <span>${escapeHtml(guidanceErrorMessage)}</span>
            </div>
          ` : ''}

          ${selectedDestination ? `
            <article class="destination">
              <div>
                <span class="destination-label">${t.destinationLabel}</span>
                <h2>${escapeHtml(selectedDestination.facility_name)}</h2>
                <p>${destinationCapacityText()}</p>
                ${availableDestinations.length > 1 ? `
                  <div style="display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap;">
                    ${availableDestinations.map(d => `
                      <button class="secondary-action ${d.facility_id === selectedDestination?.facility_id ? 'is-active' : ''}" style="font-size: 0.72rem; min-height: 1.8rem;" type="button" data-select-facility="${escapeHtml(d.facility_id)}">
                        ${escapeHtml(d.facility_name)}
                      </button>
                    `).join('')}
                  </div>
                ` : ''}
              </div>
              <div class="distance">
                <strong>${destinationDistanceText(selectedDestination).kmText}</strong>
                <span>${destinationDistanceText(selectedDestination).durationText}</span>
              </div>
            </article>
          ` : ''}

          ${routeStarted ? `
            <div class="journey-tracker" aria-label="${t.journeyTracking}">
              <div class="journey-header">
                <span><strong>${t.journeyTracking}</strong></span>
                <span class="journey-status-badge journey-status-badge--${journeyBadgeClass(journeyState)}">
                  ${journeyStateLabel(journeyState, t)}
                </span>
              </div>
              <div style="display: flex; justify-content: space-between; align-items: center; gap: 8px;">
                <small style="color: var(--color-muted);">
                  ${journeyTrackingDetail(journeyState, lastProximityEval, currentPosition, t)}
                </small>
                ${trackingButtonHtml(journeyState, t)}
              </div>
            </div>
          ` : ''}

          <button class="primary-action ${routeStarted ? 'is-success' : ''}" data-testid="start-route" type="button" data-action="route">
            ${icons.route}<span>${reservationPending ? 'Reserving...' : routeStarted ? t.routeActive : t.startRoute}</span>${icons.arrow}
          </button>
          <ol class="instructions"><li><span>1</span><p><strong>${t.instruction1Title}</strong> ${t.instruction1Body}</p></li><li><span>2</span><p>${t.instruction2}</p></li><li><span>3</span><p>${t.instruction3}</p></li></ol>
          <p class="grounding-line">${t.groundingLine}</p>
          <div class="quick-actions">
            <button type="button" data-action="directions">${icons.route}<span>${t.directions}</span></button>
            <button type="button" data-action="listen">${icons.volume}<span>${t.listen}</span></button>
            <button type="button" data-action="isl">${icons.info}<span>${t.isl}</span></button>
            <button type="button" data-action="voice-open">${icons.mic}<span>${t.askByVoice}</span></button>
          </div>
          <a class="rescue-action" data-testid="call-112" href="tel:112"><span>${t.trapped}</span><strong>${t.rescue}</strong></a>
        </section>
        <section class="map-surface" aria-label="${t.mapAria}">
          <div id="map-canvas"></div>
          <div class="map-loading" role="status">${mapTilted ? t.loadingTerrain : t.mapLoading}</div>
          <button class="map-help" type="button" data-action="assist-open">${icons.phone}<span>${t.callHelp}</span></button>
          <div class="map-controls"><div class="map-toolbar" aria-label="${t.mapTools}"><button class="${mapTilted ? 'is-active' : ''}" type="button" data-action="toggle-3d" aria-label="${t.map3d}" aria-pressed="${mapTilted}"><span class="map-toolbar__perspective-label">${t.map3d}</span><span class="map-toolbar__perspective-short" aria-hidden="true">3D</span></button><button type="button" data-action="recenter" aria-label="${t.myLocation}">${icons.locate}<span>${t.myLocation}</span></button><button class="${layersOpen ? 'is-active' : ''}" type="button" data-action="toggle-layers" aria-label="${t.mapLayers}" aria-expanded="${layersOpen}">${icons.info}<span>${t.mapLayers}</span></button></div>${layersOpen ? `<div class="layer-switcher" aria-label="${t.mapLayers}"><button class="zone-toggle zone-toggle--danger ${redZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-red-zones" aria-pressed="${redZonesVisible}"><i></i><span>${t.redZones}</span></button><button class="zone-toggle zone-toggle--relocation ${relocationZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-relocation-zones" aria-pressed="${relocationZonesVisible}"><i></i><span>${t.relocationZones}</span></button></div>` : ''}</div>
          <div class="map-key"><span class="${redZonesVisible ? '' : 'is-muted'}"><i class="hazard-key"></i>${t.redZone}</span><span><i class="route-key"></i>${t.approvedRoute}</span><span class="${relocationZonesVisible ? '' : 'is-muted'}"><i class="relocation-key"></i>${t.relocationZone}</span><span><i class="shelter-key"></i>${t.safeShelter}</span></div>
          <div class="map-disclaimer">${t.imagery} <a href="https://www.esri.com/" target="_blank" rel="noreferrer">© Esri</a> · ${t.buildingContext} · ${t.overlays}</div>
          <nav class="mobile-safety-dock mobile-safety-dock--voice" aria-label="${t.voicePrompt}"><button class="dock-action dock-action--voice dock-action--voice-primary ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-open" aria-expanded="${voiceOpen}"><span class="dock-icon" aria-hidden="true">${icons.mic}</span><span>${t.voicePrompt}</span></button></nav>
        </section>
      </main>
      <aside class="voice-console ${voiceOpen ? 'is-open' : ''}" role="dialog" aria-modal="false" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}>
        <div class="voice-head"><div><span>${t.assistant}</span><h2 id="voice-title">${t.askSituation}</h2></div><button class="icon-button" type="button" data-action="voice-close" aria-label="${t.closeAssistant}">${icons.close}</button></div>
        <div class="voice-stage ${voiceListening ? 'is-listening' : ''}"><div class="voice-orb" aria-hidden="true">${icons.mic}<i></i><i></i><i></i></div><div><strong>${voiceListening ? t.listening : commandPending ? t.checkingGuidance : t.ready}</strong><span>${t.speakNaturally}</span></div></div>
        ${voiceTranscript || commandPending || commandResponse !== words[language].voiceReady ? `<div class="command-result" aria-live="polite" aria-busy="${commandPending}"><span>${t.voiceResult}</span><p>${escapeHtml(commandResponse)}</p>${voiceTranscript ? `<small>${t.you}: ${escapeHtml(voiceTranscript)}</small>` : ''}${ambiguousPlaces.length > 0 ? `<div class="candidate-buttons" style="display: flex; gap: 6px; margin-top: 8px; flex-wrap: wrap;">${ambiguousPlaces.map((c) => `<button class="secondary-action" style="font-size: 0.72rem; min-height: 1.8rem;" type="button" data-candidate-id="${escapeHtml(c.place_id)}">${escapeHtml(c.place_id)} (${escapeHtml(c.place_kind)})</button>`).join('')}</div>` : ''}${commandPending ? `<div class="command-thinking"><i></i><i></i><i></i><span>${t.checkingExercise}</span></div>` : ''}</div>` : ''}
        ${commandError ? `<p class="command-error" role="alert">${escapeHtml(commandError)}</p>` : ''}
        <div class="voice-suggestions" aria-label="${t.suggestedQuestions}">${commandSuggestions.map((suggestion) => `<button type="button" data-command="${escapeHtml(suggestion)}">${escapeHtml(suggestion)}</button>`).join('')}</div>
        <form class="command-form" data-command-form><label for="command-input">${t.askText}</label><div><input id="command-input" name="command" autocomplete="off" placeholder="${t.askPlaceholder}" ${commandPending ? 'disabled' : ''}/><button type="submit" ${commandPending ? 'disabled' : ''}>${t.send}</button></div></form>
        <button class="listen-button ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-listen" aria-pressed="${voiceListening}">${icons.mic}<span>${voiceListening ? t.stopListening : t.startListening}</span></button>
        <p class="voice-boundary"><strong>${t.voiceBoundaryLabel}</strong> ${t.voiceBoundary}</p>
      </aside>
      ${directionsOpen ? `<aside class="side-sheet" aria-labelledby="directions-title"><div class="sheet-head"><div><span>${t.routeKicker}</span><h2 id="directions-title">${t.routeTitle}</h2></div><button class="icon-button" data-action="directions-close" aria-label="${t.closeDirections}">${icons.close}</button></div><ol><li><b>1</b><p>${t.routeStep1}<small>${t.routeStep1Note}</small></p></li><li><b>2</b><p>${t.routeStep2}<small>${t.routeStep2Note}</small></p></li><li><b>3</b><p>${t.routeStep3}<small>${t.routeStep3Note}</small></p></li></ol></aside>` : ''}
      ${detailsOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.sourceFreshness}</span><h2>${t.alertDetails}</h2></div><button class="icon-button" data-action="details-close" aria-label="${t.closeDetails}">${icons.close}</button></div><p>${t.demoNotice}</p><dl><div><dt>${t.authorityFormat}</dt><dd>NDMA SACHET / CAP (Source: SRCDEMO-1)</dd></div><div><dt>Package ID</dt><dd>${PACKAGE_ID} (Jurisdiction: ${JURISDICTION})</dd></div><div><dt>Freshness State</dt><dd>${guidanceFreshness}</dd></div><div><dt>Valid Dates</dt><dd>${getTodayYMD()} to ${getTomorrowYMD()}</dd></div><div><dt>${t.backend}</dt><dd>${runtimeCopy()}</dd></div></dl></dialog>` : ''}
      ${assistanceOpen ? `<dialog class="modal modal--critical" open><div class="sheet-head"><div><span>${t.emergencyAssistance}</span><h2>${t.callHelp}</h2></div><button class="icon-button" data-action="assist-close" aria-label="${t.close}">${icons.close}</button></div><p>${t.assistNotice}</p><div class="help-actions"><a class="primary-action" href="tel:112">${t.callRescue}</a><a class="primary-action" href="tel:112">${t.callAmbulance}</a><a class="primary-action" href="tel:112">${t.call112Now}</a></div></dialog>` : ''}
      ${arrivalOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.arrivalCheck}</span><h2>${t.arrivedSafely}</h2></div><button class="icon-button" data-action="arrival-close" aria-label="${t.closeArrival}">${icons.close}</button></div>${arrivalSuccess ? `<div class="success-message"><strong>${t.arrivalRecorded}</strong>${arrivalRecordedAt ? `<p><small>Recorded at: ${arrivalRecordedAt}</small></p>` : ''}</div>` : `<p>${t.confirmParty}</p><div class="stepper"><button type="button" data-action="party-minus" aria-label="${t.decreaseParty}">-</button><strong>${partySize} ${partySize === 1 ? t.person : t.people}</strong><button type="button" data-action="party-plus" aria-label="${t.increaseParty}">+</button></div>${arrivalError ? `<p class="command-error" role="alert" style="margin-block: 0.5rem;">${escapeHtml(arrivalError)}</p>` : ''}<div class="help-actions" style="margin-top: 1rem;"><button class="primary-action is-success" type="button" data-action="arrival-yes" ${arrivalPending ? 'disabled' : ''}>${arrivalPending ? 'Confirming...' : t.confirmArrivalPrompt}</button><button class="secondary-action" type="button" data-action="arrival-no">${t.callHelp}</button></div>`}</dialog>` : ''}
      ${audioOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.listen}</span><h2>${lastApprovedAudio && isAudioValidForReplayCheck(lastApprovedAudio) ? t.listen : t.approvedAudioUnavailable}</h2></div><button class="icon-button" data-action="audio-close" aria-label="${t.close}">${icons.close}</button></div>${lastApprovedAudio && isAudioValidForReplayCheck(lastApprovedAudio) ? `<p>${t.summary}</p><div class="help-actions"><button class="primary-action" type="button" data-action="audio-play-modal">${icons.volume} ${t.tapToPlay}</button></div>` : `<p>${t.approvedAudioUnavailable}</p><p>${t.summary}</p>`}</dialog>` : ''}
      ${islOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.isl}</span><h2>${t.islTitle}</h2></div><button class="icon-button" data-action="isl-close" aria-label="${t.close}">${icons.close}</button></div><p>${t.islPending}</p><p>${t.summary}</p></dialog>` : ''}
      <div class="toast" role="status" aria-live="polite" hidden></div>
    </div>`;
  hasRendered = true;
  bindInteractions(); void initMap(mapRenderVersion);
}

function mapColor(token: string) { return getComputedStyle(document.documentElement).getPropertyValue(token).trim(); }
async function initMap(renderVersion: number) {
  const container = document.querySelector<HTMLElement>('#map-canvas'); if (!container) return;
  const { Map, Popup } = await import('maplibre-gl');
  if (renderVersion !== mapRenderVersion || !document.body.contains(container)) return;
  const t = words[language];
  const mapZoom = mapTilted ? Math.max(perspectiveCamera?.zoom || 0, 15.5) : (perspectiveCamera?.zoom ?? 13.4);
  map = new Map({ container, center: perspectiveCamera?.center || [76.112, 11.562], zoom: mapZoom, pitch: mapTilted ? 65 : 0, bearing: mapTilted ? -18 : 0, maxPitch: 75, dragRotate: true, pitchWithRotate: true, attributionControl: false, style: { version: 8, terrain: mapTilted ? { source: 'elevation', exaggeration: 1 } : undefined, light: { anchor: 'viewport', color: 'hsl(210, 55%, 93%)', intensity: 0.42, position: [1.5, 210, 30] }, sources: {
    basemap: { type: 'raster', tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'], tileSize: 256, attribution: 'Imagery © Esri' },
    elevation: { type: 'raster-dem', tiles: ['https://s3.amazonaws.com/elevation-tiles-prod/terrarium/{z}/{x}/{y}.png'], tileSize: 256, encoding: 'terrarium', maxzoom: 15, attribution: '<a href="https://registry.opendata.aws/terrain-tiles/" target="_blank" rel="noreferrer">Terrain Tiles</a>' },
    hazard: { type: 'geojson', data: { type: 'Feature', geometry: { type: 'Polygon', coordinates: mapData.hazard }, properties: { label: t.hazardLabel, detail: t.hazardDetail } } },
    relocation: { type: 'geojson', data: { type: 'FeatureCollection', features: [
      { type: 'Feature', geometry: { type: 'Polygon', coordinates: mapData.relocationZones[0] }, properties: { label: t.relocation1Label, detail: t.relocation1Detail } },
      { type: 'Feature', geometry: { type: 'Polygon', coordinates: mapData.relocationZones[1] }, properties: { label: t.relocation2Label, detail: t.relocation2Detail } },
    ] } },
    route: { type: 'geojson', data: { type: 'Feature', geometry: { type: 'LineString', coordinates: mapData.route }, properties: {} } },
    roads: { type: 'geojson', data: { type: 'FeatureCollection', features: mapData.roads.map((coordinates) => ({ type: 'Feature', geometry: { type: 'LineString', coordinates }, properties: {} })) } },
    places: { type: 'geojson', data: { type: 'FeatureCollection', features: [{ type: 'Feature', geometry: { type: 'Point', coordinates: mapData.user }, properties: { label: t.userMapLabel, kind: 'user' } }, { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.shelter }, properties: { label: t.shelterMapLabel, kind: 'shelter' } }, { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.hospital }, properties: { label: t.hospitalMapLabel, kind: 'hospital' } }, ...(deviceLocation ? [{ type: 'Feature' as const, geometry: { type: 'Point' as const, coordinates: deviceLocation }, properties: { label: t.deviceMapLabel, kind: 'device' } }] : [])] } },
  }, layers: [
    { id: 'background', type: 'background', paint: { 'background-color': mapColor('--map-color-surface') } },
    { id: 'basemap', type: 'raster', source: 'basemap', paint: { 'raster-opacity': 0.92, 'raster-saturation': -0.12, 'raster-contrast': 0.14, 'raster-brightness-max': 0.82 } },
    { id: 'roads', type: 'line', source: 'roads', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 1.5, 'line-opacity': 0.32 } },
    { id: 'hazard-band', type: 'line', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-danger'), 'line-width': 22, 'line-blur': 7, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
    { id: 'hazard-fill', type: 'fill', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'fill-color': mapColor('--map-color-danger'), 'fill-opacity': 0, 'fill-opacity-transition': { duration: motionDuration() } } },
    { id: 'hazard-edge', type: 'line', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-danger'), 'line-width': 3.5, 'line-opacity': 0, 'line-dasharray': [1, 1.4], 'line-opacity-transition': { duration: motionDuration() } } },
    { id: 'relocation-band', type: 'line', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-success'), 'line-width': 18, 'line-blur': 6, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
    { id: 'relocation-fill', type: 'fill', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'fill-color': mapColor('--map-color-success'), 'fill-opacity': 0, 'fill-opacity-transition': { duration: motionDuration() } } },
    { id: 'relocation-edge', type: 'line', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-success'), 'line-width': 3, 'line-opacity': 0, 'line-dasharray': [1.6, 1], 'line-opacity-transition': { duration: motionDuration() } } },
    { id: 'route-casing', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 9, 'line-opacity': 0.9 } },
    { id: 'approved-route', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-accent'), 'line-width': 5, 'line-opacity': 0.98 } },
    { id: 'route-motion', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 2, 'line-opacity': routeStarted ? 0.9 : 0, 'line-dasharray': [0.2, 2.4, 1.6] } },
    { id: 'shelter-pulse', type: 'circle', source: 'places', filter: ['==', ['get', 'kind'], 'shelter'], paint: { 'circle-radius': 15, 'circle-color': mapColor('--map-color-success'), 'circle-opacity': 0.24 } },
    { id: 'hospital-pulse', type: 'circle', source: 'places', filter: ['==', ['get', 'kind'], 'hospital'], layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'circle-radius': 16, 'circle-color': mapColor('--map-color-paper'), 'circle-opacity': 0.28 } },
    { id: 'device-pulse', type: 'circle', source: 'places', filter: ['==', ['get', 'kind'], 'device'], paint: { 'circle-radius': 17, 'circle-color': mapColor('--map-color-accent'), 'circle-opacity': 0 } },
    { id: 'place-points', type: 'circle', source: 'places', filter: ['!=', ['get', 'kind'], 'hospital'], paint: { 'circle-radius': ['case', ['==', ['get', 'kind'], 'device'], 7, 8], 'circle-color': ['case', ['==', ['get', 'kind'], 'device'], mapColor('--map-color-paper'), mapColor('--map-color-accent')], 'circle-stroke-color': ['case', ['==', ['get', 'kind'], 'device'], mapColor('--map-color-accent'), mapColor('--map-color-paper')], 'circle-stroke-width': 3 } },
    { id: 'hospital-point', type: 'circle', source: 'places', filter: ['==', ['get', 'kind'], 'hospital'], layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'circle-radius': 8, 'circle-color': mapColor('--map-color-paper'), 'circle-stroke-color': mapColor('--map-color-success'), 'circle-stroke-width': 3 } },
    { id: 'place-labels', type: 'symbol', source: 'places', filter: ['all', ['!=', ['get', 'kind'], 'device'], ['!=', ['get', 'kind'], 'hospital']], layout: { 'text-field': ['get', 'label'], 'text-size': 13, 'text-offset': [0, 1.5], 'text-anchor': 'top' }, paint: { 'text-color': mapColor('--map-color-paper'), 'text-halo-color': mapColor('--map-color-surface'), 'text-halo-width': 2 } },
    { id: 'hospital-label', type: 'symbol', source: 'places', filter: ['==', ['get', 'kind'], 'hospital'], layout: { visibility: relocationZonesVisible ? 'visible' : 'none', 'text-field': ['get', 'label'], 'text-size': 13, 'text-offset': [0, 1.5], 'text-anchor': 'top' }, paint: { 'text-color': mapColor('--map-color-paper'), 'text-halo-color': mapColor('--map-color-surface'), 'text-halo-width': 2 } },
  ] } });
  perspectiveCamera = null;
  map.on('idle', () => {
    if (renderVersion === mapRenderVersion) document.querySelector<HTMLElement>('.map-loading')?.setAttribute('hidden', '');
  });
  map.once('load', () => {
    if (renderVersion !== mapRenderVersion) return;
    document.querySelector<HTMLElement>('.map-loading')?.setAttribute('hidden', '');
    map?.resize();
    if (routeStarted) focusRoute();
    revealMapLayers();
    applyQueuedVoiceActions();
    startMapAnimation();
    map?.on('mouseenter', 'place-points', () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
    map?.on('mouseleave', 'place-points', () => { if (map) map.getCanvas().style.cursor = ''; });
    ([['hazard-fill', 0.34, 0.44], ['relocation-fill', 0.26, 0.36]] as const).forEach(([layer, restingOpacity, hoverOpacity]) => {
      map?.on('mouseenter', layer, () => {
        if (!map) return;
        map.getCanvas().style.cursor = 'pointer';
        map.setPaintProperty(layer, 'fill-opacity', hoverOpacity);
        map.setPaintProperty(layer.replace('fill', 'edge'), 'line-width', 3.5);
      });
      map?.on('mouseleave', layer, () => {
        if (!map) return;
        map.getCanvas().style.cursor = '';
        map.setPaintProperty(layer, 'fill-opacity', restingOpacity);
        map.setPaintProperty(layer.replace('fill', 'edge'), 'line-width', 2.5);
      });
    });
    map?.on('click', 'place-points', (event) => {
      const feature = event.features?.[0];
      const coordinates = feature?.geometry.type === 'Point' ? (feature.geometry.coordinates as [number, number]) : null;
      if (!map || !coordinates) return;
      new Popup({ offset: 14, closeButton: false }).setLngLat(coordinates).setText(String(feature?.properties?.label || t.mapLocation)).addTo(map);
    });
    ['hazard-fill', 'relocation-fill'].forEach((layer) => map?.on('click', layer, (event) => {
      const feature = event.features?.[0];
      const center = event.lngLat;
      if (!map || !feature) return;
      new Popup({ offset: 8 }).setLngLat(center).setHTML(`<strong>${escapeHtml(String(feature.properties?.label || t.exerciseZone))}</strong><p>${escapeHtml(String(feature.properties?.detail || t.overlayDetail))}</p>`).addTo(map);
    }));
  });
}

function motionDuration() { return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 900; }
function syncPerspectiveControl() {
  const control = document.querySelector<HTMLButtonElement>('[data-action="toggle-3d"]');
  control?.classList.toggle('is-active', mapTilted);
  control?.setAttribute('aria-pressed', String(mapTilted));
}
function toggleMapPerspective() {
  if (!map) return;
  perspectiveCamera = { center: map.getCenter().toArray(), zoom: map.getZoom() };
  mapTilted = !mapTilted;
  render();
}
function recenterMap() {
  if (!deviceLocation) { requestLocation(); return; }
  mapTilted = false;
  map?.setTerrain(null);
  map?.easeTo({ center: deviceLocation, zoom: 15, pitch: 0, bearing: 0, duration: motionDuration() });
  syncPerspectiveControl();
}
function focusRoute() {
  map?.fitBounds(mapData.routeBounds as [[number, number], [number, number]], {
    padding: window.innerWidth < 768 ? { top: 140, bottom: 290, left: 40, right: 40 } : 90,
    pitch: mapTilted ? 65 : 0,
    bearing: mapTilted ? -18 : 0,
    duration: motionDuration(),
  });
}
function revealMapLayers() {
  if (!map) return;
  const show = () => {
    if (redZonesVisible) { map?.setPaintProperty('hazard-band', 'line-opacity', 0.22); map?.setPaintProperty('hazard-fill', 'fill-opacity', 0.22); map?.setPaintProperty('hazard-edge', 'line-opacity', 0.96); }
    if (relocationZonesVisible) { map?.setPaintProperty('relocation-band', 'line-opacity', 0.18); map?.setPaintProperty('relocation-fill', 'fill-opacity', 0.15); map?.setPaintProperty('relocation-edge', 'line-opacity', 0.9); }
    if (deviceLocation) map?.setPaintProperty('device-pulse', 'circle-opacity', 0.2);
  };
  if (motionDuration() === 0) show(); else requestAnimationFrame(show);
}
function startMapAnimation() {
  if (!map || motionDuration() === 0 || (!routeStarted && !redZonesVisible && !relocationZonesVisible && !deviceLocation)) return;
  const dashFrames = [[0.2, 2.4, 1.6], [0.7, 2.4, 1.1], [1.2, 2.4, 0.6], [1.7, 2.4, 0.1]];
  let frame = 0;
  const animate = () => {
    if (!map || !map.isStyleLoaded()) return;
    const cycle = Math.floor(frame / 12) % dashFrames.length;
    const pulse = (Math.sin(frame / 10) + 1) / 2;
    if (routeStarted) map.setPaintProperty('route-motion', 'line-dasharray', dashFrames[cycle]);
    if (redZonesVisible) { map.setPaintProperty('hazard-edge', 'line-dasharray', dashFrames[cycle]); map.setPaintProperty('hazard-band', 'line-opacity', 0.13 + pulse * 0.15); }
    if (relocationZonesVisible) { map.setPaintProperty('relocation-edge', 'line-dasharray', dashFrames[(cycle + 2) % dashFrames.length]); map.setPaintProperty('relocation-band', 'line-opacity', 0.1 + pulse * 0.1); }
    if (deviceLocation) { map.setPaintProperty('device-pulse', 'circle-radius', 14 + pulse * 10); map.setPaintProperty('device-pulse', 'circle-opacity', 0.08 + (1 - pulse) * 0.18); }
    frame += 1;
    mapAnimationFrame = requestAnimationFrame(animate);
  };
  mapAnimationFrame = requestAnimationFrame(animate);
}

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((b) =>
    b.addEventListener('click', () => {
      const nextLanguage = b.dataset.language as Language;
      language = nextLanguage;
      try { localStorage.setItem('sthira-language', language); } catch {}
      commandSuggestions = [...words[language].voiceCommands];
      commandResponse = words[language].voiceReady;
      commandError = '';
      voiceFeedbackKey = 'micPrivacy';
      audioGuard.invalidate();
      lastApprovedAudio = undefined;
      render();
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-action="voice-open"]').forEach((b) =>
    b.addEventListener('click', () => {
      voiceOpen = true;
      render();
    })
  );

  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => {
    cancelRecording();
    voiceOpen = false;
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => {
    void toggleLocalRecording();
  });

  document.querySelectorAll<HTMLButtonElement>('[data-command]').forEach((b) =>
    b.addEventListener('click', () => {
      const cmd = b.dataset.command || '';
      void sendVoiceOrText({ kind: 'transcript', text: cmd });
    })
  );

  document.querySelector<HTMLFormElement>('[data-command-form]')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const input = (e.currentTarget as HTMLFormElement).elements.namedItem('command') as HTMLInputElement | null;
    const text = input?.value || '';
    if (text.trim()) {
      if (input) input.value = '';
      void sendVoiceOrText({ kind: 'transcript', text });
    }
  });

  document.querySelectorAll<HTMLButtonElement>('[data-candidate-id]').forEach((b) =>
    b.addEventListener('click', () => {
      const candId = b.dataset.candidateId;
      if (candId) {
        void selectCandidatePlace(candId);
      }
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-select-facility]').forEach((b) =>
    b.addEventListener('click', () => {
      const facId = b.dataset.selectFacility;
      const found = availableDestinations.find((d) => d.facility_id === facId);
      if (found) {
        selectedDestination = found;
        render();
      }
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-action="route"], [data-action="start-route"]').forEach((b) =>
    b.addEventListener('click', () => {
      void startRouteReservation();
    })
  );

  document.querySelector<HTMLButtonElement>('[data-action="toggle-3d"]')?.addEventListener('click', toggleMapPerspective);
  document.querySelector<HTMLButtonElement>('[data-action="recenter"]')?.addEventListener('click', recenterMap);
  document.querySelector<HTMLButtonElement>('[data-action="toggle-layers"]')?.addEventListener('click', () => {
    layersOpen = !layersOpen;
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="toggle-red-zones"]')?.addEventListener('click', () => {
    redZonesVisible = !redZonesVisible;
    pendingZoneFocus = redZonesVisible ? 'RED' : null;
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="toggle-relocation-zones"]')?.addEventListener('click', () => {
    relocationZonesVisible = !relocationZonesVisible;
    pendingZoneFocus = relocationZonesVisible ? 'RELOCATION' : null;
    render();
  });

  document.querySelectorAll<HTMLButtonElement>('[data-action="directions"]').forEach((b) =>
    b.addEventListener('click', () => {
      directionsOpen = true;
      render();
    })
  );
  document.querySelector<HTMLButtonElement>('[data-action="directions-close"]')?.addEventListener('click', () => {
    directionsOpen = false;
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="details"]')?.addEventListener('click', () => {
    detailsOpen = true;
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="details-close"]')?.addEventListener('click', () => {
    detailsOpen = false;
    render();
  });

  document.querySelectorAll<HTMLButtonElement>('[data-action="assist-open"]').forEach((b) =>
    b.addEventListener('click', () => {
      assistanceOpen = true;
      render();
    })
  );
  document.querySelector<HTMLButtonElement>('[data-action="assist-close"]')?.addEventListener('click', () => {
    assistanceOpen = false;
    render();
  });

  document.querySelectorAll<HTMLAnchorElement>('a[href="tel:112"]').forEach((a) =>
    a.addEventListener('click', (e) => {
      if (!assistanceOpen) {
        e.preventDefault();
        assistanceOpen = true;
        render();
      } else {
        triggerEmergencyDial({
          number: '112',
          event: e,
          documentRef: document,
          windowRef: window,
        });
      }
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-action="listen"]').forEach((b) =>
    b.addEventListener('click', () => {
      if (lastApprovedAudio && isAudioValidForReplayCheck(lastApprovedAudio)) {
        void verifyAndPlayAudio(lastApprovedAudio);
      } else {
        audioOpen = true;
        render();
      }
    })
  );
  document.querySelector<HTMLButtonElement>('[data-action="audio-close"]')?.addEventListener('click', () => {
    audioOpen = false;
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="audio-play-modal"]')?.addEventListener('click', () => {
    if (lastApprovedAudio && isAudioValidForReplayCheck(lastApprovedAudio)) {
      void verifyAndPlayAudio(lastApprovedAudio);
    }
  });

  document.querySelector<HTMLButtonElement>('[data-action="isl"]')?.addEventListener('click', () => {
    islOpen = true;
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="isl-close"]')?.addEventListener('click', () => {
    islOpen = false;
    render();
  });

  document.querySelectorAll<HTMLButtonElement>('[data-action="arrival-open"]').forEach((b) =>
    b.addEventListener('click', () => {
      arrivalOpen = true;
      render();
    })
  );
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => {
    arrivalOpen = false;
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="party-minus"]')?.addEventListener('click', () => {
    partySize = Math.max(1, partySize - 1);
    render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="party-plus"]')?.addEventListener('click', () => {
    partySize = Math.min(10, partySize + 1);
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="arrival-yes"]')?.addEventListener('click', () => {
    void confirmArrival();
  });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-no"]')?.addEventListener('click', () => {
    arrivalOpen = false;
    assistanceOpen = true;
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="start-tracking"]')?.addEventListener('click', () => {
    startTracking();
  });
  document.querySelector<HTMLButtonElement>('[data-action="stop-tracking"]')?.addEventListener('click', () => {
    stopTracking();
  });
}

// Initial bootstrap
render();
void initSession();
void checkRuntime();
void queryGuidanceDestinations();

document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    cancelRecording();
    stopTracking();
    activeRequestId++;
    commandPending = false;
  }
  render();
});

window.addEventListener('online', () => {
  runtime = 'checking';
  void checkRuntime();
  void queryGuidanceDestinations();
});

window.addEventListener('offline', () => {
  runtime = 'offline';
  audioGuard.invalidate();
  lastApprovedAudio = undefined;
  guidanceFreshness = 'UNAVAILABLE';
  currentDataVersion = 'UNAVAILABLE';
  render();
});

if ('serviceWorker' in navigator) void navigator.serviceWorker.register('/sw.js').catch(() => undefined);
