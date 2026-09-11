import './styles.css';
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

const translations = {
  EN: {
    language: 'English', alert: 'RED', action: 'ACT NOW', hazard: 'FLOOD',
    instruction: 'Move to your assigned safe shelter now.', detail: 'Severe flooding affects Ward 12.',
    steps: ['Follow the safe route shown below.', 'Do not walk or drive through floodwater.', 'Take children, medicines and ID.'],
    shelter: 'Government School, Ward 8', leave: 'Leave before: 6:00 PM', source: 'District Disaster Authority · SYNTHETIC_DEMO',
    route: 'START SAFE ROUTE', call: 'CALL 112', view: 'VIEW INSTRUCTIONS', listen: 'Listen', isl: 'ISL Video',
    refresh: 'Last refreshed 2 min ago', map: 'Approved route preview', mapNote: 'Synthetic demo geometry · satellite layer unavailable pending government authorization.',
    demo: 'SYNTHETIC DEMO · NOT LIVE EMERGENCY GUIDANCE', voiceLabel: 'Voice Map Control', voiceOpen: 'Open voice guide', voiceTitle: 'Voice-to-text guide',
    voiceBody: 'Use an approved short voice command to move the map or repeat an instruction.', voiceStart: 'START LISTENING', voiceStop: 'STOP LISTENING',
    voiceStatus: 'IndicConformer model not connected', transcript: 'Transcript will appear here', privacy: 'No microphone recording is active in this demo.', close: 'Close voice guide',
  },
  ML: {
    language: 'മലയാളം', alert: 'ചുവപ്പ്', action: 'ഇപ്പോൾ പ്രവർത്തിക്കുക', hazard: 'വെള്ളപ്പൊക്കം',
    instruction: 'ഇപ്പോൾ നിയോഗിച്ച സുരക്ഷിത കേന്ദ്രത്തിലേക്ക് മാറുക.', detail: 'വാർഡ് 12-ൽ ഗുരുതര വെള്ളപ്പൊക്കം.',
    steps: ['താഴെ കാണുന്ന സുരക്ഷിത മാർഗ്ഗം പിന്തുടരുക.', 'വെള്ളത്തിലൂടെ നടക്കുകയോ വാഹനമോടിക്കുകയോ ചെയ്യരുത്.', 'കുട്ടികൾ, മരുന്നുകൾ, തിരിച്ചറിയൽ രേഖകൾ എടുക്കുക.'],
    shelter: 'ഗവൺമെന്റ് സ്കൂൾ, വാർഡ് 8', leave: 'പുറപ്പെടേണ്ട സമയം: വൈകിട്ട് 6:00', source: 'ജില്ലാ ദുരന്ത നിവാരണ അതോറിറ്റി · SYNTHETIC_DEMO',
    route: 'സുരക്ഷിത മാർഗ്ഗം തുടങ്ങുക', call: '112 വിളിക്കുക', view: 'നിർദ്ദേശങ്ങൾ കാണുക', listen: 'കേൾക്കുക', isl: 'ISL വീഡിയോ',
    refresh: 'അവസാനം പുതുക്കിയത് 2 മിനിറ്റ് മുമ്പ്', map: 'അനുവദിച്ച മാർഗ്ഗത്തിന്റെ പ്രിവ്യൂ', mapNote: 'സിന്തറ്റിക് ഡെമോ ജ്യാമിതി · സർക്കാർ അനുമതി കാത്തിരിക്കുന്നതിനാൽ സാറ്റലൈറ്റ് ലെയർ ലഭ്യമല്ല.',
    demo: 'സിന്തറ്റിക് ഡെമോ · തത്സമയ അടിയന്തര മാർഗ്ഗനിർദ്ദേശമല്ല', voiceLabel: 'വോയ്സ് മാപ്പ് നിയന്ത്രണം', voiceOpen: 'വോയ്സ് ഗൈഡ് തുറക്കുക', voiceTitle: 'വോയ്സ്-ടു-ടെക്സ്റ്റ് ഗൈഡ്',
    voiceBody: 'മാപ്പ് നീക്കാനോ നിർദ്ദേശം ആവർത്തിക്കാനോ അംഗീകൃത ഹ്രസ്വ വോയ്സ് കമാൻഡ് ഉപയോഗിക്കുക.', voiceStart: 'കേൾക്കാൻ തുടങ്ങുക', voiceStop: 'കേൾക്കുന്നത് നിർത്തുക',
    voiceStatus: 'IndicConformer മോഡൽ ബന്ധിപ്പിച്ചിട്ടില്ല', transcript: 'ട്രാൻസ്‌ക്രിപ്റ്റ് ഇവിടെ കാണിക്കും', privacy: 'ഈ ഡെമോയിൽ മൈക്രോഫോൺ റെക്കോർഡിംഗ് സജീവമല്ല.', close: 'വോയ്സ് ഗൈഡ് അടയ്ക്കുക',
  },
  HI: {
    language: 'हिन्दी', alert: 'लाल', action: 'अभी कार्रवाई करें', hazard: 'बाढ़',
    instruction: 'अभी अपने निर्धारित सुरक्षित आश्रय में जाएं।', detail: 'वार्ड 12 में गंभीर बाढ़ का असर है।',
    steps: ['नीचे दिखाए गए सुरक्षित मार्ग का पालन करें।', 'बाढ़ के पानी में पैदल या वाहन से न जाएं।', 'बच्चे, दवाइयां और पहचान पत्र साथ लें।'],
    shelter: 'सरकारी स्कूल, वार्ड 8', leave: 'पहले निकलें: शाम 6:00 बजे', source: 'जिला आपदा प्राधिकरण · SYNTHETIC_DEMO',
    route: 'सुरक्षित मार्ग शुरू करें', call: '112 पर कॉल करें', view: 'निर्देश देखें', listen: 'सुनें', isl: 'ISL वीडियो',
    refresh: '2 मिनट पहले अपडेट', map: 'अनुमोदित मार्ग का पूर्वावलोकन', mapNote: 'सिंथेटिक डेमो ज्यामिति · सरकारी अनुमति तक सैटेलाइट परत उपलब्ध नहीं है।',
    demo: 'सिंथेटिक डेमो · लाइव आपातकालीन मार्गदर्शन नहीं', voiceLabel: 'वॉयस मैप कंट्रोल', voiceOpen: 'वॉयस गाइड खोलें', voiceTitle: 'वॉयस-टू-टेक्स्ट गाइड',
    voiceBody: 'मानचित्र को स्थानांतरित करने या निर्देश दोहराने के लिए स्वीकृत छोटा वॉयस कमांड इस्तेमाल करें।', voiceStart: 'सुनना शुरू करें', voiceStop: 'सुनना बंद करें',
    voiceStatus: 'IndicConformer मॉडल जुड़ा नहीं है', transcript: 'ट्रांसक्रिप्ट यहां दिखाई देगा', privacy: 'इस डेमो में माइक्रोफोन रिकॉर्डिंग सक्रिय नहीं है।', close: 'वॉयस गाइड बंद करें',
  },
} as const;

