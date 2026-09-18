import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Map, Popup } from 'maplibre-gl';
import mapData from './mapData.json';
import { words, type Language } from './i18n';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
import '@fontsource/noto-sans-malayalam/400.css';
import '@fontsource/noto-sans-malayalam/700.css';
import '@fontsource/noto-sans-devanagari/400.css';
import '@fontsource/noto-sans-devanagari/700.css';

type RuntimeState = 'checking' | 'demo' | 'blocked' | 'offline';
type ChatMessage = { role: 'USER' | 'ASSISTANT'; text: string };
type MapAction = 'NONE' | 'FOCUS_SHELTER' | 'SHOW_ROUTE' | 'SHOW_HAZARD' | 'OPEN_DIRECTIONS' | 'OPEN_RESCUE' | 'CONFIRM_ARRIVAL';
type ChatResponse = { reply: string; map_action: MapAction; suggestions: string[]; source_status: 'SYNTHETIC_DEMO' };
type VoiceStatus = { data?: { ready?: boolean; supported_languages?: string[] } };

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
let localAsrReady = false;
let chatPending = false;
let chatError = '';
let pendingMapAction: MapAction = 'NONE';
let chatSuggestions: string[] = [...words.EN.suggestions];
let chatMessages: ChatMessage[] = [{ role: 'ASSISTANT', text: words.EN.welcome }];
const chatSessionId = globalThis.crypto?.randomUUID?.() || `browser-${Date.now()}`;
let routeStarted = false;
let directionsOpen = false;
let detailsOpen = false;
let arrivalOpen = false;
let assistanceOpen = false;
let partySize = 1;
let arrivalSuccess = false;

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

