import './styles.css';
import 'maplibre-gl/dist/maplibre-gl.css';
import { Map, setWorkerUrl } from 'maplibre-gl';
import maplibreWorkerUrl from 'maplibre-gl/dist/maplibre-gl-worker.mjs?url';
import scenario from './scenario.json';
import { executeMapActions, type MapAction, type Panel } from './mapActions';
import '@fontsource/noto-sans/400.css';
import '@fontsource/noto-sans/600.css';
import '@fontsource/noto-sans/700.css';
import '@fontsource/noto-sans-malayalam/400.css';
import '@fontsource/noto-sans-malayalam/600.css';
import '@fontsource/noto-sans-malayalam/700.css';
import '@fontsource/noto-sans-devanagari/400.css';
import '@fontsource/noto-sans-devanagari/600.css';
import '@fontsource/noto-sans-devanagari/700.css';

setWorkerUrl(maplibreWorkerUrl);

type Language = 'EN' | 'ML' | 'HI';

const copy = {
  EN: {
    lang: 'English', demo: 'SYNTHETIC DEMO · NO LIVE FEED', alert: 'SYNTHETIC EXERCISE · FLOOD', hazard: 'FLOOD EXERCISE', live: 'Synthetic · 12 Sep 2026',
    headline: 'Evacuate to Government School, Ward 8', summary: 'Severe flooding affects Ward 12 in this exercise. Use the stored demo route.',
    steps: ['Follow the safe route shown below.', 'Avoid low-lying roads and floodwater.', 'Take medicines & ID.'], shelter: 'Government School, Ward 8',
    open: 'OPEN · DEMO CAPACITY DECLARED', deadline: 'Deadline', assigned: 'Assigned shelter', routeMetric: 'Distance / steps', deadlineValue: 'Leave before 6:00 PM', assignedValue: 'Ward 8 Govt School', routeValue: 'Stored demo route',
    start: 'START SAFE ROUTE', call: 'CALL 112', listen: 'Listen', isl: 'ISL Video', directions: 'Step-by-step directions', details: 'View alert details',
    source: 'SYNTHETIC_DEMO · local scenario package · 12 Sep 2026', mapNote: 'SYNTHETIC DEMO · OpenFreeMap visual basemap only', mapAria: 'Synthetic map showing red zone, citizen position, stored route, and safe zones',
    voice: 'Voice Control', voiceOpen: 'Open Voice Map Control', voiceTitle: 'Voice-to-text control', voiceBody: 'Approved short commands will move the map when IndicConformer is connected.', voicePending: 'Model connection pending', listenStart: 'START LISTENING', listenStop: 'STOP LISTENING', transcript: 'Transcript appears here',
    close: 'Close', drawer: 'Expand guidance', collapse: 'Collapse guidance', arrivalTitle: 'Have you arrived at the synthetic assigned zone?', arrivalBody: 'This demo does not change capacity or confirm arrival.', yes: 'YES, I HAVE ARRIVED', no: 'NO, I NEED HELP',
    offline: 'OFFLINE · LAST VALID SYNTHETIC DATA', online: 'SYNTHETIC DEMO · NO LIVE FEED', detailsTitle: 'Alert details', detailsBody: 'Synthetic data only. This is not an active government alert.', issued: 'Scenario date 12 Sep 2026 · issued 4:00 PM', expires: 'Synthetic exercise expires 12 Sep 2026 · 6:00 PM', detailsClose: 'Close alert details',
  },
  ML: {
    lang: 'മലയാളം', demo: 'സിന്തറ്റിക് ഡെമോ · ലൈവ് ഫീഡ് ഇല്ല', alert: 'സിന്തറ്റിക് പരിശീലനം · വെള്ളപ്പൊക്കം', hazard: 'പരിശീലന വെള്ളപ്പൊക്കം', live: 'സിന്തറ്റിക് · 12 സെപ്റ്റംബർ 2026',
    headline: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8-ലേക്ക് മാറുക', summary: 'വാർഡ് 12-ലെ ഈ പരിശീലനത്തിൽ ഗുരുതര വെള്ളപ്പൊക്കം. സംഭരിച്ച ഡെമോ മാർഗ്ഗം ഉപയോഗിക്കുക.',
    steps: ['താഴെ കാണുന്ന സുരക്ഷിത മാർഗ്ഗം പിന്തുടരുക.', 'താഴ്ന്ന റോഡുകളും വെള്ളവും ഒഴിവാക്കുക.', 'മരുന്നുകളും തിരിച്ചറിയൽ രേഖകളും എടുക്കുക.'], shelter: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8',
    open: 'തുറന്നിരിക്കുന്നു · ഡെമോ ശേഷി', deadline: 'അവസാന സമയം', assigned: 'നിയോഗിച്ച കേന്ദ്രം', routeMetric: 'ദൂരം / ഘട്ടങ്ങൾ', deadlineValue: 'പുറപ്പെടുക: വൈകിട്ട് 6:00', assignedValue: 'വാർഡ് 8 സർക്കാർ സ്കൂൾ', routeValue: 'സംഭരിച്ച ഡെമോ മാർഗ്ഗം',
    start: 'സുരക്ഷിത മാർഗ്ഗം തുടങ്ങുക', call: '112 വിളിക്കുക', listen: 'കേൾക്കുക', isl: 'ISL വീഡിയോ', directions: 'ഘട്ടംഘട്ടമായ നിർദ്ദേശങ്ങൾ', details: 'മുന്നറിയിപ്പ് വിവരങ്ങൾ',
    source: 'SYNTHETIC_DEMO · പ്രാദേശിക സീനാരിയോ പാക്കേജ്', mapNote: 'സിന്തറ്റിക് മാപ്പ് · OpenFreeMap ദൃശ്യ ബേസ്മാപ്പ് മാത്രം', mapAria: 'സിന്തറ്റിക് ചുവപ്പ് മേഖല, പൗരന്റെ സ്ഥാനം, സംഭരിച്ച മാർഗ്ഗം, സുരക്ഷിത മേഖലകൾ കാണിക്കുന്ന മാപ്പ്',
    voice: 'വോയ്സ് നിയന്ത്രണം', voiceOpen: 'വോയ്സ് മാപ്പ് നിയന്ത്രണം തുറക്കുക', voiceTitle: 'വോയ്സ്-ടു-ടെക്സ്റ്റ് നിയന്ത്രണം', voiceBody: 'IndicConformer ബന്ധിപ്പിച്ചാൽ അംഗീകൃത ഹ്രസ്വ കമാൻഡുകൾ മാപ്പ് നീക്കും.', voicePending: 'മോഡൽ കണക്ഷൻ കാത്തിരിക്കുന്നു', listenStart: 'കേൾക്കാൻ തുടങ്ങുക', listenStop: 'കേൾക്കുന്നത് നിർത്തുക', transcript: 'ട്രാൻസ്‌ക്രിപ്റ്റ് ഇവിടെ കാണിക്കും',
    close: 'അടയ്ക്കുക', drawer: 'മാർഗ്ഗനിർദ്ദേശം വികസിപ്പിക്കുക', collapse: 'മാർഗ്ഗനിർദ്ദേശം ചുരുക്കുക', arrivalTitle: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8-ൽ എത്തിയോ?', arrivalBody: 'ഈ പ്രിവ്യൂ ശേഷി മാറ്റില്ല. എത്തിച്ചേരൽ സ്ഥിരീകരണം ഘട്ടം 6-ൽ ബന്ധിപ്പിക്കും.', yes: 'അതെ, എത്തി', no: 'ഇല്ല, സഹായം വേണം',
    offline: 'ഓഫ്‌ലൈൻ · അവസാനത്തെ സാധുവായ ഡെമോ ഡാറ്റ', online: 'ഡെമോ ഫീഡ് സജീവം', detailsTitle: 'മുന്നറിയിപ്പ് വിവരങ്ങൾ', detailsBody: 'സിന്തറ്റിക് ഡാറ്റ മാത്രം. ഇത് സജീവ സർക്കാർ മുന്നറിയിപ്പല്ല.', issued: 'നൽകിയത് 11 സെപ്റ്റംബർ 2026 · വൈകിട്ട് 4:00', expires: 'കാലാവധി 11 സെപ്റ്റംബർ 2026 · വൈകിട്ട് 6:00', detailsClose: 'മുന്നറിയിപ്പ് അടയ്ക്കുക',
  },
  HI: {
    lang: 'हिन्दी', demo: 'सिंथेटिक डेमो · लाइव फीड नहीं', alert: 'सिंथेटिक अभ्यास · बाढ़', hazard: 'अभ्यास बाढ़', live: 'सिंथेटिक · 12 सितम्बर 2026',
    headline: 'सरकारी स्कूल, वार्ड 8 में जाएं', summary: 'इस अभ्यास में वार्ड 12 में गंभीर बाढ़ है। संग्रहीत डेमो मार्ग का उपयोग करें.',
    steps: ['नीचे दिखाए सुरक्षित मार्ग का पालन करें।', 'निचली सड़कों और बाढ़ के पानी से बचें।', 'दवाइयां और पहचान पत्र साथ लें।'], shelter: 'सरकारी स्कूल, वार्ड 8',
    open: 'खुला · डेमो क्षमता', deadline: 'समय सीमा', assigned: 'निर्धारित आश्रय', routeMetric: 'दूरी / चरण', deadlineValue: 'पहले निकलें: शाम 6:00', assignedValue: 'वार्ड 8 सरकारी स्कूल', routeValue: 'संग्रहीत डेमो मार्ग',
    start: 'सुरक्षित मार्ग शुरू करें', call: '112 पर कॉल करें', listen: 'सुनें', isl: 'ISL वीडियो', directions: 'चरण-दर-चरण निर्देश', details: 'अलर्ट विवरण',
    source: 'SYNTHETIC_DEMO · स्थानीय परिदृश्य पैकेज', mapNote: 'सिंथेटिक मानचित्र · OpenFreeMap दृश्य बेसमैप मात्र', mapAria: 'सिंथेटिक लाल क्षेत्र, नागरिक स्थान, संग्रहीत मार्ग और सुरक्षित क्षेत्र दिखाने वाला मानचित्र',
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
let partySize = 1;
let arrivalSuccess = false;
let assistanceOpen = false;
let directionsOpen = false;
let listenOpen = false;
let islOpen = false;
let layersOpen = false;
let showHazard = true;
let showRoute = true;
let showShelters = true;
let pitched = false;
let callOpen = false;
let assignmentState: 'UNASSIGNED' | 'RESERVED' | 'ARRIVED' = 'UNASSIGNED';
let scenarioState: 'LOADING' | 'READY' | 'NO_ACTIVE' | 'ERROR' = 'LOADING';
const API_BASE = window.location.port === '5173' ? 'http://127.0.0.1:8000' : '';

async function loadScenario(): Promise<void> {
  scenarioState = 'LOADING';
  render();
  try {
    const response = await fetch(`${API_BASE}/api/v2/demo/scenario`, { headers: { Accept: 'application/json' } });
    if (!response.ok) throw new Error(`scenario request failed: ${response.status}`);
    const body = await response.json() as { data?: unknown; source_status?: unknown };
    const data = body.data as Record<string, unknown> | undefined;
    if (body.source_status !== 'SYNTHETIC_DEMO' || !data || data.evidence_class !== 'SYNTHETIC_DEMO' || !data.disclaimer || typeof data.expires_at !== 'string' || !data.alert || !Array.isArray(data.red_zones) || !Array.isArray(data.safe_zones) || !Array.isArray(data.routes)) throw new Error('scenario provenance or shape invalid');
    Object.assign(scenario as object, data);
    localStorage.setItem('sthira-v2-last-valid-scenario', JSON.stringify({ saved_at: new Date().toISOString(), data }));
    scenarioState = (data.alert as { active?: boolean }).active === false ? 'NO_ACTIVE' : 'READY';
  } catch {
    const cached = !navigator.onLine ? localStorage.getItem('sthira-v2-last-valid-scenario') : null;
    if (cached) {
      try {
        const parsed = JSON.parse(cached) as { data?: Record<string, unknown> };
        if (parsed.data?.evidence_class === 'SYNTHETIC_DEMO' && typeof parsed.data.expires_at === 'string' && Date.parse(parsed.data.expires_at) > Date.now()) {
          Object.assign(scenario as object, parsed.data);
          isOffline = true;
          scenarioState = (parsed.data.alert as { active?: boolean } | undefined)?.active === false ? 'NO_ACTIVE' : 'READY';
          render();
          return;
        }
      } catch {
        localStorage.removeItem('sthira-v2-last-valid-scenario');
      }
    }
    scenarioState = 'ERROR';
  }
  render();
}

async function reserveDemoAssignment(): Promise<void> {
  try {
    const response = await fetch(`${API_BASE}/api/v2/assignments`, {
      method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ assignment_id: 'ui-demo-assignment', alert_id: 'DEMO-WYD-LANDSLIDE-001', citizen_session_id: 'synthetic-ui-session-001', idempotency_key: 'ui-demo-assignment-key', party_size: partySize }),
    });
    if (!response.ok) throw new Error('assignment unavailable');
    const body = await response.json() as { data?: { state?: string } };
    if (body.data?.state !== 'RESERVED' && body.data?.state !== 'ARRIVED') throw new Error('assignment state invalid');
    assignmentState = body.data.state as 'RESERVED' | 'ARRIVED';
  } catch {
    assignmentState = 'UNASSIGNED';
    showToast('Synthetic assignment unavailable; no capacity claim was made.');
  }
}