let language: Language = 'EN';
let voiceOpen = false;
let voiceListening = false;

function render() {
  const copy = translations[language];
  document.querySelector<HTMLDivElement>('#app')!.innerHTML = `
    <div class="app-shell emergency-shell">
      <header class="topbar">
        <a class="wordmark" href="/" aria-label="Sthira home"><span class="wordmark-mark">S</span><span>Sthira</span></a>
        <div class="topbar-actions">
          <span class="demo-pill"><span class="status-dot"></span>${copy.demo}</span>
          <button class="voice-pet-button" type="button" aria-label="${copy.voiceOpen}" aria-expanded="${voiceOpen}" data-action="voice"><span class="voice-pet" aria-hidden="true"><i></i><b></b></span><span class="voice-pet-label">${copy.voiceLabel}</span></button>
          <div class="language-switcher" role="group" aria-label="Language"><button class="language-choice ${language === 'EN' ? 'is-selected' : ''}" data-language="EN" type="button">English</button><button class="language-choice ${language === 'ML' ? 'is-selected' : ''}" data-language="ML" type="button">മലയാളം</button><button class="language-choice ${language === 'HI' ? 'is-selected' : ''}" data-language="HI" type="button">हिन्दी</button></div>
        </div>
      </header>
      <main class="emergency-main">
        <section class="alert-screen" aria-labelledby="alert-title">
          <div class="severity-banner severity-red"><span aria-hidden="true">!</span><strong>${copy.alert}</strong><span aria-hidden="true">•</span><strong>${copy.action}</strong><span aria-hidden="true">•</span><strong>${copy.hazard}</strong></div>
          <div class="alert-content">
            <p class="demo-kicker">${copy.refresh}</p>
            <h1 id="alert-title">${copy.instruction}</h1><p class="hazard-detail">${copy.detail}</p>
            <ol class="emergency-steps">${copy.steps.map((step, index) => `<li><span>${index + 1}</span><p>${step}</p></li>`).join('')}</ol>
            <div class="destination"><span class="destination-icon" aria-hidden="true">⌖</span><div><strong>${copy.shelter}</strong><span>${copy.leave}</span></div></div>
            <p class="source-line">${copy.source}</p>
            <div class="primary-actions"><button class="large-button route-button" type="button" data-action="route">${copy.route}</button><a class="large-button emergency-button" href="tel:112">${copy.call}</a></div>
            <div class="secondary-actions"><button class="outline-button" type="button" data-action="instructions">${copy.view}</button><button class="outline-button" type="button" data-action="listen">▶ ${copy.listen}</button><button class="outline-button" type="button" data-action="isl">◉ ${copy.isl}</button></div>
          </div>
        </section>
        <section class="map-panel compact-map" aria-labelledby="map-title"><div class="panel-heading"><div><p class="eyebrow">${copy.map}</p><h2 id="map-title">${copy.shelter}</h2></div><span class="map-status">DEMO</span></div><div class="map-stage" role="img" aria-label="Synthetic map preview showing an approved route and safe shelter"><div class="map-grid"></div><div class="zone zone-red"><span>${copy.hazard}</span></div><div class="route-line"></div><div class="safe-zone"><span>SAFE</span></div><div class="map-pin">S</div><div class="map-legend"><span><i class="legend-red"></i>${copy.hazard}</span><span><i class="legend-route"></i>${copy.route}</span></div></div><p class="panel-note"><span class="info-icon">i</span>${copy.mapNote}</p></section>
      </main>
      <footer class="footer"><span>Sthira v2 · ${copy.demo}</span><span>${copy.source}</span></footer>
      <div class="toast" role="status" aria-live="polite" hidden></div>
      <aside class="voice-panel ${voiceOpen ? 'is-open' : ''}" aria-labelledby="voice-title" ${voiceOpen ? '' : 'hidden'}><div class="voice-panel-head"><div><p class="eyebrow">${copy.voiceLabel}</p><h2 id="voice-title">${copy.voiceTitle}</h2></div><button class="voice-close" type="button" aria-label="${copy.close}" data-action="voice-close">×</button></div><p class="voice-body">${copy.voiceBody}</p><div class="voice-status"><span class="status-dot"></span><span>${copy.voiceStatus}</span></div><div class="voice-transcript" aria-live="polite"><span>${copy.transcript}</span><strong>${voiceListening ? 'Listening preview...' : 'SHOW MY LOCATION'}</strong></div><button class="voice-listen-button ${voiceListening ? 'is-listening' : ''}" type="button" aria-pressed="${voiceListening}" data-action="voice-listen"><span class="voice-mic" aria-hidden="true">${voiceListening ? '■' : '●'}</span>${voiceListening ? copy.voiceStop : copy.voiceStart}</button><p class="voice-privacy">${copy.privacy}</p></aside>
    </div>`;
  bindInteractions();
}

