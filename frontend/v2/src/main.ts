import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Map } from 'maplibre-gl';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
import '@fontsource/noto-sans-malayalam/400.css';
import '@fontsource/noto-sans-malayalam/600.css';
import '@fontsource/noto-sans-malayalam/700.css';
import '@fontsource/noto-sans-devanagari/400.css';
import '@fontsource/noto-sans-devanagari/600.css';
import '@fontsource/noto-sans-devanagari/700.css';

type Language = 'EN' | 'ML' | 'HI';

const copy = {
  EN: {
    lang: 'English', demo: 'DEMO ENVIRONMENT · FEED ACTIVE', alert: 'CRITICAL ALERT · NDMA SACHET · FLOOD', hazard: 'FLOOD', live: 'Live · 2 min ago',
    headline: 'Evacuate to Government School, Ward 8', summary: 'Severe flooding affects Ward 12. Move now using the approved route.',
    steps: ['Follow the safe route shown below.', 'Avoid low-lying roads and floodwater.', 'Take medicines & ID.'], shelter: 'Government School, Ward 8',
    open: 'OPEN · CAPACITY VERIFIED', deadline: 'Deadline', assigned: 'Assigned shelter', routeMetric: 'Distance / steps', deadlineValue: 'Leave before 6:00 PM', assignedValue: 'Ward 8 Govt School', routeValue: 'Official route active',
    start: 'START SAFE ROUTE', call: 'CALL 112', listen: 'Listen', isl: 'ISL Video', directions: 'Step-by-step directions', details: 'View alert details',
    source: 'NDMA SACHET · Synthetic Demo Package v2', mapNote: 'Synthetic map canvas · official basemap pending authorization', mapAria: 'Synthetic map showing red flood zone, citizen position, approved route, and safe shelter',
    voice: 'Voice Control', voiceOpen: 'Open Voice Map Control', voiceTitle: 'Voice-to-text control', voiceBody: 'Approved short commands will move the map when IndicConformer is connected.', voicePending: 'Model connection pending', listenStart: 'START LISTENING', listenStop: 'STOP LISTENING', transcript: 'Transcript appears here',
    close: 'Close', drawer: 'Expand guidance', collapse: 'Collapse guidance', arrivalTitle: 'Have you arrived at Government School, Ward 8?', arrivalBody: 'This preview does not change capacity. Arrival confirmation will be connected in Phase 6.', yes: 'YES, I HAVE ARRIVED', no: 'NO, I NEED HELP',
    offline: 'OFFLINE · LAST VALID DEMO DATA', online: 'DEMO FEED ACTIVE', detailsTitle: 'Alert details', detailsBody: 'Synthetic data only. This is not an active government alert.', issued: 'Issued 11 Sep 2026 · 4:00 PM', expires: 'Expires 11 Sep 2026 · 6:00 PM', detailsClose: 'Close alert details',
  },
  ML: {
    lang: 'മലയാളം', demo: 'ഡെമോ പരിസ്ഥിതി · ഫീഡ് സജീവം', alert: 'ഗുരുതര മുന്നറിയിപ്പ് · NDMA SACHET · വെള്ളപ്പൊക്കം', hazard: 'വെള്ളപ്പൊക്കം', live: 'സജീവം · 2 മിനിറ്റ് മുമ്പ്',
    headline: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8-ലേക്ക് മാറുക', summary: 'വാർഡ് 12-ൽ ഗുരുതര വെള്ളപ്പൊക്കം. അനുവദിച്ച മാർഗ്ഗത്തിലൂടെ ഇപ്പോൾ മാറുക.',
    steps: ['താഴെ കാണുന്ന സുരക്ഷിത മാർഗ്ഗം പിന്തുടരുക.', 'താഴ്ന്ന റോഡുകളും വെള്ളവും ഒഴിവാക്കുക.', 'മരുന്നുകളും തിരിച്ചറിയൽ രേഖകളും എടുക്കുക.'], shelter: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8',
    open: 'തുറന്നിരിക്കുന്നു · ശേഷി പരിശോധിച്ചു', deadline: 'അവസാന സമയം', assigned: 'നിയോഗിച്ച കേന്ദ്രം', routeMetric: 'ദൂരം / ഘട്ടങ്ങൾ', deadlineValue: 'പുറപ്പെടുക: വൈകിട്ട് 6:00', assignedValue: 'വാർഡ് 8 സർക്കാർ സ്കൂൾ', routeValue: 'ഔദ്യോഗിക മാർഗ്ഗം സജീവം',
    start: 'സുരക്ഷിത മാർഗ്ഗം തുടങ്ങുക', call: '112 വിളിക്കുക', listen: 'കേൾക്കുക', isl: 'ISL വീഡിയോ', directions: 'ഘട്ടംഘട്ടമായ നിർദ്ദേശങ്ങൾ', details: 'മുന്നറിയിപ്പ് വിവരങ്ങൾ',
    source: 'NDMA SACHET · സിന്തറ്റിക് ഡെമോ പാക്കേജ് v2', mapNote: 'സിന്തറ്റിക് മാപ്പ് · ഔദ്യോഗിക ബേസ്മാപ്പ് അനുമതി കാത്തിരിക്കുന്നു', mapAria: 'ചുവപ്പ് വെള്ളപ്പൊക്ക മേഖല, പൗരന്റെ സ്ഥാനം, അനുവദിച്ച മാർഗ്ഗം, സുരക്ഷിത കേന്ദ്രം കാണിക്കുന്ന സിന്തറ്റിക് മാപ്പ്',
    voice: 'വോയ്സ് നിയന്ത്രണം', voiceOpen: 'വോയ്സ് മാപ്പ് നിയന്ത്രണം തുറക്കുക', voiceTitle: 'വോയ്സ്-ടു-ടെക്സ്റ്റ് നിയന്ത്രണം', voiceBody: 'IndicConformer ബന്ധിപ്പിച്ചാൽ അംഗീകൃത ഹ്രസ്വ കമാൻഡുകൾ മാപ്പ് നീക്കും.', voicePending: 'മോഡൽ കണക്ഷൻ കാത്തിരിക്കുന്നു', listenStart: 'കേൾക്കാൻ തുടങ്ങുക', listenStop: 'കേൾക്കുന്നത് നിർത്തുക', transcript: 'ട്രാൻസ്‌ക്രിപ്റ്റ് ഇവിടെ കാണിക്കും',
    close: 'അടയ്ക്കുക', drawer: 'മാർഗ്ഗനിർദ്ദേശം വികസിപ്പിക്കുക', collapse: 'മാർഗ്ഗനിർദ്ദേശം ചുരുക്കുക', arrivalTitle: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8-ൽ എത്തിയോ?', arrivalBody: 'ഈ പ്രിവ്യൂ ശേഷി മാറ്റില്ല. എത്തിച്ചേരൽ സ്ഥിരീകരണം ഘട്ടം 6-ൽ ബന്ധിപ്പിക്കും.', yes: 'അതെ, എത്തി', no: 'ഇല്ല, സഹായം വേണം',
    offline: 'ഓഫ്‌ലൈൻ · അവസാനത്തെ സാധുവായ ഡെമോ ഡാറ്റ', online: 'ഡെമോ ഫീഡ് സജീവം', detailsTitle: 'മുന്നറിയിപ്പ് വിവരങ്ങൾ', detailsBody: 'സിന്തറ്റിക് ഡാറ്റ മാത്രം. ഇത് സജീവ സർക്കാർ മുന്നറിയിപ്പല്ല.', issued: 'നൽകിയത് 11 സെപ്റ്റംബർ 2026 · വൈകിട്ട് 4:00', expires: 'കാലാവധി 11 സെപ്റ്റംബർ 2026 · വൈകിട്ട് 6:00', detailsClose: 'മുന്നറിയിപ്പ് അടയ്ക്കുക',
  },
  HI: {
    lang: 'हिन्दी', demo: 'डेमो वातावरण · फीड सक्रिय', alert: 'गंभीर अलर्ट · NDMA SACHET · बाढ़', hazard: 'बाढ़', live: 'सक्रिय · 2 मिनट पहले',
    headline: 'सरकारी स्कूल, वार्ड 8 में जाएं', summary: 'वार्ड 12 में गंभीर बाढ़ है। अनुमोदित मार्ग से अभी जाएं.',
    steps: ['नीचे दिखाए सुरक्षित मार्ग का पालन करें।', 'निचली सड़कों और बाढ़ के पानी से बचें।', 'दवाइयां और पहचान पत्र साथ लें।'], shelter: 'सरकारी स्कूल, वार्ड 8',
    open: 'खुला · क्षमता सत्यापित', deadline: 'समय सीमा', assigned: 'निर्धारित आश्रय', routeMetric: 'दूरी / चरण', deadlineValue: 'पहले निकलें: शाम 6:00', assignedValue: 'वार्ड 8 सरकारी स्कूल', routeValue: 'अनुमोदित मार्ग सक्रिय',
    start: 'सुरक्षित मार्ग शुरू करें', call: '112 पर कॉल करें', listen: 'सुनें', isl: 'ISL वीडियो', directions: 'चरण-दर-चरण निर्देश', details: 'अलर्ट विवरण',
    source: 'NDMA SACHET · सिंथेटिक डेमो पैकेज v2', mapNote: 'सिंथेटिक मानचित्र · सरकारी बेसमैप अनुमति तक लंबित', mapAria: 'लाल बाढ़ क्षेत्र, नागरिक स्थान, अनुमोदित मार्ग और सुरक्षित आश्रय दिखाने वाला सिंथेटिक मानचित्र',
    voice: 'वॉयस कंट्रोल', voiceOpen: 'वॉयस मैप कंट्रोल खोलें', voiceTitle: 'वॉयस-टू-टेक्स्ट कंट्रोल', voiceBody: 'IndicConformer जुड़ने पर अनुमोदित छोटे कमांड मानचित्र को स्थानांतरित करेंगे.', voicePending: 'मॉडल कनेक्शन लंबित', listenStart: 'सुनना शुरू करें', listenStop: 'सुनना बंद करें', transcript: 'ट्रांसक्रिप्ट यहां दिखाई देगा',
    close: 'बंद करें', drawer: 'मार्गदर्शन खोलें', collapse: 'मार्गदर्शन बंद करें', arrivalTitle: 'क्या आप सरकारी स्कूल, वार्ड 8 पहुंच गए?', arrivalBody: 'यह प्रीव्यू क्षमता नहीं बदलता। पहुंच की पुष्टि चरण 6 में जोड़ी जाएगी.', yes: 'हां, मैं पहुंच गया', no: 'नहीं, मदद चाहिए',
    offline: 'ऑफलाइन · अंतिम मान्य डेमो डेटा', online: 'डेमो फीड सक्रिय', detailsTitle: 'अलर्ट विवरण', detailsBody: 'केवल सिंथेटिक डेटा। यह सक्रिय सरकारी अलर्ट नहीं है.', issued: 'जारी 11 सितम्बर 2026 · शाम 4:00', expires: 'समाप्ति 11 सितम्बर 2026 · शाम 6:00', detailsClose: 'अलर्ट विवरण बंद करें',
  },
} as const;

