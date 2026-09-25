import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import type { Map as MapLibreMap } from 'maplibre-gl';
import mapData from './mapData.json';
import { words, type Language } from './i18n';
import { validateVoiceResponse, type MapAction as VoiceMapAction } from './mapActions';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
import '@fontsource/noto-sans/800.css';
import '@fontsource/noto-sans-malayalam/400.css';
import '@fontsource/noto-sans-malayalam/700.css';
import '@fontsource/noto-sans-devanagari/400.css';
import '@fontsource/noto-sans-devanagari/700.css';

type RuntimeState = 'checking' | 'demo' | 'blocked' | 'offline';
type VoiceStatus = { data?: { ready?: boolean; supported_languages?: string[] } };
type OnboardingStep = 'language' | 'location' | null;
type LocationStatus = 'idle' | 'checking' | 'ready' | 'unavailable';

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

function savedLanguage(): Language {
  try {
    const saved = localStorage.getItem('sthira-language');
    return saved === 'ML' || saved === 'HI' || saved === 'EN' ? saved : 'EN';
  } catch { return 'EN'; }
}

function savedOnboardingStep(): OnboardingStep {
  // Setup is deliberately shown on every fresh app load. In an emergency, this
  // keeps the spoken language and location choice visible instead of hiding them
  // behind a previous browser session.
  return 'language';
}

let language: Language = savedLanguage();
let onboardingStep: OnboardingStep = savedOnboardingStep();
let locationStatus: LocationStatus = 'idle';
let deviceLocation: [number, number] | null = null;
let runtime: RuntimeState = navigator.onLine ? 'checking' : 'offline';
let runtimeDetail: 'blocked' | 'responding' | 'disconnected' = 'blocked';
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
let localAsrReady = false;
let commandPending = false;
let commandError = '';
let commandResponse: string = words.EN.voiceReady;
let commandSuggestions: string[] = [...words.EN.voiceCommands];
let routeStarted = false;
let directionsOpen = false;
let detailsOpen = false;
let assistanceOpen = false;
let audioOpen = false;
let islOpen = false;

function runtimeCopy() {
  const t = words[language];
  if (runtime === 'offline') return t.offline;
  if (runtime === 'blocked' || runtimeDetail === 'blocked') return t.blocked;
  if (runtimeDetail === 'disconnected') return t.localDisconnected;
  if (runtime === 'demo') return t.responding;
  return t.checking;
}