function render() {
  const t = words[language];
  if (mapAnimationFrame !== null) cancelAnimationFrame(mapAnimationFrame);
  mapAnimationFrame = null;
  map?.remove();
  document.documentElement.lang = language === 'ML' ? 'ml' : language === 'HI' ? 'hi' : 'en';
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell ${hasRendered ? '' : 'is-entering'}">
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
          <div class="quick-actions"><button type="button" data-action="directions">${icons.route}<span>${t.directions}</span></button><button type="button" data-action="listen">${icons.volume}<span>${t.listen}</span></button><button type="button" data-action="voice-open">${icons.mic}<span>${t.askByVoice}</span></button></div>
          <button class="arrival-action" data-testid="arrival-confirmation" type="button" data-action="arrival-open">${t.arrived}</button>
          <a class="rescue-action" data-testid="call-112" href="tel:112"><span>${t.trapped}</span><strong>${t.rescue}</strong></a>
        </section>
        <section class="map-surface" aria-label="${t.mapAria}">
          <div id="map-canvas"></div>
          <div class="map-toolbar" aria-label="${t.mapTools}"><button type="button" data-action="recenter">${icons.locate}<span>${t.myLocation}</span></button><button type="button" data-action="map-route">${icons.route}<span>${t.fullRoute}</span></button></div>
          <div class="layer-switcher" aria-label="${t.mapLayers}"><button class="zone-toggle zone-toggle--danger ${redZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-red-zones" aria-pressed="${redZonesVisible}"><i></i><span>${t.redZones}</span></button><button class="zone-toggle zone-toggle--relocation ${relocationZonesVisible ? 'is-active' : ''}" type="button" data-action="toggle-relocation-zones" aria-pressed="${relocationZonesVisible}"><i></i><span>${t.relocationZones}</span></button></div>
          <div class="map-key"><span class="${redZonesVisible ? '' : 'is-muted'}"><i class="hazard-key"></i>${t.redZone}</span><span><i class="route-key"></i>${t.approvedRoute}</span><span class="${relocationZonesVisible ? '' : 'is-muted'}"><i class="relocation-key"></i>${t.relocationZone}</span><span><i class="shelter-key"></i>${t.safeShelter}</span></div>
          <div class="map-disclaimer">${t.imagery} <a href="https://www.esri.com/" target="_blank" rel="noreferrer">© Esri</a>, ${t.overlays}</div>
        </section>
      </main>
      <aside class="voice-console ${voiceOpen ? 'is-open' : ''}" role="dialog" aria-modal="false" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}>
        <div class="voice-head"><div><span>${t.assistant}</span><h2 id="voice-title">${t.askSituation}</h2></div><button class="icon-button" type="button" data-action="voice-close" aria-label="${t.closeAssistant}">${icons.close}</button></div>
        <div class="voice-stage ${voiceListening ? 'is-listening' : ''}"><div class="voice-orb" aria-hidden="true">${icons.mic}<i></i><i></i><i></i></div><div><strong>${voiceListening ? t.listening : chatPending ? t.checkingGuidance : t.ready}</strong><span>${voiceListening ? t.speakNaturally : t[voiceFeedbackKey]}</span></div></div>
        <div class="chat-thread" aria-live="polite" aria-busy="${chatPending}">${chatMessages.map((message, index) => `<article class="chat-message chat-message--${message.role.toLowerCase()}"><span>${message.role === 'USER' ? t.you : 'Sthira'}</span><p>${escapeHtml(message.text)}</p>${message.role === 'ASSISTANT' ? `<button type="button" data-speak-message="${index}" aria-label="${t.readAloud}">${icons.volume}<span>${t.listen}</span></button>` : ''}</article>`).join('')}${chatPending ? `<div class="chat-thinking"><i></i><i></i><i></i><span>${t.checkingExercise}</span></div>` : ''}</div>
        ${chatError ? `<p class="chat-error" role="alert">${escapeHtml(chatError)}</p>` : ''}
        <div class="voice-suggestions" aria-label="${t.suggestedQuestions}">${chatSuggestions.map((suggestion) => `<button type="button" data-command="${escapeHtml(suggestion)}">${escapeHtml(suggestion)}</button>`).join('')}</div>
        <form class="command-form" data-command-form><label for="command-input">${t.askText}</label><div><input id="command-input" name="command" autocomplete="off" placeholder="${t.askPlaceholder}" ${chatPending ? 'disabled' : ''}/><button type="submit" ${chatPending ? 'disabled' : ''}>${t.send}</button></div></form>
        <button class="listen-button ${voiceListening ? 'is-listening' : ''}" type="button" data-action="voice-listen" aria-pressed="${voiceListening}">${icons.mic}<span>${voiceListening ? t.stopListening : t.startListening}</span></button>
        <p class="voice-boundary"><strong>${t.voiceBoundaryLabel}</strong> ${t.voiceBoundary}</p>
      </aside>
      ${directionsOpen ? `<aside class="side-sheet" aria-labelledby="directions-title"><div class="sheet-head"><div><span>${t.routeKicker}</span><h2 id="directions-title">${t.routeTitle}</h2></div><button class="icon-button" data-action="directions-close" aria-label="${t.closeDirections}">${icons.close}</button></div><ol><li><b>1</b><p>${t.routeStep1}<small>${t.routeStep1Note}</small></p></li><li><b>2</b><p>${t.routeStep2}<small>${t.routeStep2Note}</small></p></li><li><b>3</b><p>${t.routeStep3}<small>${t.routeStep3Note}</small></p></li></ol></aside>` : ''}
      ${detailsOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.sourceFreshness}</span><h2>${t.alertDetails}</h2></div><button class="icon-button" data-action="details-close" aria-label="${t.closeDetails}">${icons.close}</button></div><p>${t.demoNotice}</p><dl><div><dt>${t.authorityFormat}</dt><dd>NDMA SACHET / CAP</dd></div><div><dt>${t.issued}</dt><dd>11 Sep 2026, 4:00 PM</dd></div><div><dt>${t.expires}</dt><dd>11 Sep 2026, 6:00 PM</dd></div><div><dt>${t.backend}</dt><dd>${runtimeCopy()}</dd></div></dl></dialog>` : ''}
      ${arrivalOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.arrivalCheck}</span><h2>${t.arrivedSafely}</h2></div><button class="icon-button" data-action="arrival-close" aria-label="${t.closeArrival}">${icons.close}</button></div>${arrivalSuccess ? `<div class="success-message">${t.arrivalRecorded}</div>` : `<p>${t.confirmParty}</p><div class="stepper"><button type="button" data-action="party-minus" aria-label="${t.decreaseParty}">-</button><strong>${partySize} ${partySize === 1 ? t.person : t.people}</strong><button type="button" data-action="party-plus" aria-label="${t.increaseParty}">+</button></div><div class="modal-actions"><button class="primary-action" type="button" data-action="arrival-yes">${t.weArrived}</button><button class="secondary-action" type="button" data-action="arrival-no">${t.needHelp}</button></div>`}</dialog>` : ''}
      ${assistanceOpen ? `<dialog class="modal" open><div class="sheet-head"><div><span>${t.emergencyAssistance}</span><h2>${t.call112Question}</h2></div><button class="icon-button" data-action="assist-close" aria-label="${t.close}">${icons.close}</button></div><p>${t.assistNotice}</p><a class="primary-action" href="tel:112">${t.call112Now}</a></dialog>` : ''}
      <div class="toast" role="status" aria-live="polite" hidden></div>
    </div>`;
  hasRendered = true;
  bindInteractions(); initMap();
  requestAnimationFrame(() => { const thread = document.querySelector<HTMLElement>('.chat-thread'); if (thread) thread.scrollTop = thread.scrollHeight; });
}

function mapColor(token: string) { return getComputedStyle(document.documentElement).getPropertyValue(token).trim(); }
function initMap() {
  const container = document.querySelector<HTMLElement>('#map-canvas'); if (!container) return;
  const t = words[language];
  map = new Map({ container, center: [76.112, 11.562], zoom: 13.4, attributionControl: false, style: { version: 8, sources: {
    basemap: { type: 'raster', tiles: ['https://server.arcgisonline.com/ArcGIS/rest/services/World_Imagery/MapServer/tile/{z}/{y}/{x}'], tileSize: 256, attribution: 'Imagery © Esri' },
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
  map.once('load', () => {
    map?.resize();
    if (routeStarted) focusRoute();
    revealMapLayers();
    applyPendingMapAction();
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
function focusRoute() { map?.fitBounds(mapData.routeBounds as [[number, number], [number, number]], { padding: 90, duration: motionDuration() }); }
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
function speakText(text: string) { if (!('speechSynthesis' in window)) return; window.speechSynthesis.cancel(); const u = new SpeechSynthesisUtterance(text); u.lang = language === 'ML' ? 'ml-IN' : language === 'HI' ? 'hi-IN' : 'en-IN'; window.speechSynthesis.speak(u); }
function speakInstruction() { speakText(words[language].summary); }

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
    await sendChat(transcript, true);
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

function applyChatAction(action: MapAction) {
  pendingMapAction = action;
  if (action === 'SHOW_HAZARD') redZonesVisible = true;
  if (action === 'SHOW_ROUTE' || action === 'OPEN_DIRECTIONS') routeStarted = true;
  if (action === 'OPEN_DIRECTIONS') directionsOpen = true;
  if (action === 'OPEN_RESCUE') assistanceOpen = true;
  if (action === 'CONFIRM_ARRIVAL') { arrivalSuccess = false; arrivalOpen = true; }
}

function applyPendingMapAction() {
  const action = pendingMapAction;
  pendingMapAction = 'NONE';
  if (action === 'FOCUS_SHELTER') map?.easeTo({ center: mapData.shelter as [number, number], zoom: 15, duration: motionDuration() });
  if (action === 'SHOW_ROUTE' || action === 'OPEN_DIRECTIONS') focusRoute();
  if (action === 'SHOW_HAZARD') map?.fitBounds(mapData.hazardBounds as [[number, number], [number, number]], { padding: 80, duration: motionDuration() });
  if (pendingZoneFocus === 'RED') map?.fitBounds(mapData.hazardBounds as [[number, number], [number, number]], { padding: 70, duration: motionDuration() });
  if (pendingZoneFocus === 'RELOCATION') map?.fitBounds(mapData.relocationBounds as [[number, number], [number, number]], { padding: 70, duration: motionDuration() });
  pendingZoneFocus = null;
}

async function sendChat(raw: string, readReply = false) {
  const text = raw.trim();
  voiceTranscript = text;
  if (!text || chatPending) { if (!text) voiceFeedbackKey = 'askFirst'; render(); return; }
  chatMessages.push({ role: 'USER', text });
  chatPending = true; chatError = ''; voiceFeedbackKey = 'checkingBackend'; render();
  try {
    const response = await fetch('/api/v2/guidance/chat', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ session_id: chatSessionId, language, messages: chatMessages.slice(-20) }) });
    if (!response.ok) throw new Error(`Guidance service returned ${response.status}`);
    const result = await response.json() as ChatResponse;
    chatMessages.push({ role: 'ASSISTANT', text: result.reply });
    chatSuggestions = result.suggestions;
    voiceFeedbackKey = 'responseReady';
    applyChatAction(result.map_action);
    chatPending = false; render();
    if (readReply) speakText(result.reply);
  } catch {
    chatPending = false;
    chatError = words[language].assistantUnavailable;
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
    if (chatMessages.length === 1 && chatMessages[0].role === 'ASSISTANT') chatMessages = [{ role: 'ASSISTANT', text: words[nextLanguage].welcome }];
    language = nextLanguage;
    chatSuggestions = [...words[language].suggestions];
    chatError = '';
    voiceFeedbackKey = 'micPrivacy';
    render();
  }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="voice-open"]').forEach((b) => b.addEventListener('click', () => { voiceOpen = true; render(); document.querySelector<HTMLInputElement>('#command-input')?.focus(); }));
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => {
    if (mediaRecorder?.state === 'recording') mediaRecorder.stop();
    voiceListening = false; voiceOpen = false; render();
  });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', toggleListening);
  document.querySelectorAll<HTMLButtonElement>('[data-command]').forEach((b) => b.addEventListener('click', () => void sendChat(b.dataset.command || '')));
  document.querySelectorAll<HTMLButtonElement>('[data-speak-message]').forEach((b) => b.addEventListener('click', () => { const message = chatMessages[Number(b.dataset.speakMessage)]; if (message) speakText(message.text); }));
  document.querySelector<HTMLFormElement>('[data-command-form]')?.addEventListener('submit', (e) => { e.preventDefault(); void sendChat(String(new FormData(e.currentTarget as HTMLFormElement).get('command') || '')); });
  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => { routeStarted = true; directionsOpen = true; render(); focusRoute(); });
  document.querySelector<HTMLButtonElement>('[data-action="map-route"]')?.addEventListener('click', focusRoute);
  document.querySelector<HTMLButtonElement>('[data-action="recenter"]')?.addEventListener('click', () => map?.easeTo({ center: mapData.user as [number, number], zoom: 15, duration: motionDuration() }));
  document.querySelector<HTMLButtonElement>('[data-action="toggle-red-zones"]')?.addEventListener('click', () => { redZonesVisible = !redZonesVisible; pendingZoneFocus = redZonesVisible ? 'RED' : null; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="toggle-relocation-zones"]')?.addEventListener('click', () => { relocationZonesVisible = !relocationZonesVisible; pendingZoneFocus = relocationZonesVisible ? 'RELOCATION' : null; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="directions"]').forEach((b) => b.addEventListener('click', () => { directionsOpen = true; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="directions-close"]')?.addEventListener('click', () => { directionsOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', speakInstruction);
  document.querySelector<HTMLButtonElement>('[data-action="details"]')?.addEventListener('click', () => { detailsOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="details-close"]')?.addEventListener('click', () => { detailsOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-open"]')?.addEventListener('click', () => { arrivalSuccess = false; arrivalOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => { arrivalOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="party-minus"]')?.addEventListener('click', () => { partySize = Math.max(1, partySize - 1); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="party-plus"]')?.addEventListener('click', () => { partySize = Math.min(10, partySize + 1); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-yes"]')?.addEventListener('click', () => { arrivalSuccess = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-no"]')?.addEventListener('click', () => { arrivalOpen = false; assistanceOpen = true; render(); });
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
