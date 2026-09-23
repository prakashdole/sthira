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
  evaluateProximity,
  transitionOnPosition,
  transitionOnArrival,
  transitionOnRevocation,
  DEFAULT_JOURNEY_OPTIONS,
} from './journey';
import { renderOperatorView } from './operator';

type RuntimeState = 'checking' | 'demo' | 'blocked' | 'offline';
type ChatMessage = { role: 'USER' | 'ASSISTANT'; text: string; audioB64?: string };
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

// State for Go /api/v3 session and guidance integration
const JURISDICTION = 'IN-KL';
const PACKAGE_ID = 'PKG-EXERCISE-01';
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

let ambiguousPlaces: AmbiguousCandidate[] = [];
let availableDestinations: DestinationChoice[] = [
  {
    facility_id: 'FAC-DEMO-01',
    safe_zone_id: 'SZ-DEMO-01',
    facility_name: 'Demo Community Hall',
    capacity_known: true,
    free: 45,
    route_id: 'ROUTE-DEMO-01',
    route_verified: true,
    distance_km: 2.8,
    duration_minutes: 35,
  },
];
let selectedDestination: DestinationChoice = availableDestinations[0]!;

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
    id: selectedDestination.facility_id,
    name: selectedDestination.facility_name,
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

async function initSession(): Promise<void> {
  if (sessionToken) return;
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

async function queryGuidanceDestinations(): Promise<void> {
  try {
    const res = await fetch('/api/v3/guidance/query', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        jurisdiction: JURISDICTION,
        package_id: PACKAGE_ID,
        party_size: partySize,
        start_date: '2026-09-12',
        end_date: '2026-09-13',
      }),
    });
    if (res.ok) {
      const result = await res.json() as {
        data?: {
          destinations?: Array<{
            facility_id: string;
            safe_zone_id: string;
            capacity_known: boolean;
            free: number | null;
            route_id?: string;
            route_verified: boolean;
          }>;
        };
      };
      if (result.data?.destinations && result.data.destinations.length > 0) {
        availableDestinations = result.data.destinations.map((d, index) => ({
          facility_id: d.facility_id,
          safe_zone_id: d.safe_zone_id,
          facility_name: d.facility_id === 'FAC-DEMO-01' ? 'Demo Community Hall' : `Shelter ${d.facility_id}`,
          capacity_known: d.capacity_known,
          free: d.free,
          route_id: d.route_id || 'ROUTE-DEMO-01',
          route_verified: d.route_verified,
          distance_km: 2.8 + index * 1.2,
          duration_minutes: 35 + index * 15,
        }));
        selectedDestination = availableDestinations[0]!;
      }
    }
  } catch {
    // Keep synthetic fallback
  }
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
        render();
        return;
      }
    }
    if (res.ok) {
      ambiguousPlaces = [];
      const data = await res.json() as { data?: { place_id: string } };
      if (data.data?.place_id) {
        chatMessages.push({ role: 'ASSISTANT', text: `Resolved location: ${data.data.place_id}` });
        render();
      }
    }
  } catch {
    // Fail gracefully
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

  const destinationCapacityText = selectedDestination.capacity_known
    ? (selectedDestination.free !== null && selectedDestination.free > 0
        ? `${selectedDestination.free} spaces available`
        : t.capacityFull)
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

          <article class="destination">
            <div>
              <span class="destination-label">${t.destinationLabel}</span>
              <h2>${escapeHtml(selectedDestination.facility_name)}</h2>
              <p>${destinationCapacityText}</p>
            </div>
            <div class="distance">
              <strong>${selectedDestination.distance_km ?? 2.8} km</strong>
              <span>${selectedDestination.duration_minutes ?? 35} min</span>
            </div>
          </article>

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

          <button class="primary-action ${routeStarted ? 'is-success' : ''}" data-testid="start-route" type="button" data-action="route">
            ${icons.route}<span>${routeStarted ? t.routeActive : t.startRoute}</span>${icons.arrow}
          </button>

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
            ${arrivalRecordedAt ? t.arrivalRecorded : t.arrived}
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
              ${message.role === 'ASSISTANT' ? `<button type="button" data-speak-message="${index}" aria-label="${t.readAloud}">${icons.volume}<span>${t.listen}</span></button>` : ''}
            </article>`
          ).join('')}
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
            <div><dt>${t.authorityFormat}</dt><dd>NDMA SACHET / CAP (Go /api/v3)</dd></div>
            <div><dt>${t.issued}</dt><dd>11 Sep 2026, 4:00 PM</dd></div>
            <div><dt>${t.expires}</dt><dd>11 Sep 2026, 6:00 PM</dd></div>
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
            <div class="modal-actions">
              <button class="primary-action" type="button" data-action="arrival-yes">${t.weArrived}</button>
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

function speakText(text: string) {
  if (!('speechSynthesis' in window)) return;
  window.speechSynthesis.cancel();
  const u = new SpeechSynthesisUtterance(text);
  u.lang = speechLanguageTag(language);
  window.speechSynthesis.speak(u);
}

function playAudioB64(b64: string, fallbackText?: string) {
  try {
    const audio = new Audio(`data:audio/wav;base64,${b64}`);
    audio.play().catch(() => {
      if (fallbackText) speakText(fallbackText);
    });
  } catch {
    if (fallbackText) speakText(fallbackText);
  }
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
      // If voice model worker is unavailable (503), fall back gracefully to text commands
      if (res.status === 503 || res.status === 504) {
        if (input.kind === 'transcript') {
          await fallbackTextCommand(input.text);
          return;
        }
        throw new Error('MODEL_UNAVAILABLE');
      }
      throw new Error(`Voice pipeline returned ${res.status}`);
    }

    const envelope = await res.json() as {
      data?: {
        validated_proposal?: unknown;
        template?: { speech_key?: string };
        audio?: { audio_b64?: string };
        state?: string;
      };
    };

    const out = envelope.data;
    const replyText = out?.template?.speech_key
      ? (scenario.instruction[language === 'ML' ? 'ML' : 'EN'] || words[language].summary)
      : words[language].responseReady;

    chatMessages.push({ role: 'ASSISTANT', text: replyText });
    voiceFeedbackKey = 'responseReady';

    // Execute validated map action if proposal is present
    if (out?.validated_proposal && map) {
      executeMapActions(map, out.validated_proposal, motionDuration() === 0, (panel) => {
        if (panel === 'ROUTE_GUIDANCE') directionsOpen = true;
        if (panel === 'ALERT_DETAILS') detailsOpen = true;
        if (panel === 'EMERGENCY_CALL_CONFIRMATION') assistanceOpen = true;
        render();
      }, (newLang) => {
        if (newLang === 'ml-IN') language = 'ML';
        if (newLang === 'hi-IN') language = 'HI';
        if (newLang === 'en-IN') language = 'EN';
        render();
      });
    }

    // Play synthesized audio if returned
    if (out?.audio?.audio_b64) {
      playAudioB64(out.audio.audio_b64, replyText);
    } else {
      speakText(replyText);
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

async function fallbackTextCommand(text: string) {
  try {
    const res = await fetch('/api/v3/voice/commands', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        request_id: 'req-' + Math.random().toString(36).slice(2, 10),
        data_version: '3.0',
        jurisdiction: JURISDICTION,
        proposal: {
          schema_version: '1.0',
          status: 'OK',
          actions: [{ type: 'RECENTER', view_id: 'DEMO_OVERVIEW' }],
        },
      }),
    });
    if (res.ok) {
      chatMessages.push({ role: 'ASSISTANT', text: words[language].responseReady });
    } else {
      chatMessages.push({ role: 'ASSISTANT', text: words[language].summary });
    }
  } catch {
    chatMessages.push({ role: 'ASSISTANT', text: words[language].summary });
  }
  chatPending = false;
  render();
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
  if (geolocationWatchId !== null) {
    navigator.geolocation.clearWatch(geolocationWatchId);
    geolocationWatchId = null;
  }
  render();
}

async function startRouteReservation() {
  routeStarted = true;
  directionsOpen = true;
  if (journeyState === 'NOT_STARTED') {
    startTracking();
  }
  render();
  focusRoute();

  // Create explicit reservation on Go backend if session is initialized
  try {
    await initSession();
    const res = await fetch('/api/v3/reservations', {
      method: 'POST',
      headers: authHeaders(),
      body: JSON.stringify({
        facility_id: selectedDestination.facility_id,
        package_id: PACKAGE_ID,
        route_id: selectedDestination.route_id || 'ROUTE-DEMO-01',
        party_size: partySize,
        start_date: '2026-09-12',
        end_date: '2026-09-13',
        idempotency_key: 'idem-' + Math.random().toString(36).slice(2, 10),
        snapshot_version: 1,
      }),
    });
    if (res.ok) {
      const data = await res.json() as { data?: { reservation_id?: string; stay_id?: string } };
      if (data.data?.reservation_id) {
        activeReservationId = data.data.reservation_id;
        sessionStorage.setItem('sthira_reservation_id', activeReservationId);
      }
      if (data.data?.stay_id) {
        activeStayId = data.data.stay_id;
        sessionStorage.setItem('sthira_stay_id', activeStayId);
      }
    }
  } catch {
    // Keep local route state active
  }
}

async function confirmArrival() {
  const transition = transitionOnArrival(journeyState);
  if (!transition.isNewTransition) {
    arrivalOpen = false;
    render();
    return;
  }

  journeyState = 'ARRIVAL_REPORTED';
  arrivalSuccess = true;
  arrivalRecordedAt = new Date().toLocaleTimeString();
  stopTracking();
  render();

  // Send explicit ARRIVE event to Go backend if reservation exists
  if (activeReservationId || activeStayId) {
    const id = activeReservationId || activeStayId;
    try {
      await fetch(`/api/v3/reservations/${id}/events`, {
        method: 'POST',
        headers: authHeaders(),
        body: JSON.stringify({
          type: 'ARRIVE',
          idempotency_key: 'arrive-' + Math.random().toString(36).slice(2, 10),
          party_size: partySize,
          payload: {
            proximity_verified: lastProximityEval?.isNear ?? false,
            accuracy_meters: currentPosition?.accuracyMeters ?? null,
            timestamp: new Date().toISOString(),
          },
        }),
      });
    } catch {
      // Local arrival confirmation retained
    }
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
      if (message) {
        if (message.audioB64) playAudioB64(message.audioB64, message.text);
        else speakText(message.text);
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
        ambiguousPlaces = [];
        chatMessages.push({ role: 'USER', text: `Selected: ${candId}` });
        render();
        void resolvePlace(candId);
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
    speakText(words[language].summary);
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
        render();
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
  render();
});