function bindInteractions() {
  document.querySelectorAll<HTMLButtonElement>('[data-language]').forEach((button) => button.addEventListener('click', () => { language = button.dataset.language as Language; render(); }));
  document.querySelector<HTMLButtonElement>('[data-action="voice"]')?.addEventListener('click', () => { voiceOpen = true; render(); document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.focus(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-close"]')?.addEventListener('click', () => { voiceOpen = false; voiceListening = false; render(); document.querySelector<HTMLButtonElement>('[data-action="voice"]')?.focus(); });
  document.querySelector<HTMLButtonElement>('[data-action="voice-listen"]')?.addEventListener('click', () => { voiceListening = !voiceListening; render(); });
  document.querySelector<HTMLButtonElement>('[data-action="route"]')?.addEventListener('click', () => showToast('Approved route selection will connect to the operational package.'));
  document.querySelector<HTMLButtonElement>('[data-action="instructions"]')?.addEventListener('click', () => showToast('Instructions are sourced from the authorized alert package.'));
  document.querySelector<HTMLButtonElement>('[data-action="listen"]')?.addEventListener('click', () => showToast('Audio is not connected in this demo. Text remains available.'));
  document.querySelector<HTMLButtonElement>('[data-action="isl"]')?.addEventListener('click', () => showToast('Approved ISL media is pending review.'));
}

function showToast(message: string) {
  const toast = document.querySelector<HTMLDivElement>('.toast');
  if (!toast) return;
  toast.textContent = message; toast.hidden = false; window.setTimeout(() => { toast.hidden = true; }, 3600);
}

render();