let language: Language = 'EN';
let voiceOpen = false;
let voiceListening = false;
let drawerExpanded = true;
let arrivalOpen = false;
let detailsOpen = false;
let routeStarted = false;
let isOffline = !navigator.onLine;
let map: Map | null = null;

function render() {
  const t = copy[language];
  map?.remove();
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell tactical-shell ${drawerExpanded ? 'drawer-expanded' : 'drawer-collapsed'}">
      <div class="tactical-map" role="img" aria-label="${t.mapAria}">
        <div id="map-canvas" aria-hidden="true"></div><div class="map-noise"></div><div class="map-contours"></div><div class="hazard-zone"><span>⚠ HIGH HAZARD · WARD 12 FLOOD INUNDATION</span></div><div class="route-trail ${routeStarted ? 'route-active' : ''}"></div><div class="route-dots"><i></i><i></i><i></i><i></i></div><div class="citizen-pin">S</div><div class="shelter-marker"><span>⌂</span><small>WARD 8</small><b>SAFE SHELTER</b></div>
        <div class="map-controls" aria-label="Map controls"><button type="button" aria-label="Zoom in" data-action="zoom-in">+</button><button type="button" aria-label="Zoom out" data-action="zoom-out">−</button></div><div class="map-attribution">${t.source}</div>
      </div>
      <header class="tactical-topbar">
        <a class="wordmark" href="/" aria-label="Sthira home"><span class="wordmark-mark">S</span><span>Sthira <small>v2</small></span></a>
        <div class="topbar-center"><span class="heartbeat"><i></i>${isOffline ? t.offline : t.demo}</span></div>
        <div class="topbar-actions"><button class="voice-control ${voiceListening ? 'is-listening' : ''}" type="button" aria-label="${t.voiceOpen}" aria-expanded="${voiceOpen}" data-action="voice"><span class="mic-icon">●</span><span>${t.voice}</span></button><div class="language-switcher" role="group" aria-label="Language"><button class="language-choice ${language === 'EN' ? 'is-selected' : ''}" data-language="EN" type="button">EN</button><button class="language-choice ${language === 'ML' ? 'is-selected' : ''}" data-language="ML" type="button">ML</button><button class="language-choice ${language === 'HI' ? 'is-selected' : ''}" data-language="HI" type="button">HI</button></div></div>
      </header>
      <main class="tactical-main">
        <section class="floating-drawer" data-testid="emergency-card" aria-labelledby="alert-title">
          <button class="drawer-handle" type="button" aria-expanded="${drawerExpanded}" aria-label="${drawerExpanded ? t.collapse : t.drawer}" data-action="drawer"><span></span></button>
          <div class="drawer-scroll">
            <div class="alert-row"><span class="critical-badge"><i></i>${t.alert}</span><span class="live-stamp">${t.live}</span></div>
            <div class="alert-heading"><div><p class="overline">SYNTHETIC_DEMO · SOURCE VERIFIED FOR UI ONLY</p><h1 id="alert-title">${t.headline}</h1><p class="summary">${t.summary}</p></div><button class="detail-trigger" type="button" data-action="details" aria-label="${t.details}">ⓘ</button></div>
            <ol class="timeline">${t.steps.map((step, index) => `<li><span class="timeline-dot">${index + 1}</span><p>${step}</p></li>`).join('')}</ol>
            <article class="shelter-card"><div class="shelter-header"><div class="shelter-icon">⌂</div><div><h2>${t.shelter}</h2><span class="open-badge">✓ ${t.open}</span></div></div><div class="shelter-grid"><div><span>${t.deadline}</span><strong>${t.deadlineValue}</strong></div><div><span>${t.assigned}</span><strong>${t.assignedValue}</strong></div><div><span>${t.routeMetric}</span><strong>${t.routeValue}</strong></div></div></article>
            <div class="action-bar"><button class="start-button ${routeStarted ? 'is-started' : ''}" type="button" data-testid="start-route" data-action="route"><span>${routeStarted ? '✓' : '↗'}</span>${routeStarted ? 'ROUTE ACTIVE' : t.start}</button><a class="call-button" data-testid="call-112" href="tel:112"><span>☎</span>${t.call}</a></div>
            <div class="utility-row"><button type="button" data-action="listen">▶ ${t.listen}</button><button type="button" data-action="isl">◉ ${t.isl}</button><button type="button" data-action="directions">☷ ${t.directions}</button></div>
          </div>
        </section>
      </main>
      <div class="map-legend-floating"><span><i class="legend-crimson"></i>${t.hazard}</span><span><i class="legend-cobalt"></i>${t.routeValue}</span><span><i class="legend-mint"></i>${t.shelter}</span></div>
      <footer class="tactical-footer"><span>${t.mapNote}</span><button type="button" data-action="details">${t.details}</button></footer>
      <div class="toast" role="status" aria-live="polite" hidden></div>
      <dialog class="tactical-dialog" aria-labelledby="details-title" ${detailsOpen ? 'open' : ''}><div class="dialog-top"><h2 id="details-title">${t.detailsTitle}</h2><button type="button" aria-label="${t.detailsClose}" data-action="details-close">×</button></div><p>${t.detailsBody}</p><dl><div><dt>${t.issued}</dt><dd>${t.source}</dd></div><div><dt>${t.expires}</dt></div></dl><button class="start-button" type="button" data-action="details-close">${t.detailsClose}</button></dialog>
      <dialog class="arrival-dialog" aria-labelledby="arrival-title" ${arrivalOpen ? 'open' : ''}><div class="dialog-top"><span class="arrival-icon">?</span><button type="button" aria-label="${t.close}" data-action="arrival-close">×</button></div><h2 id="arrival-title">${t.arrivalTitle}</h2><p>${t.arrivalBody}</p><div class="arrival-actions"><button class="arrived-button" type="button" data-action="arrival-yes">${t.yes}</button><button class="help-button" type="button" data-action="arrival-no">${t.no}</button></div></dialog>
      <aside class="voice-panel ${voiceOpen ? 'is-open' : ''}" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}><div class="dialog-top"><div><p class="overline">${t.voice}</p><h2 id="voice-title">${t.voiceTitle}</h2></div><button type="button" aria-label="${t.close}" data-action="voice-close">×</button></div><p>${t.voiceBody}</p><div class="voice-pending">● ${t.voicePending}</div><div class="voice-transcript" aria-live="polite"><span>${t.transcript}</span><strong>${voiceListening ? 'Listening preview...' : 'SHOW MY LOCATION'}</strong></div><button class="voice-listen-button ${voiceListening ? 'is-listening' : ''}" type="button" aria-pressed="${voiceListening}" data-action="voice-listen">${voiceListening ? t.listenStop : t.listenStart}</button></aside>
    </div>`;
  bindInteractions();
  initMap();
}

function initMap() {
  const container = document.querySelector<HTMLElement>('#map-canvas');
  if (!container) return;
  map = new Map({
    container,
    center: [76.1, 11.55],
    zoom: 9.2,
    attributionControl: false,
    style: {
      version: 8,
      sources: {
        hazard: { type: 'geojson', data: { type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[76.00, 11.62], [76.18, 11.67], [76.24, 11.5], [76.05, 11.45], [76.00, 11.62]]] }, properties: {} } },
        route: { type: 'geojson', data: { type: 'Feature', geometry: { type: 'LineString', coordinates: [[75.98, 11.48], [76.08, 11.55], [76.19, 11.62]] }, properties: {} } },
        roads: { type: 'geojson', data: { type: 'FeatureCollection', features: [{ type: 'Feature', geometry: { type: 'LineString', coordinates: [[75.9, 11.45], [76.05, 11.53], [76.3, 11.57]] }, properties: {} }, { type: 'Feature', geometry: { type: 'LineString', coordinates: [[76.02, 11.72], [76.1, 11.55], [76.21, 11.42]] }, properties: {} }] } },
        river: { type: 'geojson', data: { type: 'Feature', geometry: { type: 'LineString', coordinates: [[75.9, 11.68], [76.02, 11.6], [76.15, 11.48], [76.3, 11.43]] }, properties: {} } },
        buildings: { type: 'geojson', data: { type: 'FeatureCollection', features: [{ type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[76.16, 11.58], [76.18, 11.58], [76.18, 11.6], [76.16, 11.6], [76.16, 11.58]]] }, properties: {} }, { type: 'Feature', geometry: { type: 'Polygon', coordinates: [[[76.2, 11.53], [76.22, 11.53], [76.22, 11.55], [76.2, 11.55], [76.2, 11.53]]] }, properties: {} }] } },
      },
      layers: [
        { id: 'background', type: 'background', paint: { 'background-color': '#101b24' } },
        { id: 'river', type: 'line', source: 'river', paint: { 'line-color': '#164e63', 'line-width': 5, 'line-opacity': 0.72 } },
        { id: 'roads', type: 'line', source: 'roads', paint: { 'line-color': '#64748b', 'line-width': 1.6, 'line-opacity': 0.46 } },
        { id: 'buildings', type: 'fill', source: 'buildings', paint: { 'fill-color': '#334155', 'fill-opacity': 0.58, 'fill-outline-color': '#64748b' } },
        { id: 'hazard-fill', type: 'fill', source: 'hazard', paint: { 'fill-color': '#ef4444', 'fill-opacity': 0.12 } },
        { id: 'hazard-edge', type: 'line', source: 'hazard', paint: { 'line-color': '#f87171', 'line-width': 2, 'line-opacity': 0.65 } },
        { id: 'approved-route', type: 'line', source: 'route', paint: { 'line-color': '#3b82f6', 'line-width': 3, 'line-opacity': 0.85 } },
      ],
    },
  });
}

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((button) => button.addEventListener('click', () => { language = button.dataset.language as Language; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="drawer"]')?.addEventListener('click', () => { drawerExpanded = !drawerExpanded; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice"]')?.addEventListener('click', () => { voiceOpen = true; render(); document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.focus(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => { voiceOpen = false; voiceListening = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => { voiceListening = !voiceListening; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => { routeStarted = true; arrivalOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="directions"]')?.addEventListener('click', () => { drawerExpanded = true; render(); showToast('Step-by-step directions are shown in the text-first drawer.'); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="details"]').forEach((button) => button.addEventListener('click', () => { detailsOpen = true; render(); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="details-close"]').forEach((button) => button.addEventListener('click', () => { detailsOpen = false; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', () => showToast('Audio is not connected in this demo. Text remains available.'));
  document.querySelector<HTMLButtonElement>('[data-action="isl"]')?.addEventListener('click', () => showToast('Approved ISL media is pending review.'));
  document.querySelectorAll<HTMLButtonElement>('[data-action="zoom-in"], [data-action="zoom-out"]').forEach((button) => button.addEventListener('click', () => showToast('Map camera control is ready for the approved geometry layer.')));
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => { arrivalOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-no"]')?.addEventListener('click', () => { arrivalOpen = false; render(); showToast('Help request preview only. No call was placed.'); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-yes"]')?.addEventListener('click', () => { arrivalOpen = false; render(); showToast('Arrival confirmation preview only. Capacity was not changed.'); });
}

function showToast(message: string) {
  const toast = document.querySelector<HTMLDivElement>('.toast');
  if (!toast) return;
  toast.textContent = message; toast.hidden = false; window.setTimeout(() => { toast.hidden = true; }, 3600);
}

render();
window.addEventListener('online', () => { isOffline = false; render(); });
window.addEventListener('offline', () => { isOffline = true; render(); });
