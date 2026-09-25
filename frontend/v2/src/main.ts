import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Map, Popup } from 'maplibre-gl';
import mapData from './mapData.json';
import scenario from './scenario.json';
import { words, type Language } from './i18n';
import { executeMapActions } from './mapActions';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
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
import { renderOperatorView } from './operator';
import { triggerEmergencyDial, recordEmergencyAudit } from './emergency';

type RuntimeState = 'checking' | 'demo' | 'blocked' | 'offline';
type AudioMetadata = {
  audio_b64: string;
  content_type?: string;
  byte_size?: number;
  checksum_sha256?: string;
  source_id?: string;
  source_version?: number;
  data_version?: string;
  template_key?: string;
  template_version?: number;
  language?: string;
  settings?: {
    sample_rate_hz?: number;
    channels?: number;
    bit_depth?: number;
    codec?: string;
  };
};
type ChatMessage = { role: 'USER' | 'ASSISTANT'; text: string; audio?: AudioMetadata };
type AmbiguousCandidate = { place_id: string; place_kind: string };
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
  coordinates?: [number, number];
};

const icons: Record<string, string> = {
  arrow: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12h14M13 6l6 6-6 6"/></svg>',
  mic: '<svg viewBox="0 0 24 24" aria-hidden="true"><rect x="9" y="3" width="6" height="12" rx="3"/><path d="M5 11a7 7 0 0 0 14 0M12 18v3M9 21h6"/></svg>',
  locate: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3"/></svg>',
  route: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="6" cy="18" r="2"/><circle cx="18" cy="6" r="2"/><path d="M8 18h3a3 3 0 0 0 3-3V9a3 3 0 0 1 3-3"/></svg>',
  volume: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M11 5 6 9H3v6h3l5 4V5ZM15 9a5 5 0 0 1 0 6M18 6a9 9 0 0 1 0 12"/></svg>',
  close: '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="m6 6 12 12M18 6 6 18"/></svg>',
  info: '<svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7h.01"/></svg>',
};

let language: Language = 'EN';
let runtime: RuntimeState = navigator.onLine ? 'checking' : 'offline';
let runtimeDetail: 'blocked' | 'responding' | 'disconnected' = 'blocked';
let map: Map | null = null;
let mapAnimationFrame: number | null = null;
let hasRendered = false;
let redZonesVisible = false;
let relocationZonesVisible = false;
let pendingZoneFocus: 'RED' | 'RELOCATION' | null = null;
let voiceOpen = false;
let voiceListening = false;
let voiceTranscript = '';
let voiceFeedbackKey: Exclude<keyof typeof words.EN, 'suggestions'> = 'micPrivacy';
let mediaRecorder: MediaRecorder | null = null;
let mediaStream: MediaStream | null = null;
let recordingStartedAt = 0;
let chatPending = false;
let chatError = '';
let chatSuggestions: string[] = [...words.EN.suggestions];
let chatMessages: ChatMessage[] = [{ role: 'ASSISTANT', text: words.EN.welcome }];

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

let sessionToken: string | null = sessionStorage.getItem('sthira_session_token');
let sessionId: string = sessionStorage.getItem('sthira_session_id') || ('SES-' + Math.random().toString(36).slice(2, 10));
let activeReservationId: string | null = sessionStorage.getItem('sthira_reservation_id');
let activeStayId: string | null = sessionStorage.getItem('sthira_stay_id');

let routeStarted = false;
let directionsOpen = false;
let detailsOpen = false;
let arrivalOpen = false;
let assistanceOpen = false;
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

// R04: Consent-based foreground journey tracking state
let journeyState: JourneyState = 'NOT_STARTED';
let currentPosition: PositionReading | null = null;
let lastProximityEval: ProximityEvaluation | null = null;
let geolocationWatchId: number | null = null;
let tabInBackground = false;
let lastBackgroundTime: number | null = null;
let simulationActive = false;
let simulationNote = '';
let locationErrorMessage = '';

function getDestinationTarget(): DestinationTarget {
  return {
    id: selectedDestination ? selectedDestination.facility_id : EXERCISE_CONFIG.facility_id,
    name: selectedDestination ? selectedDestination.facility_name : 'Safe Shelter',
    longitude: mapData.shelter[0],
    latitude: mapData.shelter[1],
  };
}

function journeyBadgeClass(state: JourneyState): string {
  switch (state) {
    case 'TRACKING': return 'tracking';
    case 'NEAR_DESTINATION': return 'near';
    case 'ARRIVAL_REPORTED': return 'near';
    case 'ROUTE_REVOKED': return 'revoked';
    case 'LOCATION_UNAVAILABLE': return 'unavailable';
    case 'PAUSED': return 'paused';
    default: return 'paused';
  }
}

type TranslationWords = (typeof words)[Language];

function journeyStateLabel(state: JourneyState, t: TranslationWords): string {
  switch (state) {
    case 'TRACKING': return t.trackingActive;
    case 'NEAR_DESTINATION': return t.trackingNear;
    case 'ARRIVAL_REPORTED': return t.arrivalRecorded;
    case 'ROUTE_REVOKED': return 'Route Revoked';
    case 'LOCATION_UNAVAILABLE': return 'GPS Unavailable';
    case 'PAUSED': return t.trackingPaused;
    default: return 'Ready to Track';
  }
}

function journeyTrackingDetail(
  state: JourneyState,
  evalResult: ProximityEvaluation | null,
  pos: PositionReading | null,
  t: TranslationWords
): string {
  if (state === 'ROUTE_REVOKED') return t.routeRevokedNotice;
  if (state === 'LOCATION_UNAVAILABLE') return locationErrorMessage || t.locationUnavailable;
  if (state === 'ARRIVAL_REPORTED') return t.arrivalRecorded;
  if (state === 'NOT_STARTED') return 'Consent-based GPS tracking is off. Start tracking for arrival assistance.';
  if (state === 'PAUSED') return 'GPS tracking paused by citizen.';

  if (evalResult && pos) {
    const km = (evalResult.distanceMeters / 1000).toFixed(1);
    const acc = Math.round(pos.accuracyMeters);
    if (evalResult.isStale) return `Signal stale (>30s old) · Distance ~${km} km`;
    if (!evalResult.isAccurateEnough) return `Signal uncertain (±${acc}m) · Distance ~${km} km`;
    if (evalResult.isNear) return `Within ${evalResult.distanceMeters}m of shelter (accuracy ±${acc}m)`;
    return `Distance: ${km} km remaining (accuracy ±${acc}m)`;
  }
  return 'Acquiring GPS fix...';
}