function escapeHtml(value: string) {
  return value.replace(/[&<>"']/g, (character) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[character]!);
}

function completeOnboarding() {
  onboardingStep = null;
  render();
}

function requestLocation() {
  if (!navigator.geolocation) { locationStatus = 'unavailable'; render(); return; }
  locationStatus = 'checking'; render();
  navigator.geolocation.getCurrentPosition(
    (position) => { deviceLocation = [position.coords.longitude, position.coords.latitude]; locationStatus = 'ready'; render(); },
    () => { locationStatus = 'unavailable'; render(); },
    { enableHighAccuracy: false, timeout: 8_000, maximumAge: 300_000 },
  );
}

function renderOnboarding() {
  const t = words[language];
  document.documentElement.lang = language === 'ML' ? 'ml' : language === 'HI' ? 'hi' : 'en';
  const languageStep = onboardingStep === 'language';
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <main class="onboarding" aria-labelledby="onboarding-title">
      <section class="onboarding-card">
        <a class="brand onboarding-brand" href="/" aria-label="${t.brandHome}"><span class="brand-mark">സ്</span><span><strong>Sthira</strong><small>${t.tagline}</small></span></a>
        <div class="onboarding-copy">
          <span>${t.onboardingKicker}</span>
          <h1 id="onboarding-title">${languageStep ? t.onboardingLanguageTitle : t.onboardingLocationTitle}</h1>
          <p>${languageStep ? t.onboardingLanguageBody : t.onboardingLocationBody}</p>
        </div>
        ${languageStep ? `
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
          <article class="destination"><div><span class="destination-label">${t.destinationLabel}</span><h2>${t.destinationName}</h2><p>${t.destinationMeta}</p></div><div class="distance"><strong>${t.distance}</strong><span>${t.duration}</span></div></article>
          <button class="primary-action ${routeStarted ? 'is-success' : ''}" data-testid="start-route" type="button" data-action="route">${icons.route}<span>${routeStarted ? t.routeActive : t.startRoute}</span>${icons.arrow}</button>
          <ol class="instructions"><li><span>1</span><p><strong>${t.instruction1Title}</strong> ${t.instruction1Body}</p></li><li><span>2</span><p>${t.instruction2}</p></li><li><span>3</span><p>${t.instruction3}</p></li></ol>
          <p class="grounding-line">${t.groundingLine}</p>
          <div class="quick-actions"><button type="button" data-action="directions">${icons.route}<span>${t.directions}</span></button><button type="button" data-action="listen">${icons.volume}<span>${t.listen}</span></button><button type="button" data-action="isl">${icons.info}<span>${t.isl}</span></button><button type="button" data-action="voice-open">${icons.mic}<span>${t.askByVoice}</span></button></div>
          <a class="rescue-action" data-testid="call-112" href="tel:112"><span>${t.trapped}</span><strong>${t.rescue}</strong></a>
        </section>
        <section class="map-surface" aria-label="${t.mapAria}">
          <div id="map-canvas"></div>
          <div class="map-loading" role="status">${mapTilted ? t.loadingTerrain : t.mapLoading}</div>
          <div class="map-controls"><div class="map-toolbar" aria-label="${t.mapTools}"><button class="${mapTilted ? 'is-active' : ''}" type="button" data-action="toggle-3d" aria-label="${t.map3d}" aria-pressed="${mapTilted}"><span class="map-toolbar__perspective-label">${t.map3d}</span><span class="map-toolbar__perspective-short" aria-hidden="true">3D</span></button><button type="button" data-action="recenter" aria-label="${t.myLocation}">${icons.locate}<span>${t.myLocation}</span></button><button class="${layersOpen ? 'is-active' : ''}" type="button" data-action="toggle-layers" aria-label="${t.mapLayers}" aria-expanded="${layersOpen}">${icons.info}<span>${t.mapLayers}</span></button></div>${layersOpen ? `<div class="layer-switcher" aria-label="${t.mapLayers}"><button class="zone-toggle zone-toggle--danger ${redZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-red-zones" aria-pressed="${redZonesVisible}"><i></i><span>${t.redZones}</span></button><button class="zone-toggle zone-toggle--relocation ${relocationZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-relocation-zones" aria-pressed="${relocationZonesVisible}"><i></i><span>${t.relocationZones}</span></button></div>` : ''}</div>
          <div class="map-key"><span class="${redZonesVisible ? '' : 'is-muted'}"><i class="hazard-key"></i>${t.redZone}</span><span><i class="route-key"></i>${t.approvedRoute}</span><span class="${relocationZonesVisible ? '' : 'is-muted'}"><i class="relocation-key"></i>${t.relocationZone}</span><span><i class="shelter-key"></i>${t.safeShelter}</span></div>
          <div class="map-disclaimer">${t.imagery} <a href="https://www.esri.com/" target="_blank" rel="noreferrer">© Esri</a> · ${t.buildingContext} · ${t.overlays}</div>
          <nav class="mobile-safety-dock" aria-label="${t.emergencyAssistance}"><button class="dock-action dock-action--emergency" type="button" data-action="assist-open"><span class="dock-icon" aria-hidden="true">${icons.phone}</span><span>${t.emergencyCall}</span></button><button class="dock-action dock-action--voice ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-open" aria-expanded="${voiceOpen}"><span class="dock-icon" aria-hidden="true">${icons.mic}</span><span>${t.voicePrompt}</span></button><button class="dock-action" type="button" data-action="start-route"><span class="dock-icon" aria-hidden="true">${icons.route}</span><span>${routeStarted ? t.routeActive : t.routeShort}</span></button></nav>
        </section>
      </main>
      <aside class="voice-console ${voiceOpen ? 'is-open' : ''}" role="dialog" aria-modal="false" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}>
        <div class="voice-head"><div><span>${t.assistant}</span><h2 id="voice-title">${t.askSituation}</h2></div><button class="icon-button" type="button" data-action="voice-close" aria-label="${t.closeAssistant}">${icons.close}</button></div>
        <div class="voice-stage ${voiceListening ? 'is-listening' : ''}"><div class="voice-orb" aria-hidden="true">${icons.mic}<i></i><i></i><i></i></div><div><strong>${voiceListening ? t.listening : commandPending ? t.checkingGuidance : t.ready}</strong><span>${voiceListening ? t.speakNaturally : t[voiceFeedbackKey]}</span></div></div>
        ${voiceTranscript || commandPending || commandResponse !== words[language].voiceReady ? `<div class="command-result" aria-live="polite" aria-busy="${commandPending}"><span>${t.voiceResult}</span><p>${escapeHtml(commandResponse)}</p>${voiceTranscript ? `<small>${t.you}: ${escapeHtml(voiceTranscript)}</small>` : ''}${commandPending ? `<div class="command-thinking"><i></i><i></i><i></i><span>${t.checkingExercise}</span></div>` : ''}</div>` : ''}
        ${commandError ? `<p class="command-error" role="alert">${escapeHtml(commandError)}</p>` : ''}
        <div class="voice-suggestions" aria-label="${t.suggestedQuestions}">${commandSuggestions.map((suggestion) => `<button type="button" data-command="${escapeHtml(suggestion)}">${escapeHtml(suggestion)}</button>`).join('')}</div>
        <form class="command-form" data-command-form><label for="command-input">${t.askText}</label><div><input id="command-input" name="command" autocomplete="off" placeholder="${t.askPlaceholder}" ${commandPending ? 'disabled' : ''}/><button type="submit" ${commandPending ? 'disabled' : ''}>${t.send}</button></div></form>
        <button class="listen-button ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-listen" aria-pressed="${voiceListening}">${icons.mic}<span>${voiceListening ? t.stopListening : t.startListening}</span></button>
        <p class="voice-boundary"><strong>${t.voiceBoundaryLabel}</strong> ${t.voiceBoundary}</p>
      </aside>
      ${directionsOpen ? `<aside class="side-sheet" aria-labelledby="directions-title"><div class="sheet-head"><div><span>${t.routeKicker}</span><h2 id="directions-title">${t.routeTitle}</h2></div><button class="icon-button" data-action="directions-close" aria-label="${t.closeDirections}">${icons.close}</button></div><ol><li><b>1</b><p>${t.routeStep1}<small>${t.routeStep1Note}</small></p></li><li><b>2</b><p>${t.routeStep2}<small>${t.routeStep2Note}</small></p></li><li><b>3</b><p>${t.routeStep3}<small>${t.routeStep3Note}</small></p></li></ol></aside>` : ''}
      ${detailsOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.sourceFreshness}</span><h2>${t.alertDetails}</h2></div><button class="icon-button" data-action="details-close" aria-label="${t.closeDetails}">${icons.close}</button></div><p>${t.demoNotice}</p><dl><div><dt>${t.authorityFormat}</dt><dd>NDMA SACHET / CAP</dd></div><div><dt>${t.issued}</dt><dd>11 Sep 2026, 4:00 PM</dd></div><div><dt>${t.expires}</dt><dd>11 Sep 2026, 6:00 PM</dd></div><div><dt>${t.backend}</dt><dd>${runtimeCopy()}</dd></div></dl></dialog>` : ''}
      ${assistanceOpen ? `<dialog class="modal modal--critical" open><div class="sheet-head"><div><span>${t.emergencyAssistance}</span><h2>${t.call112Question}</h2></div><button class="icon-button" data-action="assist-close" aria-label="${t.close}">${icons.close}</button></div><p>${t.assistNotice}</p><a class="primary-action" href="tel:112">${t.call112Now}</a></dialog>` : ''}
      ${audioOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.listen}</span><h2>${t.approvedAudioUnavailable}</h2></div><button class="icon-button" data-action="audio-close" aria-label="${t.close}">${icons.close}</button></div><p>${t.summary}</p></dialog>` : ''}
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
    places: { type: 'geojson', data: { type: 'FeatureCollection', features: [{ type: 'Feature', geometry: { type: 'Point', coordinates: mapData.user }, properties: { label: t.userMapLabel, kind: 'user' } }, { type: 'Feature', geometry: { type: 'Point', coordinates: mapData.shelter }, properties: { label: t.shelterMapLabel, kind: 'shelter' } }] } },
  }, layers: [
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
      const coordinates = feature?.geometry.type === 'Point' ? feature.geometry.coordinates as [number, number] : null;
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
function focusRoute() { map?.fitBounds(mapData.routeBounds as [[number, number], [number, number]], { padding: window.innerWidth < 768 ? { top: 140, bottom: 290, left: 40, right: 40 } : 90, pitch: mapTilted ? 65 : 0, bearing: mapTilted ? -18 : 0, duration: motionDuration() }); }
function revealMapLayers() {
  if (!map) return;
  const show = () => {
    if (redZonesVisible) { map?.setPaintProperty('hazard-band', 'line-opacity', 0.2); map?.setPaintProperty('hazard-fill', 'fill-opacity', 0.34); map?.setPaintProperty('hazard-edge', 'line-opacity', 0.92); }
    if (relocationZonesVisible) { map?.setPaintProperty('relocation-band', 'line-opacity', 0.2); map?.setPaintProperty('relocation-fill', 'fill-opacity', 0.26); map?.setPaintProperty('relocation-edge', 'line-opacity', 0.92); }
  };
  if (motionDuration() === 0) show(); else requestAnimationFrame(show);
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
function speechLanguage() { return language === 'ML' ? 'ml-IN' : language === 'HI' ? 'hi-IN' : 'en-IN'; }

async function blobToBase64(blob: Blob) {
  const bytes = new Uint8Array(await blob.arrayBuffer());
  let binary = '';
  for (const byte of bytes) binary += String.fromCharCode(byte);
  return btoa(binary);
}

async function transcribeLocally(blob: Blob, durationSeconds: number) {
  voiceFeedbackKey = 'transcribing'; render();
  try {
    const response = await fetch('/api/v2/voice/transcriptions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        audio_base64: await blobToBase64(blob),
        language: speechLanguage(),
        duration_seconds: Math.min(30, Math.max(0.1, durationSeconds)),
        media_type: blob.type.split(';')[0] || 'audio/webm',
      }),
    });
    if (!response.ok) throw new Error(`Transcription service returned ${response.status}`);
    const result = await response.json() as { data?: { text?: string } };
    const transcript = result.data?.text?.trim();
    if (!transcript) throw new Error('Transcription was empty');
    voiceTranscript = transcript;
    await runVoiceCommand(transcript);
  } catch {
    voiceListening = false;
    voiceFeedbackKey = 'recognitionUnavailable';
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
    const mimeType = MediaRecorder.isTypeSupported('audio/webm;codecs=opus') ? 'audio/webm;codecs=opus' : 'audio/webm';
    const chunks: Blob[] = [];
    mediaRecorder = new MediaRecorder(mediaStream, { mimeType });
    mediaRecorder.ondataavailable = (event) => { if (event.data.size) chunks.push(event.data); };
    mediaRecorder.onstop = () => {
      const duration = (Date.now() - recordingStartedAt) / 1000;
      const recording = new Blob(chunks, { type: mimeType });
      mediaStream?.getTracks().forEach((track) => track.stop());
      mediaStream = null; mediaRecorder = null; voiceListening = false;
      void transcribeLocally(recording, duration);
    };
    recordingStartedAt = Date.now();
    voiceListening = true; voiceFeedbackKey = 'recording'; render();
    mediaRecorder.start();
    window.setTimeout(() => { if (mediaRecorder?.state === 'recording') mediaRecorder.stop(); }, 30_000);
  } catch {
    voiceListening = false; voiceFeedbackKey = 'micStopped'; render();
  }
}

let queuedVoiceActions: VoiceMapAction[] = [];

function applyQueuedVoiceActions() {
  const actions = queuedVoiceActions;
  queuedVoiceActions = [];
  for (const action of actions) {
    if (action.type === 'FOCUS_FEATURE' || action.type === 'HIGHLIGHT_FEATURE') {
      if (action.target_id === 'RZ-DEMO-01') map?.fitBounds(mapData.hazardBounds as [[number, number], [number, number]], { padding: 80, duration: motionDuration() });
      if (action.target_id === 'SZ-DEMO-01') map?.easeTo({ center: mapData.shelter as [number, number], zoom: 15, duration: motionDuration() });
      if (action.target_id === 'MY-LOCATION-DEMO') recenterMap();
      if (action.target_id === 'ROUTE-DEMO-01') focusRoute();
    }
    if (action.type === 'FIT_FEATURES') focusRoute();
    if (action.type === 'ZOOM') map?.zoomTo(map.getZoom() + (action.direction === 'IN' ? 1 : -1), { duration: motionDuration() });
    if (action.type === 'PAN') {
      const [lng, lat] = map?.getCenter().toArray() || mapData.user;
      const offsets = { NORTH: [0, .01], SOUTH: [0, -.01], EAST: [.01, 0], WEST: [-.01, 0] } as const;
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
      if (action.panel === 'ROUTE_GUIDANCE') directionsOpen = true;
      if (action.panel === 'EMERGENCY_CALL_CONFIRMATION') assistanceOpen = true;
    }
    if (action.type === 'SET_LANGUAGE') language = action.language === 'ml-IN' ? 'ML' : action.language === 'hi-IN' ? 'HI' : 'EN';
  }
}

async function runVoiceCommand(raw: string) {
  const text = raw.trim();
  voiceTranscript = text;
  if (!text || commandPending) { if (!text) voiceFeedbackKey = 'askFirst'; render(); return; }
  commandPending = true; commandError = ''; voiceFeedbackKey = 'checkingBackend'; render();
  try {
    const response = await fetch('/api/v2/voice/commands', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ transcript: text, language: speechLanguage(), confidence: 1 }) });
    if (!response.ok) throw new Error(`Voice command service returned ${response.status}`);
    const result = await response.json() as Record<string, unknown>;
    const candidate = (result.data || result) as Record<string, unknown>;
    const validated = validateVoiceResponse(candidate);
    if (!validated) throw new Error('Voice command response failed validation');
    commandResponse = typeof candidate.screen_response === 'string' ? candidate.screen_response : words[language].responseReady;
    voiceFeedbackKey = 'responseReady';
    applyVoiceActions(validated.actions);
    commandSuggestions = [...words[language].voiceCommands];
    commandPending = false; render();
  } catch {
    commandPending = false;
    commandError = words[language].commandUnavailable;
    voiceFeedbackKey = 'backendUnavailable';
    render();
  }
}

function toggleListening() {
  if (language === 'EN' || !localAsrReady) { voiceFeedbackKey = 'recognitionUnavailable'; render(); return; }
  void toggleLocalRecording();
}

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((b) => b.addEventListener('click', () => {
    const nextLanguage = b.dataset.language as Language;
    language = nextLanguage;
    try { localStorage.setItem('sthira-language', language); } catch { /* Continue without storage. */ }
    commandSuggestions = [...words[language].voiceCommands];
    commandResponse = words[language].voiceReady;
    commandError = '';
    voiceFeedbackKey = 'micPrivacy';
    render();
  }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="voice-open"]').forEach((b) => b.addEventListener('click', () => { voiceOpen = true; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => {
    if (mediaRecorder?.state === 'recording') mediaRecorder.stop();
    voiceListening = false; voiceOpen = false; render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', toggleListening);
  document.querySelectorAll<HTMLButtonElement>('[data-command]').forEach((b) => b.addEventListener('click', () => void runVoiceCommand(b.dataset.command || '')));
  document.querySelector<HTMLFormElement>('[data-command-form]')?.addEventListener('submit', (e) => { e.preventDefault(); void runVoiceCommand(String(new FormData(e.currentTarget as HTMLFormElement).get('command') || '')); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="route"]').forEach((button) => button.addEventListener('click', () => { routeStarted = true; directionsOpen = true; render(); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="start-route"]').forEach((button) => button.addEventListener('click', () => { routeStarted = true; directionsOpen = true; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="toggle-3d"]')?.addEventListener('click', toggleMapPerspective);
  document.querySelector<HTMLButtonElement>('[data-action="recenter"]')?.addEventListener('click', recenterMap);
  document.querySelector<HTMLButtonElement>('[data-action="toggle-layers"]')?.addEventListener('click', () => { layersOpen = !layersOpen; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="toggle-red-zones"]')?.addEventListener('click', () => { redZonesVisible = !redZonesVisible; pendingZoneFocus = redZonesVisible ? 'RED' : null; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="toggle-relocation-zones"]')?.addEventListener('click', () => { relocationZonesVisible = !relocationZonesVisible; pendingZoneFocus = relocationZonesVisible ? 'RELOCATION' : null; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="directions"]').forEach((b) => b.addEventListener('click', () => { directionsOpen = true; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="directions-close"]')?.addEventListener('click', () => { directionsOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', () => { audioOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="audio-close"]')?.addEventListener('click', () => { audioOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="isl"]')?.addEventListener('click', () => { islOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="isl-close"]')?.addEventListener('click', () => { islOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="details"]')?.addEventListener('click', () => { detailsOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="details-close"]')?.addEventListener('click', () => { detailsOpen = false; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="assist-open"]').forEach((button) => button.addEventListener('click', () => { assistanceOpen = true; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="assist-close"]')?.addEventListener('click', () => { assistanceOpen = false; render(); });
  document.querySelectorAll<HTMLAnchorElement>('a[href="tel:112"]').forEach((a) => a.addEventListener('click', (e) => { if (!assistanceOpen) { e.preventDefault(); assistanceOpen = true; render(); } }));
}

async function checkRuntime() {
  if (!navigator.onLine) return;
  try {
    const [statusResponse, readinessResponse, voiceResponse] = await Promise.all([fetch('/api/v2/status'), fetch('/api/v2/health/readiness'), fetch('/api/v2/voice/status')]);
    const statusData = await statusResponse.json();
    const voiceData = await voiceResponse.json() as VoiceStatus;
    localAsrReady = Boolean(voiceData.data?.ready);
    runtime = readinessResponse.ok ? 'demo' : 'blocked'; runtimeDetail = statusData?.source_status === 'NO_LIVE_GOVERNMENT_SOURCE_CONFIGURED' ? 'blocked' : 'responding';
  } catch { runtime = 'demo'; runtimeDetail = 'disconnected'; }
  render();
}

render(); void checkRuntime();
window.addEventListener('online', () => { runtime = 'checking'; void checkRuntime(); });
window.addEventListener('offline', () => { runtime = 'offline'; render(); });
if ('serviceWorker' in navigator) void navigator.serviceWorker.register('/sw.js').catch(() => undefined);
