import './styles.css';

type Language = 'EN' | 'ML';

const translations = {
  EN: {
    alertLabel: 'ACTIVE ALERT',
    alertTitle: 'Heavy rainfall warning',
    alertBody: 'Move away from marked red zones and follow the approved route to your assigned safe zone.',
    viewAlert: 'View alert details',
    routeAction: 'View my route',
    locationAction: 'Set my place',
    mapTitle: 'Guidance map',
    mapNote: 'Synthetic demo map · official geometry will appear here',
    routeTitle: 'Your approved route',
    routeBody: 'No assignment yet. Set your place to see eligible guidance.',
    latest: 'Last refreshed 2 min ago',
    source: 'Source: SYNTHETIC_DEMO',
    voiceLabel: 'Voice Map Control',
    voiceBody: 'Speak a map command when the approved voice model is connected.',
    voiceOpen: 'Open voice guide',
    voiceStart: 'Start listening',
    voiceStatus: 'Model connection pending',
    voiceTranscript: 'Your transcript will appear here',
    voicePrivacy: 'Short audio is deleted after processing. No recording is active in this demo.',
    satelliteStatus: 'Satellite layer unavailable',
    satelliteNote: 'Government basemap authorization is pending. Synthetic guidance remains visible.',
  },
  ML: {
    alertLabel: 'സജീവ മുന്നറിയിപ്പ്',
    alertTitle: 'കനത്ത മഴ മുന്നറിയിപ്പ്',
    alertBody: 'ചുവപ്പ് മേഖലകളിൽ നിന്ന് മാറി, അനുവദിച്ച സുരക്ഷിത കേന്ദ്രത്തിലേക്കുള്ള മാർഗ്ഗം പിന്തുടരുക.',
    viewAlert: 'മുന്നറിയിപ്പ് കാണുക',
    routeAction: 'എന്റെ മാർഗ്ഗം കാണുക',
    locationAction: 'സ്ഥലം തിരഞ്ഞെടുക്കുക',
    mapTitle: 'മാർഗ്ഗനിർദ്ദേശ ഭൂപടം',
    mapNote: 'സിന്തറ്റിക് ഡെമോ ഭൂപടം · ഔദ്യോഗിക ജ്യാമിതി ഇവിടെ കാണിക്കും',
    routeTitle: 'അനുവദിച്ച മാർഗ്ഗം',
    routeBody: 'ഇതുവരെ നിയോഗമില്ല. മാർഗ്ഗനിർദ്ദേശം കാണാൻ സ്ഥലം തിരഞ്ഞെടുക്കുക.',
    latest: 'അവസാനം പുതുക്കിയത് 2 മിനിറ്റ് മുമ്പ്',
    source: 'ഉറവിടം: SYNTHETIC_DEMO',
    voiceLabel: 'വോയ്സ് മാപ്പ് നിയന്ത്രണം',
    voiceBody: 'അംഗീകൃത വോയ്സ് മോഡൽ ബന്ധിപ്പിച്ചാൽ മാപ്പ് കമാൻഡ് പറയാം.',
    voiceOpen: 'വോയ്സ് ഗൈഡ് തുറക്കുക',
    voiceStart: 'കേൾക്കാൻ തുടങ്ങുക',
    voiceStatus: 'മോഡൽ ബന്ധിപ്പിക്കൽ കാത്തിരിക്കുന്നു',
    voiceTranscript: 'നിങ്ങളുടെ ട്രാൻസ്‌ക്രിപ്റ്റ് ഇവിടെ കാണിക്കും',
    voicePrivacy: 'പ്രോസസ്സിംഗിന് ശേഷം ഹ്രസ്വ ഓഡിയോ ഇല്ലാതാക്കും. ഈ ഡെമോ റെക്കോർഡ് ചെയ്യുന്നില്ല.',
    satelliteStatus: 'സാറ്റലൈറ്റ് ലെയർ ലഭ്യമല്ല',
    satelliteNote: 'സർക്കാർ ഭൂപട അനുമതി കാത്തിരിക്കുന്നു. സിന്തറ്റിക് മാർഗ്ഗനിർദ്ദേശം കാണാം.',
  },
} as const;

