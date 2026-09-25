"""Local deterministic guidance and controls for the synthetic map."""

from sthira_v2.contracts import ChatRequest, ChatResponse, MapAction


_COPY = {
    "EN": {
        "route": "Use the approved Ridge Road route to the Ward 8 school north gate. I have shown the full 1.4 km route on the map. Avoid Main Canal Road bridge because it is marked submerged in this exercise.",
        "shelter": "Your assigned exercise shelter is Government Higher Secondary School in Ward 8. Use the north gate. The demo package lists medical support there.",
        "hazard": "The shaded red area is the synthetic flood zone near the river basin. Keep outside it and avoid the Main Canal Road bridge.",
        "rescue": "If you are trapped or cut off, call 112. I can open the call confirmation, but Sthira cannot dispatch help or place a silent call.",
        "arrival": "I can open the arrival check so you can confirm how many people reached the shelter. This demo will not change live capacity.",
        "time": "The exercise guidance says to leave Ward 12 before 6:00 PM. Start on the approved route now if it is safe to move.",
        "pack": "Take essential medicines, drinking water, a charged phone, and identity documents. Keep the load light and do not delay evacuation to collect belongings.",
        "support": "Keep children, older adults, and anyone needing assistance with the group. The assigned shelter lists medical support; call 112 if anyone cannot move safely.",
        "why": "Main Canal Road bridge is marked submerged in this exercise package, so the approved route stays on the elevated Ridge Road stretch.",
        "hello": "I am the Sthira guidance assistant. Ask me about the safe shelter, route, flood area, what to carry, arrival, or emergency help.",
        "fallback": "The general-purpose AI is temporarily unavailable. Map and emergency exercise controls still work.",
    },
    "ML": {
        "route": "വാർഡ് 8 സ്കൂളിന്റെ വടക്കേ ഗേറ്റിലേക്കുള്ള അംഗീകൃത റിഡ്ജ് റോഡ് വഴി ഉപയോഗിക്കുക. 1.4 കിലോമീറ്റർ പൂർണ്ണവഴി മാപ്പിൽ കാണിച്ചിട്ടുണ്ട്. ഈ അഭ്യാസത്തിൽ മെയിൻ കനാൽ റോഡ് പാലം വെള്ളത്തിനടിയിലായതിനാൽ ഒഴിവാക്കുക.",
        "shelter": "നിങ്ങൾക്ക് നൽകിയിരിക്കുന്ന അഭ്യാസ അഭയകേന്ദ്രം വാർഡ് 8 ഗവൺമെന്റ് ഹയർ സെക്കൻഡറി സ്കൂളാണ്. വടക്കേ ഗേറ്റ് ഉപയോഗിക്കുക. അവിടെ മെഡിക്കൽ സഹായം ലഭ്യമെന്നാണ് ഡെമോ വിവരങ്ങൾ.",
        "hazard": "ചുവപ്പായി കാണിച്ചിരിക്കുന്നത് നദീതടത്തിന് സമീപമുള്ള സിന്തറ്റിക് വെള്ളപ്പൊക്ക മേഖലയാണ്. അതിന് പുറത്തുനിൽക്കുകയും മെയിൻ കനാൽ റോഡ് പാലം ഒഴിവാക്കുകയും ചെയ്യുക.",
        "rescue": "നിങ്ങൾ കുടുങ്ങിയിരിക്കുകയോ വഴി മുടങ്ങിയിരിക്കുകയോ ആണെങ്കിൽ 112 വിളിക്കുക. കോൾ സ്ഥിരീകരണം തുറക്കാം, പക്ഷേ സ്തിരയ്ക്ക് സഹായം അയയ്ക്കാനോ രഹസ്യമായി കോൾ ചെയ്യാനോ കഴിയില്ല.",
        "arrival": "എത്ര പേർ അഭയകേന്ദ്രത്തിലെത്തി എന്ന് സ്ഥിരീകരിക്കാൻ അറൈവൽ ചെക്ക് തുറക്കാം. ഈ ഡെമോ തത്സമയ ശേഷി മാറ്റില്ല.",
        "time": "അഭ്യാസ നിർദേശം വൈകിട്ട് 6:00-ന് മുമ്പ് വാർഡ് 12 വിടാനാണ്. നീങ്ങുന്നത് സുരക്ഷിതമാണെങ്കിൽ അംഗീകൃത വഴിയിലൂടെ ഇപ്പോൾ പുറപ്പെടുക.",
        "pack": "അത്യാവശ്യ മരുന്നുകൾ, കുടിവെള്ളം, ചാർജ് ചെയ്ത ഫോൺ, തിരിച്ചറിയൽ രേഖകൾ എന്നിവ എടുക്കുക. ഭാരം കുറച്ച് വയ്ക്കുക; സാധനങ്ങൾ ശേഖരിക്കാൻ ഒഴിപ്പിക്കൽ വൈകിക്കരുത്.",
        "support": "കുട്ടികളെയും മുതിർന്നവരെയും സഹായം ആവശ്യമുള്ളവരെയും സംഘത്തോടൊപ്പം നിർത്തുക. അഭയകേന്ദ്രത്തിൽ മെഡിക്കൽ സഹായമുണ്ട്; സുരക്ഷിതമായി നീങ്ങാൻ കഴിയില്ലെങ്കിൽ 112 വിളിക്കുക.",
        "why": "ഈ അഭ്യാസ പാക്കേജിൽ മെയിൻ കനാൽ റോഡ് പാലം വെള്ളത്തിനടിയിലാണെന്ന് രേഖപ്പെടുത്തിയതിനാൽ അംഗീകൃത വഴി ഉയർന്ന റിഡ്ജ് റോഡിലൂടെയാണ്.",
        "hello": "ഞാൻ സ്തിര മാർഗനിർദേശ സഹായി ആണ്. അഭയകേന്ദ്രം, വഴി, വെള്ളപ്പൊക്ക മേഖല, കൊണ്ടുപോകേണ്ടവ, എത്തിച്ചേരൽ, അടിയന്തര സഹായം എന്നിവ ചോദിക്കാം.",
        "fallback": "പൊതുവായ AI താൽക്കാലികമായി ലഭ്യമല്ല. മാപ്പും അടിയന്തര അഭ്യാസ നിയന്ത്രണങ്ങളും ഇപ്പോഴും പ്രവർത്തിക്കും.",
    },
    "HI": {
        "route": "वार्ड 8 स्कूल के उत्तरी गेट तक स्वीकृत रिज रोड मार्ग लें। मैंने 1.4 किमी का पूरा मार्ग मानचित्र पर दिखाया है। इस अभ्यास में मेन कैनाल रोड पुल डूबा हुआ चिह्नित है, इसलिए उससे बचें।",
        "shelter": "आपका निर्धारित अभ्यास आश्रय वार्ड 8 का गवर्नमेंट हायर सेकेंडरी स्कूल है। उत्तरी गेट का उपयोग करें। डेमो पैकेज के अनुसार वहाँ चिकित्सा सहायता है।",
        "hazard": "लाल छायांकित भाग नदी क्षेत्र के पास कृत्रिम बाढ़ क्षेत्र है। उससे बाहर रहें और मेन कैनाल रोड पुल से बचें।",
        "rescue": "यदि आप फँसे हैं या रास्ता बंद है, तो 112 पर कॉल करें। मैं कॉल पुष्टि खोल सकता हूँ, लेकिन स्थिर मदद भेजने या चुपचाप कॉल करने का दावा नहीं कर सकता।",
        "arrival": "मैं आगमन जाँच खोल सकता हूँ ताकि आप बता सकें कि कितने लोग आश्रय पहुँचे। यह डेमो लाइव क्षमता नहीं बदलेगा।",
        "time": "अभ्यास मार्गदर्शन वार्ड 12 को शाम 6:00 बजे से पहले छोड़ने को कहता है। यदि चलना सुरक्षित है तो अभी स्वीकृत मार्ग लें।",
        "pack": "ज़रूरी दवाएँ, पीने का पानी, चार्ज किया हुआ फोन और पहचान दस्तावेज़ लें। सामान हल्का रखें और चीज़ें इकट्ठी करने के लिए निकासी में देरी न करें।",
        "support": "बच्चों, बुज़ुर्गों और सहायता चाहने वाले लोगों को समूह के साथ रखें। निर्धारित आश्रय में चिकित्सा सहायता है; सुरक्षित रूप से चलना संभव न हो तो 112 पर कॉल करें।",
        "why": "इस अभ्यास पैकेज में मेन कैनाल रोड पुल डूबा हुआ दर्ज है, इसलिए स्वीकृत मार्ग ऊँचे रिज रोड हिस्से पर रहता है।",
        "hello": "मैं स्थिर मार्गदर्शन सहायक हूँ। सुरक्षित आश्रय, मार्ग, बाढ़ क्षेत्र, साथ ले जाने वाली चीज़ें, आगमन या आपात सहायता के बारे में पूछें।",
        "fallback": "सामान्य AI अभी उपलब्ध नहीं है। मानचित्र और आपात अभ्यास नियंत्रण अभी भी काम करेंगे।",
    },
}