function trackingButtonHtml(state: JourneyState, t: TranslationWords): string {
  if (state === 'TRACKING' || state === 'NEAR_DESTINATION') {
    return `<button class="secondary-action" style="min-height: 2.25rem; font-size: 0.75rem;" type="button" data-action="stop-tracking">${t.stopJourneyTracking}</button>`;
  }
  if (state === 'NOT_STARTED' || state === 'PAUSED' || state === 'LOCATION_UNAVAILABLE') {
    return `<button class="secondary-action" style="min-height: 2.25rem; font-size: 0.75rem;" type="button" data-action="start-tracking">${t.startJourneyTracking}</button>`;
  }
  return '';
}

function speechLanguageTag(lang: Language): string {
  switch (lang) {
    case 'ML': return 'ml-IN';
    case 'HI': return 'hi-IN';
    default: return 'en-IN';
  }
}

function runtimeCopy() {
  const t = words[language];
  if (runtime === 'offline') return t.offline;
  if (runtime === 'blocked' || runtimeDetail === 'blocked') return t.blocked;
  if (runtimeDetail === 'disconnected') return t.localDisconnected;
  if (runtime === 'demo') return t.responding;
  return t.checking;
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[character]!);
}

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

async function reconcileStayState(): Promise<void> {
  const savedStayId = sessionStorage.getItem('sthira_stay_id');
  if (!savedStayId) return;

  try {
    const res = await fetch(`/api/v3/reservations/${encodeURIComponent(savedStayId)}`, {
      method: 'GET',
      headers: authHeaders(),
    });
    if (res.ok) {
      const body = await res.json() as { data?: { stay_id?: string; state?: string; facility_id?: string } };
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
    // Session optional for public preview reads; fallback gracefully
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
      const data = await res.json() as { data?: { token?: string; session_id?: string } };
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
    // Session optional for public preview reads; fallback gracefully
  }
}

async function checkRuntime(): Promise<void> {
  if (!navigator.onLine) {
    runtime = 'offline';
    render();
    return;
  }
  try {
    const readyRes = await fetch('/health/ready');
    if (readyRes.ok) {
      runtime = 'demo';
      runtimeDetail = 'responding';
    } else if (readyRes.status === 503) {
      runtime = 'blocked';
      runtimeDetail = 'blocked';
    } else {
      runtime = 'blocked';
      runtimeDetail = 'disconnected';
    }
  } catch {
    runtime = 'demo';
    runtimeDetail = 'disconnected';
  }
  render();
}

function destinationDistanceText(d: DestinationChoice | null): { kmText: string; durationText: string } {
  if (!d) return { kmText: '—', durationText: '—' };
  if (d.is_illustrative) return { kmText: '2.8 km (est.)', durationText: '35 min' };
  if (currentPosition) {
    const distM = computeDistanceMeters(currentPosition.longitude, currentPosition.latitude, mapData.shelter[0], mapData.shelter[1]);
    const km = (distM / 1000).toFixed(1);
    const mins = Math.max(1, Math.round(distM / 80));
    return { kmText: `${km} km`, durationText: `${mins} min` };
  }
  return { kmText: '—', durationText: '—' };
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
      const envelope = await res.json() as {
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
        autoplayBlockedAudio = null;
        availableDestinations = [];
        selectedDestination = null;
        isIllustrativePreview = false;
        render();
        return;
      }
      guidanceFreshness = 'CURRENT';
      currentDataVersion = envelope.data_version || 'v1';
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
      autoplayBlockedAudio = null;
      availableDestinations = [];
      selectedDestination = null;
      guidanceErrorMessage = `Authority returned HTTP ${res.status}. Guidance unavailable.`;
    }
  } catch {
    isIllustrativePreview = false;
    guidanceStatus = 'ERROR';
    guidanceFreshness = 'UNAVAILABLE';
    currentDataVersion = 'UNAVAILABLE';
    autoplayBlockedAudio = null;
    availableDestinations = [];
    selectedDestination = null;
    guidanceErrorMessage = 'Network error: could not connect to guidance service.';
  }
  render();
}

async function selectCandidatePlace(candId: string) {
  ambiguousPlaces = [];
  chatMessages.push({ role: 'USER', text: `Selected location: ${candId}` });
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
      // Ambiguous place: server returns candidates to choose from
      const errData = await res.json() as { errors?: Array<{ code: string; message: string; details?: { candidates?: AmbiguousCandidate[] } }> };
      const candidates = errData.errors?.[0]?.details?.candidates;
      if (candidates && candidates.length > 0) {
        ambiguousPlaces = candidates;
        chatMessages.push({ role: 'ASSISTANT', text: `Multiple locations match "${query}". Please choose one:` });
        render();
        return;
      }
    }
    if (res.ok) {
      ambiguousPlaces = [];
      const data = await res.json() as { data?: { place_id: string; place_kind: string } };
      if (data.data?.place_id) {
        const pId = data.data.place_id;
        chatMessages.push({ role: 'ASSISTANT', text: `Resolved location: ${pId} (${data.data.place_kind || 'place'})` });
        // Bound candidate selection to next guidance lookup
        await queryGuidanceDestinations();
        render();
      }
    } else if (res.status === 404) {
      ambiguousPlaces = [];
      chatMessages.push({ role: 'ASSISTANT', text: `Location "${query}" not found in jurisdiction ${JURISDICTION}.` });
      render();
    }
  } catch {
    chatMessages.push({ role: 'ASSISTANT', text: `Place lookup failed for "${query}".` });
    render();
  }
}