async function confirmDemoArrival(responseValue: 'YES' | 'NO'): Promise<void> {
  try {
    const response = await fetch(`${API_BASE}/api/v2/assignments/ui-demo-assignment/arrival-confirmations`, {
      method: 'POST', headers: { 'Content-Type': 'application/json', Accept: 'application/json' },
      body: JSON.stringify({ response: responseValue, idempotency_key: `ui-demo-arrival-${responseValue.toLowerCase()}` }),
    });
    if (!response.ok) throw new Error('arrival unavailable');
    const body = await response.json() as { data?: { state?: string } };
    if (responseValue === 'YES' && body.data?.state === 'ARRIVED') assignmentState = 'ARRIVED';
    showToast(responseValue === 'YES' ? 'Arrival recorded in the synthetic ledger.' : 'No arrival or capacity change was recorded.');
  } catch {
    showToast('Arrival service unavailable; no capacity claim was made.');
  }
}

function render() {
  if (scenarioState !== 'READY') {
    const message = scenarioState === 'LOADING' ? 'Verifying the versioned synthetic scenario package.' : scenarioState === 'NO_ACTIVE' ? 'There is no active synthetic alert. No route, safe zone, or capacity guidance is available.' : 'The scenario package could not be verified. No alert, route, safe zone, or capacity guidance is being shown.';
    document.querySelector<HTMLDivElement>('#app')!.innerHTML = `<main class="scenario-gate" role="status" aria-live="polite"><p class="eyebrow">Sthira v2 · SYNTHETIC_DEMO</p><h1>${scenarioState === 'LOADING' ? 'Loading demo guidance…' : scenarioState === 'NO_ACTIVE' ? 'No active alert' : 'Demo guidance unavailable'}</h1><p>${message}</p>${scenarioState === 'ERROR' ? '<button type="button" data-action="retry-scenario">Retry scenario load</button>' : ''}</main>`;
    document.querySelector<HTMLButtonElement>('[data-action="retry-scenario"]')?.addEventListener('click', () => void loadScenario());
    return;
  }
  const t = copy[language];
  map?.remove();
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell tactical-shell ${drawerExpanded ? 'drawer-expanded' : 'drawer-collapsed'}">
      <div class="tactical-map" role="img" aria-label="${t.mapAria}"><div id="map-canvas" aria-hidden="true"></div><div class="map-controls" aria-label="Map controls"><button type="button" aria-label="Zoom in" data-action="zoom-in">+</button><button type="button" aria-label="Zoom out" data-action="zoom-out">−</button></div><div class="map-action-pill"><button type="button" data-action="recenter">Recenter</button><button type="button" data-action="directions">Turn-by-turn list</button><button type="button" data-action="voice">Voice control</button></div>
      </div>
      <header class="tactical-topbar">
        <a class="wordmark" href="/" aria-label="Sthira home"><span class="wordmark-mark">S</span><span>Sthira <small>v2</small></span></a>
        <div class="topbar-center"><span class="heartbeat"><i></i>${isOffline ? t.offline : t.demo}</span></div>
        <div class="topbar-actions"><button class="voice-control ${voiceListening ? 'is-listening' : ''}" type="button" aria-label="${t.voiceOpen}" aria-expanded="${voiceOpen}" data-action="voice"><span class="mic-icon">●</span><span>${t.voice}</span></button><div class="language-switcher" role="group" aria-label="Language"><button class="language-choice ${language === 'EN' ? 'is-selected' : ''}" data-language="EN" type="button">EN</button><button class="language-choice ${language === 'ML' ? 'is-selected' : ''}" data-language="ML" type="button">ML</button><button class="language-choice ${language === 'HI' ? 'is-selected' : ''}" data-language="HI" type="button">HI</button></div></div>
      </header>
      <main class="tactical-main">
        <section class="floating-drawer civic-drawer" data-testid="emergency-card" aria-labelledby="alert-title">
          <button class="drawer-handle" type="button" aria-expanded="${drawerExpanded}" aria-label="${drawerExpanded ? t.collapse : t.drawer}" data-action="drawer"><span></span></button>
          <div class="drawer-scroll">
            <div class="official-strip">SYNTHETIC EXERCISE · NOT OFFICIAL GUIDANCE <span>${t.live} · SYNTHETIC_DEMO</span></div>
            <div class="urgent-header"><span class="critical-badge"><i></i>${t.alert}</span><button class="detail-trigger" type="button" data-action="details" aria-label="${t.details}">ⓘ</button><h1 id="alert-title">${t.headline}</h1><p class="summary">${t.summary}</p></div>
            <article class="civic-destination"><div><p class="overline">WHERE TO GO · SYNTHETIC ASSIGNMENT</p><h2>Government Higher Secondary School, Ward 8</h2><span class="open-badge">✓ OPEN · DEMO CAPACITY DECLARED</span></div><strong>1.4 km<br /><small>~18 min walk · demo value</small></strong></article>
            <button class="start-button civic-start ${routeStarted ? 'is-started' : ''}" type="button" data-testid="start-route" data-action="route"><span>${routeStarted ? '✓' : '➜'}</span>${routeStarted ? 'ROUTE ACTIVE' : 'START STEP-BY-STEP EVACUATION ROUTE'}</button>
            <ul class="civic-directives"><li>Bridges on Main Canal Rd are <strong>SUBMERGED</strong> — use Ridge Road only.</li><li>Move on foot or light vehicle; avoid low-lying culverts.</li><li>Take essential medications and identity documents.</li></ul>
            <div class="utility-row"><button type="button" data-action="listen">🔊 ${t.listen}</button><button type="button" data-action="isl">✋ ${t.isl}</button><button type="button" data-action="directions">☷ ${t.directions}</button></div>
            <button class="arrival-trigger" data-testid="arrival-confirmation" type="button" data-action="arrival-open">I HAVE REACHED THE SHELTER</button>
            <a class="rescue-link" data-testid="call-112" href="tel:112">Trapped or cut off by water? <strong>Call 112 for Rescue</strong></a>
          </div>
        </section>
      </main>
      <div class="map-legend-floating context-widget"><span><i class="legend-crimson"></i> Synthetic hazard area · Ward 12</span><span><i class="legend-cobalt"></i> Stored demo route</span><span><i class="legend-mint"></i> Synthetic assignment</span></div>
      <footer class="tactical-footer"><span>${t.mapNote}</span><button type="button" data-action="details">${t.details}</button></footer>
      <div class="toast" role="status" aria-live="polite" hidden></div>
      <dialog class="tactical-dialog" aria-labelledby="details-title" ${detailsOpen ? 'open' : ''}><div class="dialog-top"><h2 id="details-title">${t.detailsTitle}</h2><button type="button" aria-label="${t.detailsClose}" data-action="details-close">×</button></div><p>${t.detailsBody}</p><dl><div><dt>${t.issued}</dt><dd>${t.source}</dd></div><div><dt>${t.expires}</dt></div></dl><button class="start-button" type="button" data-action="details-close">${t.detailsClose}</button></dialog>
      <aside class="voice-panel ${voiceOpen ? 'is-open' : ''}" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}><div class="dialog-top"><div><p class="overline">${t.voice}</p><h2 id="voice-title">${t.voiceTitle}</h2></div><button type="button" aria-label="${t.close}" data-action="voice-close">×</button></div><p>${t.voiceBody}</p><div class="voice-wave" aria-hidden="true"><i></i><i></i><i></i><i></i><i></i><i></i><i></i></div><div class="voice-pending">● ${t.voicePending}</div><div class="voice-transcript" aria-live="polite"><span>${t.transcript}</span><strong>${voiceListening ? 'Listening preview...' : 'SHOW MY LOCATION'}</strong></div><div class="voice-chips"><button type="button" data-action="voice-safe">🎯 Focus Safe Zone</button><button type="button" data-action="voice-alert">⚠ Show Alert Area</button><button type="button" data-action="voice-route">🗺 Full Route</button><button type="button" data-action="voice-recenter">🔄 Recenter</button><button type="button" data-action="voice-repeat">🔊 Repeat Instruction</button></div><button class="voice-listen-button ${voiceListening ? 'is-listening' : ''}" type="button" aria-pressed="${voiceListening}" data-action="voice-listen">${voiceListening ? t.listenStop : t.listenStart}</button></aside>
      <dialog class="arrival-dialog capability-dialog" aria-labelledby="arrival-title" ${arrivalOpen ? 'open' : ''}><div class="dialog-top"><div><span class="arrival-icon">?</span><h2 id="arrival-title">Have you and your party safely arrived?</h2></div><button type="button" aria-label="${t.close}" data-action="arrival-close">×</button></div><p>Synthetic assignment · ${assignmentState === 'RESERVED' ? 'reservation recorded' : assignmentState === 'ARRIVED' ? 'arrival recorded' : 'reservation pending'}</p>${arrivalSuccess ? '<div class="success-state">✓ Arrival recorded in this demo. No live capacity was changed.</div>' : `<div class="party-stepper"><button type="button" data-action="party-minus" aria-label="Decrease party size">−</button><strong>${partySize} ${partySize === 1 ? 'Person' : 'People'}</strong><button type="button" data-action="party-plus" aria-label="Increase party size">+</button></div><div class="arrival-actions"><button class="arrived-button" type="button" data-action="arrival-yes">YES, WE HAVE ARRIVED</button><button class="help-button" type="button" data-action="arrival-no">NO, NEED ASSISTANCE</button></div>`}</dialog>
      <dialog class="arrival-dialog capability-dialog" aria-labelledby="assist-title" ${assistanceOpen ? 'open' : ''}><div class="dialog-top"><h2 id="assist-title">Need assistance?</h2><button type="button" aria-label="${t.close}" data-action="assist-close">×</button></div><p>No reservation or capacity event will be created. Use the device dialler for emergency help.</p><a class="call-button" href="tel:112">☎ CALL 112</a></dialog>
      <dialog class="arrival-dialog capability-dialog" aria-labelledby="call-title" ${callOpen ? 'open' : ''}><div class="dialog-top"><h2 id="call-title">Call emergency services?</h2><button type="button" aria-label="${t.close}" data-action="call-close">×</button></div><p>Sthira will open your device dialler for 112. It will not place a silent call or claim dispatch.</p><div class="arrival-actions"><a class="call-button" data-action="call-confirm" href="tel:112">☎ CALL 112</a><button class="help-button" type="button" data-action="call-close">CANCEL</button></div></dialog>
      <aside class="directions-sheet ${directionsOpen ? 'is-open' : ''}" ${directionsOpen ? '' : 'hidden'}><div class="dialog-top"><div><p class="overline">STORED DEMO ROUTE · 1.4 km · DEMO VALUE</p><h2>Sector 12 to Ward 8 School</h2></div><button type="button" aria-label="${t.close}" data-action="directions-close">×</button></div><ol class="landmark-list"><li><b>1</b><span>Depart Ward 12 Community Hall toward High Ridge Rd.<small>Avoid low culvert.</small></span></li><li><b>2</b><span>Turn right at Post Office Junction.<small>Elevated paved stretch.</small></span></li><li><b>3</b><span>Arrive at Government Higher Secondary School, Ward 8.<small>Use North Gate.</small></span></li></ol><div class="sheet-actions"><button type="button" data-action="offline-card">📥 Save Offline Route Card</button><button type="button" data-action="accessibility">♿ Accessibility Notes</button></div></aside>
      <aside class="assist-sheet ${listenOpen ? 'is-open' : ''}" ${listenOpen ? '' : 'hidden'}><div class="dialog-top"><h2>Listen · TTS preview</h2><button type="button" aria-label="${t.close}" data-action="listen-close">×</button></div><p>Approved audio artifact pending. Text transcript remains available.</p><div class="audio-player"><button type="button" aria-label="Audio unavailable" disabled data-action="audio-toggle">▶</button><span>Step 1 of 3 · Follow the stored demo route</span><button type="button" aria-label="Audio unavailable" disabled data-action="audio-repeat">↻</button></div></aside>
      <aside class="assist-sheet isl-sheet ${islOpen ? 'is-open' : ''}" ${islOpen ? '' : 'hidden'}><div class="dialog-top"><h2>ISL guidance preview</h2><button type="button" aria-label="${t.close}" data-action="isl-close">×</button></div><div class="isl-video">ISL MEDIA PENDING APPROVAL<br /><small>Approved media and Deaf/ISL review are required.</small></div><p>Caption: Text instructions remain available as the accessible fallback.</p></aside>
      ${layersOpen ? '<aside class="layers-panel"><strong>Map layers</strong><label><input type="checkbox" data-layer="hazard" checked /> Synthetic Red Zone</label><label><input type="checkbox" data-layer="route" checked /> Stored Demo Route</label><label><input type="checkbox" data-layer="shelters" checked /> Synthetic Safe Zones</label></aside>' : ''}
    </div>`;
  bindInteractions();
  initMap();
}

function initMap() {
  const container = document.querySelector<HTMLElement>('#map-canvas');
  if (!container) return;
  map = new Map({
    container,
    center: [78.9629, 20.5937],
    zoom: 3.5,
    bearing: 0,
    pitch: 0,
    attributionControl: { compact: false },
    style: 'https://tiles.openfreemap.org/styles/liberty',
  });
  map.once('load', () => {
    if (!map) return;
    map.addSource('red-zones', { type: 'geojson', data: { type: 'FeatureCollection', features: scenario.red_zones.map((item) => ({ type: 'Feature', id: item.id, properties: { id: item.id }, geometry: item.geometry })) } as never });
    map.addSource('safe-zones', { type: 'geojson', data: { type: 'FeatureCollection', features: scenario.safe_zones.map((item) => ({ type: 'Feature', id: item.id, properties: { id: item.id, assigned: item.assigned }, geometry: item.geometry })) } as never });
    map.addSource('routes', { type: 'geojson', data: { type: 'FeatureCollection', features: scenario.routes.map((item) => ({ type: 'Feature', id: item.id, properties: { id: item.id }, geometry: item.geometry })) } as never });
    map.addSource('my-location', { type: 'geojson', data: { type: 'Feature', id: scenario.citizen_location.id, properties: {}, geometry: scenario.citizen_location.geometry } as never });
    map.addLayer({ id: 'red-zones-fill', type: 'fill', source: 'red-zones', paint: { 'fill-color': '#dc2626', 'fill-opacity': 0.32 } });
    map.addLayer({ id: 'red-zones-border', type: 'line', source: 'red-zones', paint: { 'line-color': '#fecaca', 'line-width': 3 } });
    map.addLayer({ id: 'safe-zones', type: 'circle', source: 'safe-zones', paint: { 'circle-color': ['case', ['get', 'assigned'], '#34d399', '#93c5fd'], 'circle-radius': 9, 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 2 } });
    map.addLayer({ id: 'routes', type: 'line', source: 'routes', paint: { 'line-color': '#fbbf24', 'line-width': 5, 'line-opacity': 0.95 } });
    map.addLayer({ id: 'my-location', type: 'circle', source: 'my-location', paint: { 'circle-color': '#2563eb', 'circle-radius': 8, 'circle-stroke-color': '#ffffff', 'circle-stroke-width': 3 } });
    map.resize();
  });
  map.on('error', () => showToast('Basemap unavailable. Synthetic overlays and text guidance remain available.'));
  const observer = new ResizeObserver(() => map?.resize());
  observer.observe(container);
}

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((button) => button.addEventListener('click', () => { language = button.dataset.language as Language; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="drawer"]')?.addEventListener('click', () => { drawerExpanded = !drawerExpanded; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="voice"]').forEach((button) => button.addEventListener('click', () => { voiceOpen = true; render(); document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.focus(); }));
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => { voiceOpen = false; voiceListening = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => { voiceListening = !voiceListening; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => { routeStarted = true; void reserveDemoAssignment(); render(); window.setTimeout(() => { arrivalOpen = true; render(); }, 700); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-open"]')?.addEventListener('click', () => { arrivalSuccess = false; arrivalOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="party-minus"]')?.addEventListener('click', () => { partySize = Math.max(1, partySize - 1); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="party-plus"]')?.addEventListener('click', () => { partySize = Math.min(10, partySize + 1); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => { arrivalOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="assist-close"]')?.addEventListener('click', () => { assistanceOpen = false; render(); });
  document.querySelectorAll<HTMLAnchorElement>('a[href="tel:112"]:not([data-action="call-confirm"])').forEach((button) => button.addEventListener('click', (event) => { event.preventDefault(); callOpen = true; render(); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="call-close"]').forEach((button) => button.addEventListener('click', () => { callOpen = false; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="directions"]')?.addEventListener('click', () => { drawerExpanded = true; render(); showToast('Step-by-step directions are shown in the text-first drawer.'); });
  document.querySelector<HTMLButtonElement>('[data-action="directions"]')?.addEventListener('click', () => { directionsOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="directions-close"]')?.addEventListener('click', () => { directionsOpen = false; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="details"]').forEach((button) => button.addEventListener('click', () => { detailsOpen = true; render(); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="details-close"]').forEach((button) => button.addEventListener('click', () => { detailsOpen = false; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', () => { listenOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="listen-close"]')?.addEventListener('click', () => { listenOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="isl"]')?.addEventListener('click', () => { islOpen = true; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="isl-close"]')?.addEventListener('click', () => { islOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="offline-card"]')?.addEventListener('click', () => showToast('Offline route card preview saved locally in this demo.'));
  document.querySelector<HTMLButtonElement>('[data-action="accessibility"]')?.addEventListener('click', () => showToast('North Gate has a wheelchair ramp in this synthetic package.'));
  const reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const runMap = (actions: MapAction[]) => {
    showToast('Interpreting request…');
    window.setTimeout(() => {
      const ok = map ? executeMapActions(map, { schema_version: '1.0', status: 'OK', actions }, reducedMotion, (panel: Panel) => { if (panel === 'EMERGENCY_CALL_CONFIRMATION') { callOpen = true; render(); } else showToast(`${panel.replaceAll('_', ' ')} opened.`); }) : false;
      showToast(ok ? 'Map updated from the synthetic scenario.' : 'Request rejected; no map action was taken.');
    }, reducedMotion ? 0 : 120);
  };
  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => runMap([{ type: 'SET_LAYER_VISIBILITY', layer: 'ROUTES', visible: true }, { type: 'FIT_FEATURES', target_ids: ['SZ-DEMO-01', 'ROUTE-DEMO-01'] }, { type: 'OPEN_PANEL', panel: 'ROUTE_GUIDANCE', target_id: 'ROUTE-DEMO-01' }]));
  document.querySelector<HTMLButtonElement>('[data-action="recenter"]')?.addEventListener('click', () => runMap([{ type: 'RECENTER', view_id: 'DEMO_OVERVIEW' }]));
  document.querySelector<HTMLButtonElement>('[data-action="pitch"]')?.addEventListener('click', () => { pitched = !pitched; map?.easeTo({ pitch: pitched ? 45 : 0, duration: 600 }); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="layers"]')?.addEventListener('click', () => { layersOpen = !layersOpen; render(); });
  document.querySelectorAll<HTMLInputElement>('[data-layer]').forEach((input) => input.addEventListener('change', () => { const layer = input.dataset.layer; if (layer === 'hazard') showHazard = input.checked; if (layer === 'route') showRoute = input.checked; if (layer === 'shelters') showShelters = input.checked; const mapLayer = layer === 'hazard' ? 'red-zones-fill' : layer === 'route' ? 'routes' : 'safe-zones'; if (map?.getLayer(mapLayer)) map.setLayoutProperty(mapLayer, 'visibility', input.checked ? 'visible' : 'none'); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action^="voice-"]').forEach((button) => button.addEventListener('click', () => { const action = button.dataset.action; if (action === 'voice-safe') runMap([{ type: 'SET_LAYER_VISIBILITY', layer: 'SAFE_ZONES', visible: true }, { type: 'FOCUS_FEATURE', target_id: 'SZ-DEMO-01' }, { type: 'OPEN_PANEL', panel: 'SAFE_ZONE_DETAILS', target_id: 'SZ-DEMO-01' }]); if (action === 'voice-alert') runMap([{ type: 'SET_LAYER_VISIBILITY', layer: 'RED_ZONES', visible: true }, { type: 'FOCUS_FEATURE', target_id: 'RZ-DEMO-01' }, { type: 'OPEN_PANEL', panel: 'ALERT_DETAILS', target_id: 'RZ-DEMO-01' }]); if (action === 'voice-route') runMap([{ type: 'SET_LAYER_VISIBILITY', layer: 'ROUTES', visible: true }, { type: 'FIT_FEATURES', target_ids: ['SZ-DEMO-01', 'ROUTE-DEMO-01'] }, { type: 'OPEN_PANEL', panel: 'ROUTE_GUIDANCE', target_id: 'ROUTE-DEMO-01' }]); if (action === 'voice-recenter') runMap([{ type: 'RECENTER', view_id: 'DEMO_OVERVIEW' }]); if (action === 'voice-repeat') showToast(scenario.instruction[language === 'ML' ? 'ML' : 'EN']); }));
  document.querySelectorAll<HTMLButtonElement>('[data-action="zoom-in"], [data-action="zoom-out"]').forEach((button) => button.addEventListener('click', () => runMap([{ type: 'ZOOM', direction: button.dataset.action === 'zoom-in' ? 'IN' : 'OUT', steps: 1 }])));
  document.querySelector<HTMLButtonElement>('[data-action="arrival-close"]')?.addEventListener('click', () => { arrivalOpen = false; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-no"]')?.addEventListener('click', () => { arrivalOpen = false; void confirmDemoArrival('NO'); render(); });
  document.querySelector<HTMLButtonElement>('[data-action="arrival-yes"]')?.addEventListener('click', () => { arrivalSuccess = true; arrivalOpen = false; void confirmDemoArrival('YES'); render(); });
}

function showToast(message: string) {
  const toast = document.querySelector<HTMLDivElement>('.toast');
  if (!toast) return;
  toast.textContent = message; toast.hidden = false; window.setTimeout(() => { toast.hidden = true; }, 3600);
}

render();
window.addEventListener('online', () => { isOffline = false; render(); });
window.addEventListener('offline', () => { isOffline = true; render(); });
void loadScenario();
if ('serviceWorker' in navigator) {
  const serviceWorkerPath = window.location.pathname.startsWith('/v2') ? '/v2/sw.js' : '/sw.js';
  void navigator.serviceWorker.register(serviceWorkerPath, { scope: window.location.pathname.startsWith('/v2') ? '/v2/' : '/' });
}