def _contains(text: str, terms: tuple[str, ...]) -> bool:
    return any(term in text for term in terms)


def _map_response(request: ChatRequest) -> tuple[str, MapAction, tuple[str, ...]]:
    """Select only trusted UI actions; this never decides the model's answer."""
    latest = request.messages[-1].text.casefold()
    context = " ".join(message.text.casefold() for message in request.messages[-6:])
    copy = _COPY[request.language]
    key = "fallback"
    action = MapAction.NONE

    if _contains(latest, ("112", "rescue", "trapped", "stuck", "send help", "രക്ഷ", "കുടുങ്ങ", "बचाव", "फँस", "आपात मदद")):
        key, action = "rescue", MapAction.OPEN_RESCUE
    elif _contains(latest, ("arriv", "reached", "at the shelter", "എത്തി", "പहुंच", "पहुँच")):
        key, action = "arrival", MapAction.CONFIRM_ARRIVAL
    elif _contains(latest, ("directions to the shelter", "directions to shelter", "how do i get to the shelter", "how to get to the shelter", "navigate to shelter", "വഴികാട്ട", "कैसे पहुँच", "दिशा")):
        key, action = "route", MapAction.OPEN_DIRECTIONS
    elif _contains(latest, ("show route", "show me the route", "evacuation route", "safe route", "route to shelter", "ridge road", "വഴി", "റോഡ്", "मार्ग", "रास्त")):
        key, action = "route", MapAction.SHOW_ROUTE
    elif _contains(latest, ("shelter", "safe zone", "safe place", "where should i evacuate", "അഭയ", "കേന്ദ്ര", "आश्रय", "सुरक्षित स्थान")):
        key, action = "shelter", MapAction.FOCUS_SHELTER
    elif _contains(latest, ("flood zone", "red zone", "hazard area", "danger zone", "show hazard", "വെള്ള", "അപകട", "बाढ़", "खतरा")):
        key, action = "hazard", MapAction.SHOW_HAZARD
    elif _contains(latest, ("bring", "carry", "pack", "take with", "കൊണ്ടുപോക", "എടുക്ക", "साथ ले", "क्या लाऊ")):
        key = "pack"
    elif _contains(latest, ("child", "children", "elder", "old person", "pregnan", "wheelchair", "disab", "കുട്ട", "വൃദ്ധ", "बच्च", "बुज़ुर्ग", "गर्भ")):
        key = "support"
    elif _contains(latest, ("when", "deadline", "what time", "എപ്പോൾ", "സമയം", "कब", "समय")):
        key = "time"
    elif _contains(latest, ("why", "bridge", "എന്തുകൊണ്ട്", "പാലം", "क्यों", "पुल")) or (
        len(latest.split()) <= 5 and _contains(context, ("route", "road", "വഴി", "मार्ग"))
    ):
        key, action = "why", MapAction.SHOW_ROUTE
    elif _contains(latest, ("hello", "hi", "hey", "നമസ്കാരം", "ഹലോ", "नमस्ते", "हेलो", "thanks", "thank you")):
        key = "hello"

    suggestions = {
        "route": ("Why this route?", "What should I carry?", "I need rescue"),
        "shelter": ("Show directions", "Is medical help available?", "I have arrived"),
        "rescue": ("Open call confirmation", "Show my shelter", "Show the route"),
    }.get(key, ("Show my safe shelter", "How do I get there?", "What should I carry?"))
    return copy[key], action, suggestions


def respond(request: ChatRequest) -> ChatResponse:
    """Return local exercise guidance and an independently validated map action."""

    reply, action, suggestions = _map_response(request)
    return ChatResponse(
        reply=reply[:2_000],
        map_action=action,
        suggestions=suggestions,
        provider="LOCAL_GUIDANCE",
    )