function render() {
  if (window.location.hash === '#operator') {
    if (mapAnimationFrame !== null) cancelAnimationFrame(mapAnimationFrame);
    mapAnimationFrame = null;
    map?.remove();
    map = null;
    const app = document.querySelector<HTMLDivElement>('#app');
    if (app) renderOperatorView(app);
    return;
  }

  const t = words[language];
  if (mapAnimationFrame !== null) cancelAnimationFrame(mapAnimationFrame);
  mapAnimationFrame = null;
  map?.remove();
  document.documentElement.lang = language === 'ML' ? 'ml' : language === 'HI' ? 'hi' : 'en';

  const destinationCapacityText = selectedDestination
    ? (selectedDestination.capacity_known
        ? (selectedDestination.free !== null && selectedDestination.free > 0
            ? `${selectedDestination.free} spaces available`
            : t.capacityFull)
        : t.capacityUnknown)
    : t.capacityUnknown;

  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell ${hasRendered ? '' : 'is-entering'}">
      <header class="topbar">
        <a class="brand" href="/" aria-label="${t.brandHome}">
          <span class="brand-mark">സ്</span>
          <span><strong>Sthira</strong><small>${t.tagline}</small></span>
        </a>
        <div class="system-state system-state--${runtime}">
          <i></i><span>${runtimeCopy()}</span>
        </div>
        <div class="top-actions">
          <a href="#operator" class="secondary-action" style="min-height: 2.25rem; padding: 0.25rem 0.6rem; font-size: var(--text-xs); text-decoration: none; border-radius: var(--radius-pill);" title="Operator Portal">Operator</a>
          <button class="voice-launch ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-open" aria-expanded="${voiceOpen}">
            ${icons.mic}<span>${t.voice}</span>
          </button>
          <div class="language-switcher" role="group" aria-label="${t.chooseLanguage}">
            ${(['EN', 'ML', 'HI'] as Language[]).map((code) =>
              `<button class="${language === code ? 'is-active' : ''}" type="button" data-language="${code}" aria-pressed="${language === code}">${code}</button>`
            ).join('')}
          </div>
        </div>
      </header>

      <main class="map-workspace">
        <section class="guidance-panel" data-testid="emergency-card" aria-labelledby="alert-title">
          <div class="authority-line">
            <span>${t.exerciseAlert}</span>
            <button type="button" data-action="details">${icons.info} ${t.sourceDetails}</button>
          </div>
          <div class="severity">
            <i aria-hidden="true">!</i><span>${t.severeWarning}</span>
            <time>${t.updated}</time>
          </div>
          <h1 id="alert-title">${t.leave}</h1>
          <p class="lede">${t.summary}</p>

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

          ${journeyState === 'ROUTE_REVOKED' ? `
            <div class="warning-banner" role="alert">
              <strong>${t.routeRevokedNotice}</strong>
            </div>
          ` : ''}

          ${journeyState === 'LOCATION_UNAVAILABLE' ? `
            <div class="warning-banner" role="alert">
              <span>${escapeHtml(locationErrorMessage || t.locationUnavailable)}</span>
            </div>
          ` : ''}

          ${tabInBackground ? `
            <div class="warning-banner" role="status">
              <span>${t.tabBackgroundNotice}</span>
            </div>
          ` : ''}

          ${journeyState === 'NEAR_DESTINATION' ? `
            <div class="near-destination-advisory" role="region" aria-label="${t.nearDestinationPrompt}">
              <p>${icons.locate} ${t.nearDestinationPrompt}</p>
              <button class="primary-action is-success" type="button" data-action="arrival-confirm-now">
                ${t.confirmArrivalPrompt}
              </button>
            </div>
          ` : ''}

          ${isIllustrativePreview ? `
            <div class="illustrative-banner" role="status">
              <span>${t.illustrativeNotice}</span>
            </div>
          ` : ''}

          ${guidanceStatus === 'EMPTY' ? `
            <div class="warning-banner" role="status">
              <span>${t.noDestinations}</span>
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
                <p>${destinationCapacityText}</p>
                ${availableDestinations.length > 1 ? `
                  <div style="display: flex; gap: 6px; margin-top: 6px; flex-wrap: wrap;">
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
            ${simulationActive ? `<small style="color: var(--color-danger); font-size: 0.72rem;">[Simulation: ${escapeHtml(simulationNote)}]</small>` : ''}
          </div>

          <button class="primary-action ${routeStarted ? 'is-success' : ''}" data-testid="start-route" type="button" data-action="route" ${(!selectedDestination || isIllustrativePreview) ? 'disabled title="Awaiting verified server guidance"' : ''}>
            ${icons.route}<span>${reservationPending ? 'Reserving...' : routeStarted ? t.routeActive : t.startRoute}</span>${icons.arrow}
          </button>
          ${reservationError ? `
            <div class="warning-banner" role="alert" style="margin-top: 4px;">
              <span>${escapeHtml(reservationError)}</span>
            </div>
          ` : ''}

          <ol class="instructions">
            <li><span>1</span><p><strong>${t.instruction1Title}</strong> ${t.instruction1Body}</p></li>
            <li><span>2</span><p>${t.instruction2}</p></li>
            <li><span>3</span><p>${t.instruction3}</p></li>
          </ol>

          <div class="quick-actions">
            <button type="button" data-action="directions">${icons.route}<span>${t.directions}</span></button>
            <button type="button" data-action="listen">${icons.volume}<span>${t.listen}</span></button>
            <button type="button" data-action="voice-open">${icons.mic}<span>${t.askByVoice}</span></button>
          </div>

          <div class="simulation-panel">
            <span>${t.simulateBarTitle}</span>
            <div class="simulation-buttons">
              <button type="button" data-sim="en-route">${t.simEnRoute}</button>
              <button type="button" data-sim="near">${t.simNear}</button>
              <button type="button" data-sim="inaccurate">${t.simInaccurate}</button>
              <button type="button" data-sim="stale">${t.simStale}</button>
              <button type="button" data-sim="revoke">${t.simRevoke}</button>
              <button type="button" data-sim="reset">${t.simReset}</button>
            </div>
          </div>

          <button class="arrival-action" data-testid="arrival-confirmation" type="button" data-action="arrival-open">
            ${arrivalSuccess ? t.arrivalRecorded : arrivalPending ? 'Confirming...' : t.arrived}
          </button>

          <a class="rescue-action" data-testid="call-112" href="tel:112">
            <span>${t.trapped}</span><strong>${t.rescue}</strong>
          </a>
        </section>

        <section class="map-surface" aria-label="${t.mapAria}">
          <div id="map-canvas"></div>
          <div class="map-toolbar" aria-label="${t.mapTools}">
            <button type="button" data-action="recenter">${icons.locate}<span>${t.myLocation}</span></button>
            <button type="button" data-action="map-route">${icons.route}<span>${t.fullRoute}</span></button>
          </div>
          <div class="layer-switcher" aria-label="${t.mapLayers}">
            <button class="zone-toggle zone-toggle--danger ${redZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-red-zones" aria-pressed="${redZonesVisible}">
              <i></i><span>${t.redZones}</span>
            </button>
            <button class="zone-toggle zone-toggle--relocation ${relocationZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-relocation-zones" aria-pressed="${relocationZonesVisible}">
              <i></i><span>${t.relocationZones}</span>
            </button>
          </div>
          <div class="map-key">
            <span class="${redZonesVisible ? '' : 'is-muted'}"><i class="hazard-key"></i>${t.redZone}</span>
            <span><i class="route-key"></i>${t.approvedRoute}</span>
            <span class="${relocationZonesVisible ? '' : 'is-muted'}"><i class="relocation-key"></i>${t.relocationZone}</span>
            <span><i class="shelter-key"></i>${t.safeShelter}</span>
          </div>
          <div class="map-disclaimer">${t.imagery} <a href="https://www.esri.com/" target="_blank" rel="noreferrer">© Esri</a>, ${t.overlays}</div>
        </section>
      </main>

      <aside class="voice-console ${voiceOpen ? 'is-open' : ''}" role="dialog" aria-modal="false" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}>
        <div class="voice-head">
          <div><span>${t.assistant}</span><h2 id="voice-title">${t.askSituation}</h2></div>
          <button class="icon-button" type="button" data-action="voice-close" aria-label="${t.closeAssistant}">${icons.close}</button>
        </div>
        <div class="voice-stage ${voiceListening ? 'is-listening' : ''}">
          <div class="voice-orb" aria-hidden="true">${icons.mic}<i></i><i></i><i></i></div>
          <div>
            <strong>${voiceListening ? t.listening : chatPending ? t.checkingGuidance : t.ready}</strong>
            <span>${voiceListening ? t.speakNaturally : t[voiceFeedbackKey]}</span>
          </div>
        </div>
        <div class="chat-thread" aria-live="polite" aria-busy="${chatPending}">
          ${chatMessages.map((message, index) =>
            `<article class="chat-message chat-message--${message.role.toLowerCase()}">
              <span>${message.role === 'USER' ? t.you : 'Sthira'}</span>
              <p>${escapeHtml(message.text)}</p>
              ${message.role === 'ASSISTANT' && message.audio && isAudioValidForReplay(message.audio) ? `<button type="button" data-speak-message="${index}" aria-label="${t.readAloud}">${icons.volume}<span>${t.listen}</span></button>` : ''}
            </article>`
          ).join('')}
          ${autoplayBlockedAudio ? `
            <div style="padding: 4px 8px;">
              <button class="tap-play-button" type="button" data-action="tap-play-audio">
                ${icons.volume} <span>${t.tapToPlay}</span>
              </button>
            </div>
          ` : ''}
          ${chatPending ? `<div class="chat-thinking"><i></i><i></i><i></i><span>${t.checkingExercise}</span></div>` : ''}
        </div>
        ${chatError ? `<p class="chat-error" role="alert">${escapeHtml(chatError)}</p>` : ''}
        <div class="voice-suggestions" aria-label="${t.suggestedQuestions}">
          ${chatSuggestions.map((suggestion) => `<button type="button" data-command="${escapeHtml(suggestion)}">${escapeHtml(suggestion)}</button>`).join('')}
        </div>
        <form class="command-form" data-command-form>
          <label for="command-input">${t.askText}</label>
          <div>
            <input id="command-input" name="command" autocomplete="off" placeholder="${t.askPlaceholder}" ${chatPending ? 'disabled' : ''}/>
            <button type="submit" ${chatPending ? 'disabled' : ''}>${t.send}</button>
          </div>
        </form>
        <button class="listen-button ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-listen" aria-pressed="${voiceListening}">
          ${icons.mic}<span>${voiceListening ? t.stopListening : t.startListening}</span>
        </button>
        <p class="voice-boundary"><strong>${t.voiceBoundaryLabel}</strong> ${t.voiceBoundary}</p>
      </aside>

      ${directionsOpen ? `
        <aside class="side-sheet" aria-labelledby="directions-title">
          <div class="sheet-head">
            <div><span>${t.routeKicker}</span><h2 id="directions-title">${t.routeTitle}</h2></div>
            <button class="icon-button" data-action="directions-close" aria-label="${t.closeDirections}">${icons.close}</button>
          </div>
          <ol>
            <li><b>1</b><p>${t.routeStep1}<small>${t.routeStep1Note}</small></p></li>
            <li><b>2</b><p>${t.routeStep2}<small>${t.routeStep2Note}</small></p></li>
            <li><b>3</b><p>${t.routeStep3}<small>${t.routeStep3Note}</small></p></li>
          </ol>
        </aside>
      ` : ''}

      ${detailsOpen ? `
        <dialog class="modal" open>
          <div class="sheet-head">
            <div><span>${t.sourceFreshness}</span><h2>${t.alertDetails}</h2></div>
            <button class="icon-button" data-action="details-close" aria-label="${t.closeDetails}">${icons.close}</button>
          </div>
          <p>${t.demoNotice}</p>
          <dl>
            <div><dt>${t.authorityFormat}</dt><dd>NDMA SACHET / CAP (Source: SRCDEMO-1)</dd></div>
            <div><dt>Package ID</dt><dd>${PACKAGE_ID} (Jurisdiction: ${JURISDICTION})</dd></div>
            <div><dt>Freshness State</dt><dd>${guidanceFreshness}</dd></div>
            <div><dt>Valid Dates</dt><dd>${getTodayYMD()} to ${getTomorrowYMD()}</dd></div>
            <div><dt>${t.backend}</dt><dd>${runtimeCopy()}</dd></div>
          </dl>
        </dialog>
      ` : ''}

      ${arrivalOpen ? `
        <dialog class="modal" open>
          <div class="sheet-head">
            <div><span>${t.arrivalCheck}</span><h2>${t.arrivedSafely}</h2></div>
            <button class="icon-button" data-action="arrival-close" aria-label="${t.closeArrival}">${icons.close}</button>
          </div>
          ${arrivalSuccess ? `
            <div class="success-message">${t.arrivalRecorded}</div>
          ` : `
            <p>${t.confirmParty}</p>
            <div class="stepper">
              <button type="button" data-action="party-minus" aria-label="${t.decreaseParty}">-</button>
              <strong>${partySize} ${partySize === 1 ? t.person : t.people}</strong>
              <button type="button" data-action="party-plus" aria-label="${t.increaseParty}">+</button>
            </div>
            ${arrivalError ? `
              <div class="warning-banner" role="alert" style="margin-top: 8px;">
                <span>${escapeHtml(arrivalError)}</span>
              </div>
            ` : ''}
            <div class="modal-actions">
              <button class="primary-action" type="button" data-action="arrival-yes" ${arrivalPending ? 'disabled' : ''}>
                ${arrivalPending ? 'Confirming...' : t.weArrived}
              </button>
              <button class="secondary-action" type="button" data-action="arrival-no">${t.needHelp}</button>
            </div>
          `}
        </dialog>
      ` : ''}

      ${assistanceOpen ? `
        <dialog class="modal" open>
          <div class="sheet-head">
            <div><span>${t.emergencyAssistance}</span><h2>${t.call112Question}</h2></div>
            <button class="icon-button" data-action="assist-close" aria-label="${t.close}">${icons.close}</button>
          </div>
          <p>${t.assistNotice}</p>
          <a class="primary-action" href="tel:112">${t.call112Now}</a>
        </dialog>
      ` : ''}

      <div class="toast" role="status" aria-live="polite" hidden></div>
    </div>`;

  hasRendered = true;
  bindInteractions();
  initMap();
  requestAnimationFrame(() => {
    const thread = document.querySelector<HTMLElement>('.chat-thread');
    if (thread) thread.scrollTop = thread.scrollHeight;
  });
}

function mapColor(token: string) {
  return getComputedStyle(document.documentElement).getPropertyValue(token).trim();
}

function initMap() {
  const container = document.querySelector<HTMLElement>('#map-canvas');
  if (!container) return;
  const t = words[language];
  map = new Map({
    container,
    center: [76.112, 11.562],
    zoom: 13.4,
    attributionControl: false,
    style: {
      version: 8,
      sources: {
        basemap: {
          type: 'raster',
          tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'],
          tileSize: 256,
          attribution: 'Imagery © Esri',
        },
        hazard: {
          type: 'geojson',
          data: {
            type: 'Feature',
            geometry: { type: 'Polygon', coordinates: mapData.hazard },
            properties: { label: t.hazardLabel, detail: t.hazardDetail },
          },
        },
        relocation: {
          type: 'geojson',
          data: {
            type: 'FeatureCollection',
            features: [
              {
                type: 'Feature',
                geometry: { type: 'Polygon', coordinates: mapData.relocationZones[0] },
                properties: { label: t.relocation1Label, detail: t.relocation1Detail },
              },
              {
                type: 'Feature',
                geometry: { type: 'Polygon', coordinates: mapData.relocationZones[1] },
                properties: { label: t.relocation2Label, detail: t.relocation2Detail },
              },
            ],
          },
        },
        route: {
          type: 'geojson',
          data: {
            type: 'Feature',
            geometry: { type: 'LineString', coordinates: mapData.route },
            properties: {},
          },
        },
        roads: {
          type: 'geojson',
          data: {
            type: 'FeatureCollection',
            features: mapData.roads.map((coordinates) => ({
              type: 'Feature',
              geometry: { type: 'LineString', coordinates },
              properties: {},
            })),
          },
        },
        places: {
          type: 'geojson',
          data: {
            type: 'FeatureCollection',
            features: [
              {
                type: 'Feature',
                geometry: { type: 'Point', coordinates: mapData.user },
                properties: { label: t.userMapLabel, kind: 'user' },
              },
              {
                type: 'Feature',
                geometry: { type: 'Point', coordinates: mapData.shelter },
                properties: { label: t.shelterMapLabel, kind: 'shelter' },
              },
            ],
          },
        },
      },
      layers: [
        { id: 'background', type: 'background', paint: { 'background-color': mapColor('--map-color-surface') } },
        { id: 'basemap', type: 'raster', source: 'basemap', paint: { 'raster-opacity': 0.92, 'raster-saturation': -0.12, 'raster-contrast': 0.14, 'raster-brightness-max': 0.82 } },
        { id: 'roads', type: 'line', source: 'roads', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 1.5, 'line-opacity': 0.32 } },
        { id: 'hazard-band', type: 'line', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-danger'), 'line-width': 16, 'line-blur': 5, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
        { id: 'hazard-fill', type: 'fill', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'fill-color': mapColor('--map-color-danger'), 'fill-opacity': 0, 'fill-opacity-transition': { duration: motionDuration() } } },
        { id: 'hazard-edge', type: 'line', source: 'hazard', layout: { visibility: redZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-danger'), 'line-width': 2.5, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
        { id: 'relocation-band', type: 'line', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-success'), 'line-width': 16, 'line-blur': 5, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
        { id: 'relocation-fill', type: 'fill', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'fill-color': mapColor('--map-color-success'), 'fill-opacity': 0, 'fill-opacity-transition': { duration: motionDuration() } } },
        { id: 'relocation-edge', type: 'line', source: 'relocation', layout: { visibility: relocationZonesVisible ? 'visible' : 'none' }, paint: { 'line-color': mapColor('--map-color-success'), 'line-width': 2.5, 'line-opacity': 0, 'line-opacity-transition': { duration: motionDuration() } } },
        { id: 'route-casing', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 9, 'line-opacity': 0.9 } },
        { id: 'approved-route', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-accent'), 'line-width': 5, 'line-opacity': 0.98 } },
        { id: 'route-motion', type: 'line', source: 'route', paint: { 'line-color': mapColor('--map-color-paper'), 'line-width': 2, 'line-opacity': routeStarted ? 0.9 : 0, 'line-dasharray': [0.2, 2.4, 1.6] } },
        { id: 'shelter-pulse', type: 'circle', source: 'places', filter: ['==', ['get', 'kind'], 'shelter'], paint: { 'circle-radius': 15, 'circle-color': mapColor('--map-color-success'), 'circle-opacity': 0.24 } },
        { id: 'place-points', type: 'circle', source: 'places', paint: { 'circle-radius': 8, 'circle-color': mapColor('--map-color-accent'), 'circle-stroke-color': mapColor('--map-color-paper'), 'circle-stroke-width': 3 } },
        { id: 'place-labels', type: 'symbol', source: 'places', layout: { 'text-field': ['get', 'label'], 'text-size': 13, 'text-offset': [0, 1.5], 'text-anchor': 'top' }, paint: { 'text-color': mapColor('--map-color-paper'), 'text-halo-color': mapColor('--map-color-surface'), 'text-halo-width': 2 } },
      ],
    },
  });

  map.once('load', () => {
    map?.resize();
    if (routeStarted) focusRoute();
    revealMapLayers();
    startMapAnimation();

    map?.on('mouseenter', 'place-points', () => { if (map) map.getCanvas().style.cursor = 'pointer'; });
    map?.on('mouseleave', 'place-points', () => { if (map) map.getCanvas().style.cursor = ''; });

    map?.on('click', 'place-points', (event) => {
      const feature = event.features?.[0];
      const coordinates = feature?.geometry.type === 'Point' ? feature.geometry.coordinates as [number, number] : null;
      if (!map || !coordinates) return;
      new Popup({ offset: 14, closeButton: false })
        .setLngLat(coordinates)
        .setText(String(feature?.properties?.label || t.mapLocation))
        .addTo(map);
    });
  });
}

function motionDuration() {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 0 : 900;
}

function focusRoute() {
  map?.fitBounds(mapData.routeBounds as [[number, number], [number, number]], {
    padding: 90,
    duration: motionDuration(),
  });
}

function revealMapLayers() {
  if (!map) return;
  const show = () => {
    if (redZonesVisible) {
      map?.setPaintProperty('hazard-band', 'line-opacity', 0.2);
      map?.setPaintProperty('hazard-fill', 'fill-opacity', 0.34);
      map?.setPaintProperty('hazard-edge', 'line-opacity', 0.92);
    }
    if (relocationZonesVisible) {
      map?.setPaintProperty('relocation-band', 'line-opacity', 0.2);
      map?.setPaintProperty('relocation-fill', 'fill-opacity', 0.26);
      map?.setPaintProperty('relocation-edge', 'line-opacity', 0.92);
    }
  };
  if (motionDuration() === 0) show();
  else requestAnimationFrame(show);
}

function startMapAnimation() {
  if (!map || !routeStarted || motionDuration() === 0) return;
  const dashFrames = [[0.2, 2.4, 1.6], [0.7, 2.4, 1.1], [1.2, 2.4, 0.6], [1.7, 2.4, 0.1]];
  let frame = 0;
  const animate = () => {
    if (!map || !map.isStyleLoaded()) return;
    map.setPaintProperty('route-motion', 'line-dasharray', dashFrames[Math.floor(frame / 12) % dashFrames.length]);
    frame += 1;
    mapAnimationFrame = requestAnimationFrame(animate);
  };
  mapAnimationFrame = requestAnimationFrame(animate);
}

function isAudioValidForReplay(audio?: AudioMetadata): boolean {
  if (!audio || !audio.audio_b64) return false;
  if (guidanceFreshness !== 'CURRENT') return false;
  if (!audio.language || audio.language !== speechLanguageTag(language)) return false;
  if (!audio.data_version || audio.data_version !== currentDataVersion) return false;
  if (audio.source_version == null || audio.template_version == null) return false;
  return true;
}

let autoplayBlockedAudio: { audio: HTMLAudioElement; b64: string } | null = null;

async function verifyAndPlayAudio(audioInfo: AudioMetadata): Promise<{ success: boolean; autoplayBlocked: boolean; error?: string }> {
  if (!isAudioValidForReplay(audioInfo)) {
    return {
      success: false,
      autoplayBlocked: false,
      error: 'Audio guidance is invalid, expired, or does not match active language and version',
    };
  }

  if (audioInfo.content_type && !audioInfo.content_type.startsWith('audio/')) {
    return { success: false, autoplayBlocked: false, error: 'Invalid audio content-type: ' + audioInfo.content_type };
  }

  if (audioInfo.settings) {
    if (audioInfo.settings.sample_rate_hz !== undefined && audioInfo.settings.sample_rate_hz <= 0) {
      return { success: false, autoplayBlocked: false, error: 'Invalid audio settings: sample_rate_hz <= 0' };
    }
    if (audioInfo.settings.channels !== undefined && audioInfo.settings.channels <= 0) {
      return { success: false, autoplayBlocked: false, error: 'Invalid audio settings: channels <= 0' };
    }
  }

  let binaryStr: string;
  try {
    binaryStr = atob(audioInfo.audio_b64);
  } catch {
    return { success: false, autoplayBlocked: false, error: 'Corrupted audio base64' };
  }
  const byteLen = binaryStr.length;
  if (audioInfo.byte_size != null && byteLen !== audioInfo.byte_size) {
    return { success: false, autoplayBlocked: false, error: `Audio byte size mismatch: expected ${audioInfo.byte_size}, got ${byteLen}` };
  }
  if (byteLen > 768 * 1024) {
    return { success: false, autoplayBlocked: false, error: 'Audio exceeds maximum size ceiling' };
  }

  if (audioInfo.checksum_sha256 && window.crypto?.subtle) {
    const uint8 = new Uint8Array(byteLen);
    for (let i = 0; i < byteLen; i++) uint8[i] = binaryStr.charCodeAt(i);
    const hashBuf = await window.crypto.subtle.digest('SHA-256', uint8);
    const hashHex = Array.from(new Uint8Array(hashBuf)).map((b) => b.toString(16).padStart(2, '0')).join('');
    if (hashHex.toLowerCase() !== audioInfo.checksum_sha256.toLowerCase()) {
      return { success: false, autoplayBlocked: false, error: 'Audio checksum mismatch (integrity failure)' };
    }
  }

  return new Promise((resolve) => {
    try {
      const mime = audioInfo.content_type || 'audio/wav';
      const audio = new Audio(`data:${mime};base64,${audioInfo.audio_b64}`);
      audio.play().then(() => {
        autoplayBlockedAudio = null;
        resolve({ success: true, autoplayBlocked: false });
      }).catch((err: Error) => {
        if (err.name === 'NotAllowedError') {
          autoplayBlockedAudio = { audio, b64: audioInfo.audio_b64 };
          render();
          resolve({ success: false, autoplayBlocked: true, error: 'Autoplay blocked by browser policy. Tap to play.' });
        } else {
          resolve({ success: false, autoplayBlocked: false, error: err.message });
        }
      });
    } catch (e: any) {
      resolve({ success: false, autoplayBlocked: false, error: e?.message || 'Audio playback error' });
    }
  });
}

async function blobToBase64(blob: Blob): Promise<string> {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

async function sendVoiceOrText(input: { kind: 'audio'; body_b64: string; content_type: string } | { kind: 'transcript'; text: string }) {
  chatPending = true;
  chatError = '';
  voiceFeedbackKey = 'checkingBackend';
  render();

  try {
    const res = await fetch('/api/v3/voice/process', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        request_id: 'req-' + Math.random().toString(36).slice(2, 10),
        jurisdiction: JURISDICTION,
        language: speechLanguageTag(language),
        input,
        render: { kind: 'tts' },
      }),
    });

    if (!res.ok) {
      if (res.status === 503 || res.status === 504) {
        // If voice/middle model is unavailable, provide usable touch/text lookup via places API
        if (input.kind === 'transcript' && input.text.trim()) {
          chatPending = false;
          render();
          await resolvePlace(input.text.trim());
          return;
        }
        throw new Error('MODEL_UNAVAILABLE');
      }
      throw new Error(`Voice pipeline returned ${res.status}`);
    }

    const envelope = await res.json() as {
      data?: {
        data_version?: string;
        validated_proposal?: unknown;
        template?: { speech_key?: string; text?: string; template_version?: number };
        audio?: {
          audio_b64?: string;
          content_type?: string;
          byte_size?: number;
          checksum_sha256?: string;
          source_id?: string;
          source_version?: number;
          template_key?: string;
          template_version?: number;
          language?: string;
          settings?: {
            sample_rate_hz?: number;
            channels?: number;
            bit_depth?: number;
            codec?: string;
          };
        };
        state?: string;
      };
    };

    const out = envelope.data;
    if (!out?.template?.text) {
      chatPending = false;
      chatError = words[language].assistantUnavailable;
      voiceFeedbackKey = 'backendUnavailable';
      render();
      return;
    }

    const audioDataVersion = out.data_version || currentDataVersion;
    const audioMeta: AudioMetadata | undefined = out.audio?.audio_b64
      ? {
          audio_b64: out.audio.audio_b64,
          content_type: out.audio.content_type,
          byte_size: out.audio.byte_size,
          checksum_sha256: out.audio.checksum_sha256,
          source_id: out.audio.source_id,
          source_version: out.audio.source_version ?? 1,
          data_version: audioDataVersion,
          template_key: out.template.speech_key,
          template_version: out.audio.template_version ?? out.template.template_version ?? 1,
          language: out.audio.language || speechLanguageTag(language),
          settings: out.audio.settings,
        }
      : undefined;

    chatMessages.push({ role: 'ASSISTANT', text: out.template.text, audio: audioMeta });
    voiceFeedbackKey = 'responseReady';

    // Execute validated map action if proposal is present
    if (out?.validated_proposal && map) {
      executeMapActions(
        map,
        out.validated_proposal,
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
          render();
        },
        (candidates) => {
          ambiguousPlaces = candidates.map((id) => ({ place_id: id, place_kind: 'candidate' }));
          render();
        }
      );
    }

    // Play verified synthesized audio if returned
    if (audioMeta) {
      const playRes = await verifyAndPlayAudio(audioMeta);
      if (!playRes.success && !playRes.autoplayBlocked) {
        chatError = `Audio verification notice: ${playRes.error}`;
      }
    }

    chatPending = false;
    render();
  } catch {
    chatPending = false;
    chatError = words[language].assistantUnavailable;
    voiceFeedbackKey = 'backendUnavailable';
    render();
  }
}

async function toggleLocalRecording() {
  if (voiceListening && mediaRecorder) {
    mediaRecorder.stop();
    return;
  }
  try {
    mediaStream = await navigator.mediaDevices.getUserMedia({ audio: true });
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus')
      ? 'audio/webm;codecs=opus'
      : 'audio/webm';
    const chunks: Blob[] = [];
    mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
    mediaRecorder.ondataavailable = (event) => {
      if (event.data.size) chunks.push(event.data);
    };
    mediaRecorder.onstop = async () => {
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
    window.setTimeout(() => {
      if (mediaRecorder?.state === 'recording') mediaRecorder.stop();
    }, 20_000);
  } catch {
    voiceListening = false;
    voiceFeedbackKey = 'micStopped';
    render();
  }
}

function startTracking() {
  simulationActive = false;
  simulationNote = '';
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
  mapData.user = [reading.longitude, reading.latitude];

  const evalResult = evaluateProximity(reading, getDestinationTarget(), DEFAULT_JOURNEY_OPTIONS);
  lastProximityEval = evalResult;

  const nextState = transitionOnPosition(journeyState, evalResult);
  journeyState = nextState;
  render();
}

function simulatePosition(
  lon: number,
  lat: number,
  accuracy: number,
  timestampOffsetMs: number = 0,
  note: string = ''
) {
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  simulationActive = true;
  simulationNote = note;
  locationErrorMessage = '';

  if (journeyState === 'NOT_STARTED' || journeyState === 'PAUSED' || journeyState === 'LOCATION_UNAVAILABLE') {
    journeyState = 'TRACKING';
  }

  const reading: PositionReading = {
    longitude: lon,
    latitude: lat,
    accuracyMeters: accuracy,
    timestamp: Date.now() - timestampOffsetMs,
  };
  applyPositionUpdate(reading);
}

function revokeRoute() {
  journeyState = transitionOnRevocation(journeyState);
  autoplayBlockedAudio = null;
  guidanceFreshness = 'REVOKED';
  currentDataVersion = 'REVOKED';
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  render();
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

  // Create explicit reservation on Go backend if session is initialized
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
      const data = await res.json() as { data?: { reservation_id?: string; stay_id?: string } };
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

  // If active stay exists on backend, arrival requires explicit server acknowledgment
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

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((b) =>
    b.addEventListener('click', () => {
      const nextLanguage = b.dataset.language as Language;
      if (chatMessages.length === 1 && chatMessages[0]?.role === 'ASSISTANT') {
        chatMessages = [{ role: 'ASSISTANT', text: words[nextLanguage].welcome }];
      }
      language = nextLanguage;
      chatSuggestions = [...words[language].suggestions];
      chatError = '';
      voiceFeedbackKey = 'micPrivacy';
      autoplayBlockedAudio = null;
      render();
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-action="voice-open"]').forEach((b) =>
    b.addEventListener('click', () => {
      voiceOpen = true;
      render();
      document.querySelector<HTMLInputElement>('#command-input')?.focus();
    })
  );

  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => {
    if (mediaRecorder?.state === 'recording') mediaRecorder.stop();
    voiceListening = false;
    voiceOpen = false;
    render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => {
    void toggleLocalRecording();
  });

  document.querySelectorAll<HTMLButtonElement>('[data-command]').forEach((b) =>
    b.addEventListener('click', () => {
      const cmd = b.dataset.command || '';
      chatMessages.push({ role: 'USER', text: cmd });
      void sendVoiceOrText({ kind: 'transcript', text: cmd });
    })
  );

  document.querySelectorAll<HTMLButtonElement>('[data-speak-message]').forEach((b) =>
    b.addEventListener('click', () => {
      const message = chatMessages[Number(b.dataset.speakMessage)];
      if (message?.audio && isAudioValidForReplay(message.audio)) {
        void verifyAndPlayAudio(message.audio);
      }
    })
  );

  document.querySelector<HTMLButtonElement>('[data-action="tap-play-audio"]')?.addEventListener('click', () => {
    if (autoplayBlockedAudio) {
      const { audio } = autoplayBlockedAudio;
      autoplayBlockedAudio = null;
      audio.play().catch(() => {});
      render();
    }
  });

  document.querySelectorAll<HTMLButtonElement>('[data-select-facility]').forEach((b) =>
    b.addEventListener('click', () => {
      const facId = b.dataset.selectFacility;
      const found = availableDestinations.find((d) => d.facility_id === facId);
      if (found) {
        selectedDestination = found;
        if (found.coordinates) {
          mapData.shelter = found.coordinates;
        }
        render();
      }
    })
  );

  document.querySelector<HTMLFormElement>('[data-command-form]')?.addEventListener('submit', (e) => {
    e.preventDefault();
    const inputEl = (e.currentTarget as HTMLFormElement).elements.namedItem('command') as HTMLInputElement | null;
    const text = inputEl?.value.trim() || '';
    if (!text) return;
    chatMessages.push({ role: 'USER', text });
    if (inputEl) inputEl.value = '';
    render();
    void sendVoiceOrText({ kind: 'transcript', text });
  });

  document.querySelectorAll<HTMLButtonElement>('[data-candidate-id]').forEach((b) =>
    b.addEventListener('click', () => {
      const candId = b.dataset.candidateId;
      if (candId) {
        void selectCandidatePlace(candId);
      }
    })
  );

  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => {
    void startRouteReservation();
  });

  document.querySelector<HTMLButtonElement>('[data-action="map-route"]')?.addEventListener('click', focusRoute);

  document.querySelector<HTMLButtonElement>('[data-action="recenter"]')?.addEventListener('click', () => {
    map?.easeTo({ center: mapData.user as [number, number], zoom: 15, duration: motionDuration() });
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
    b.addEventListener('click', () => { directionsOpen = true; render(); })
  );
  document.querySelector<HTMLButtonElement>('[data-action="directions-close"]')?.addEventListener('click', () => {
    directionsOpen = false; render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', () => {
    const lastWithAudio = [...chatMessages].reverse().find((m) => m.audio && isAudioValidForReplay(m.audio));
    if (lastWithAudio?.audio) {
      void verifyAndPlayAudio(lastWithAudio.audio);
    }
  });

  document.querySelector<HTMLButtonElement>('[data-action="details"]')?.addEventListener('click', () => {
    detailsOpen = true; render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="details-close"]')?.addEventListener('click', () => {
    detailsOpen = false; render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="arrival-open"]')?.addEventListener('click', () => {
    arrivalOpen = true; render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => {
    arrivalOpen = false; render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="party-minus"]')?.addEventListener('click', () => {
    partySize = Math.max(1, partySize - 1); render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="party-plus"]')?.addEventListener('click', () => {
    partySize = Math.min(10, partySize + 1); render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="arrival-yes"]')?.addEventListener('click', () => {
    void confirmArrival();
  });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-no"]')?.addEventListener('click', () => {
    arrivalOpen = false; assistanceOpen = true; render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="assist-close"]')?.addEventListener('click', () => {
    assistanceOpen = false; render();
  });

  document.querySelector<HTMLButtonElement>('[data-action="start-tracking"]')?.addEventListener('click', () => {
    startTracking();
  });

  document.querySelector<HTMLButtonElement>('[data-action="stop-tracking"]')?.addEventListener('click', () => {
    stopTracking();
  });

  document.querySelector<HTMLButtonElement>('[data-action="arrival-confirm-now"]')?.addEventListener('click', () => {
    void confirmArrival();
  });

  document.querySelector<HTMLButtonElement>('[data-sim="en-route"]')?.addEventListener('click', () => {
    simulatePosition(76.115, 11.560, 12, 0, 'En route 1.5 km');
  });

  document.querySelector<HTMLButtonElement>('[data-sim="near"]')?.addEventListener('click', () => {
    simulatePosition(76.1053, 11.5702, 10, 0, 'Near shelter 40m');
  });

  document.querySelector<HTMLButtonElement>('[data-sim="inaccurate"]')?.addEventListener('click', () => {
    simulatePosition(76.1053, 11.5702, 250, 0, 'Inaccurate GPS ±250m');
  });

  document.querySelector<HTMLButtonElement>('[data-sim="stale"]')?.addEventListener('click', () => {
    simulatePosition(76.1053, 11.5702, 10, 45000, 'Stale GPS 45s old');
  });

  document.querySelector<HTMLButtonElement>('[data-sim="revoke"]')?.addEventListener('click', () => {
    revokeRoute();
  });

  document.querySelector<HTMLButtonElement>('[data-sim="reset"]')?.addEventListener('click', () => {
    simulationActive = false;
    simulationNote = '';
    journeyState = 'NOT_STARTED';
    currentPosition = null;
    lastProximityEval = null;
    locationErrorMessage = '';
    mapData.user = [76.123, 11.553];
    stopTracking();
    render();
  });

  document.querySelectorAll<HTMLAnchorElement>('a[href="tel:112"]').forEach((a) =>
    a.addEventListener('click', (e) => {
      if (!assistanceOpen) {
        e.preventDefault();
        assistanceOpen = true;
        recordEmergencyAudit({
          number: '112',
          action: 'OPEN_SHEET',
          isTrusted: e.isTrusted === true,
          foreground: !document.hidden,
          status: 'ALLOWED',
        });
        render();
      } else {
        const result = triggerEmergencyDial({
          number: '112',
          event: e,
          documentRef: document,
        });
        if (!result.success) {
          e.preventDefault();
        }
      }
    })
  );
}

// Initial bootstrap
render();
void initSession();
void checkRuntime();
void queryGuidanceDestinations();

document.addEventListener('visibilitychange', () => {
  if (document.hidden) {
    tabInBackground = true;
    lastBackgroundTime = Date.now();
    stopTracking();
    if (voiceListening && mediaRecorder) {
      try {
        mediaRecorder.stop();
      } catch {}
      voiceListening = false;
      if (mediaStream) {
        mediaStream.getTracks().forEach((track) => track.stop());
        mediaStream = null;
      }
    }
  } else {
    tabInBackground = false;
  }
  render();
});

window.addEventListener('hashchange', () => {
  render();
});

window.addEventListener('online', () => {
  runtime = 'checking';
  void checkRuntime();
});

window.addEventListener('offline', () => {
  runtime = 'offline';
  autoplayBlockedAudio = null;
  guidanceFreshness = 'UNAVAILABLE';
  currentDataVersion = 'UNAVAILABLE';
  render();
});