let language: Language = 'EN';
let voiceOpen = false;
let voiceListening = false;

function render() {
  const copy = translations[language];
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell">
      <header class="topbar">
        <a class="wordmark" href="/" aria-label="Sthira home"><span class="wordmark-mark">S</span><span>Sthira</span></a>
        <div class="topbar-actions">
          <span class="demo-pill"><span class="status-dot"></span> DEMO MODE</span>
          <button class="voice-pet-button" type="button" aria-label="${copy.voiceOpen}" aria-expanded="${voiceOpen}" data-action="voice">
            <span class="voice-pet" aria-hidden="true"><i></i><i></i><b></b></span><span class="voice-pet-label">${copy.voiceLabel}</span>
          </button>
          <button class="language-toggle" type="button" aria-label="Switch language">${language === 'EN' ? 'മലയാളം' : 'English'}</button>
          <button class="icon-button" type="button" aria-label="Open menu">☰</button>
        </div>
      </header>

      <main>
        <section class="intro" aria-labelledby="page-title">
          <p class="eyebrow">Citizen emergency guidance</p>
          <h1 id="page-title">Stay informed.<br /><em>Move with clarity.</em></h1>
          <p class="intro-copy">Official instructions, approved routes, and safe-zone information in one calm place.</p>
        </section>

        <section class="alert-card" aria-labelledby="alert-title">
          <div class="alert-card-top"><span class="alert-badge"><span class="pulse-dot"></span>${copy.alertLabel}</span><span class="alert-meta">Wayanad · ${copy.latest}</span></div>
          <h2 id="alert-title">${copy.alertTitle}</h2>
          <p>${copy.alertBody}</p>
          <button class="text-link" type="button" data-action="alert">${copy.viewAlert} <span aria-hidden="true">↗</span></button>
        </section>

        <div class="section-heading"><div><p class="eyebrow">Next step</p><h2>Get your guidance</h2></div><span class="step-count">01 <span>/ 02</span></span></div>
        <section class="action-grid" aria-label="Guidance actions">
          <button class="action-card action-card-primary" type="button" data-action="location"><span class="action-icon" aria-hidden="true">⌖</span><span><strong>${copy.locationAction}</strong><small>Use a place name instead of precise location</small></span><span class="arrow" aria-hidden="true">→</span></button>
          <button class="action-card" type="button" data-action="route"><span class="action-icon action-icon-muted" aria-hidden="true">↗</span><span><strong>${copy.routeAction}</strong><small>Text directions always available</small></span><span class="arrow" aria-hidden="true">→</span></button>
        </section>

        <section class="workspace-grid">
          <article class="map-panel" aria-labelledby="map-title">
            <div class="panel-heading"><div><p class="eyebrow">Satellite-ready view</p><h2 id="map-title">${copy.mapTitle}</h2></div><span class="version-chip">v0.1 demo</span></div>
            <div class="map-stage" role="img" aria-label="Synthetic map preview showing a red zone, route, and safe zone">
              <div class="map-grid"></div><div class="zone zone-red"><span>RED ZONE</span></div><div class="route-line"></div><div class="safe-zone"><span>SAFE</span></div><div class="map-pin">S</div><div class="map-legend"><span><i class="legend-red"></i> Red zone</span><span><i class="legend-route"></i> Approved route</span><span><i class="legend-safe"></i> Safe zone</span></div>
            </div>
            <p class="panel-note"><span class="info-icon">i</span><strong>${copy.satelliteStatus}</strong> · ${copy.satelliteNote}</p>
          </article>
          <article class="route-panel" aria-labelledby="route-title"><div class="panel-heading"><div><p class="eyebrow">Text-first fallback</p><h2 id="route-title">${copy.routeTitle}</h2></div><span class="route-state">PENDING</span></div><p class="route-empty">${copy.routeBody}</p><ol class="route-steps"><li><span>1</span><div><strong>Choose a place</strong><small>Example: Meppadi bus stand</small></div></li><li><span>2</span><div><strong>Receive official guidance</strong><small>Only published routes and facilities</small></div></li><li><span>3</span><div><strong>Confirm when you arrive</strong><small>Touch confirmation, never automatic</small></div></li></ol><button class="outline-button" type="button" data-action="location">${copy.locationAction} <span aria-hidden="true">→</span></button></article>
        </section>
      </main>

      <footer class="footer"><span>Sthira v2 · Citizen guidance bridge</span><span>${copy.source} · No live government connection</span></footer>
      <div class="toast" role="status" aria-live="polite" hidden></div>
      <aside class="voice-panel ${voiceOpen ? 'is-open' : ''}" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}>
        <div class="voice-panel-head"><div><p class="eyebrow">${copy.voiceLabel}</p><h2 id="voice-title">Ask Sthira to move the map</h2></div><button class="voice-close" type="button" aria-label="Close voice guide" data-action="voice-close">×</button></div>
        <p class="voice-body">${copy.voiceBody}</p>
        <div class="voice-status"><span class="status-dot"></span><span>${copy.voiceStatus}</span></div>
        <div class="voice-transcript" aria-live="polite"><span>${copy.voiceTranscript}</span><strong>${voiceListening ? 'Listening preview...' : 'SHOW MY LOCATION'}</strong></div>
        <button class="voice-listen-button ${voiceListening ? 'is-listening' : ''}" type="button" aria-pressed="${voiceListening}" data-action="voice-listen"><span class="voice-mic" aria-hidden="true">${voiceListening ? '■' : '●'}</span>${voiceListening ? 'Stop listening' : copy.voiceStart}</button>
        <p class="voice-privacy">${copy.voicePrivacy}</p>
      </aside>
    </div>`;
  bindInteractions();
}

function bindInteractions() {
  document.querySelector<HTMLButtonElement>('.language-toggle')?.addEventListener('click', () => { language = language === 'EN' ? 'ML' : 'EN'; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice"]')?.addEventListener('click', () => { voiceOpen = true; render(); document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.focus(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => { voiceOpen = false; voiceListening = false; render(); document.querySelector<HTMLButtonElement>('[data-action="voice"]')?.focus(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => { voiceListening = !voiceListening; render(); });
  document.querySelectorAll<HTMLButtonElement>('[data-action="location"]').forEach((button) => button.addEventListener('click', () => showToast(language === 'EN' ? 'Place selection will connect to the official package here.' : 'ഔദ്യോഗിക പാക്കേജിലേക്കുള്ള സ്ഥലം തിരഞ്ഞെടുക്കൽ ഇവിടെ ലഭ്യമാകും.')));
  document.querySelectorAll<HTMLButtonElement>('[data-action="route"]').forEach((button) => button.addEventListener('click', () => showToast(language === 'EN' ? 'Set your place first to request guidance.' : 'മാർഗ്ഗനിർദ്ദേശം ലഭിക്കാൻ ആദ്യം സ്ഥലം തിരഞ്ഞെടുക്കുക.')));
  document.querySelector<HTMLButtonElement>('[data-action="alert"]')?.addEventListener('click', () => showToast(language === 'EN' ? 'Alert details will be sourced from the CAP feed.' : 'മുന്നറിയിപ്പ് വിവരങ്ങൾ CAP ഫീഡിൽ നിന്ന് ലഭിക്കും.'));
}

function showToast(message: string) {
  const toast = document.querySelector<HTMLDivElement>('.toast');
  if (!toast) return;
  toast.textContent = message;
  toast.hidden = false;
  window.setTimeout(() => { toast.hidden = true; }, 3600);
}

render();
