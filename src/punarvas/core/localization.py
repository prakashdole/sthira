"""
PUNARVAS-AI Trilingual Localization Engine (EN / ML / HI).
Normative Reference: rules.md (RUL-055), trd.md §3, phases.md §8 (PH-5 multi-state language pack).

Guarantees full status, warning, and non-map parity across:
- English (National / Technical baseline)
- Malayalam (Kerala State baseline: Wayanad, Idukki)
- Hindi (Northern State baseline: Uttarakhand, Himachal Pradesh, National)
"""

from typing import Dict, List, Optional


LOCALIZATION_REGISTRY: Dict[str, Dict[str, str]] = {
    "en": {
        "brand_title": "PUNARVAS-AI (ಪುನರ್ವಾಸ್ / पुनर्वास)",
        "brand_subtitle": "Proactive Permanent Relocation Decision Support System",
        "advisory_banner": "ADVISORY DECISION SUPPORT ONLY — All outputs require DDMA review and authorized human approval.",
        "advisory_notice_full": (
            "This algorithmic analysis is advisory decision-support only. "
            "It does not constitute statutory clearance, title verification, or gazetted notification."
        ),
        "gate_pass": "PASS",
        "gate_fail": "FAIL",
        "gate_unknown": "UNKNOWN",
        "gate_blocked": "BLOCKED",
        "pathway_township": "Integrated Model Township",
        "pathway_self": "Self-Relocation Financial Assistance",
        "pathway_insitu": "In-Situ Risk Mitigation",
        "pathway_excluded": "Excluded / Safe",
        "role_approver": "Authorized Approver (District Collector / DDMA Chairperson)",
        "role_dmo": "Disaster Management Officer",
        "role_legal": "Legal & Rights Officer",
        "role_revenue": "Land & Revenue Officer",
        "role_gis": "GIS & Hazard Analyst",
        "role_field": "Field Verifier",
        "discrepancy_alert": "Land Truth Discrepancy Detected",
        "audit_valid": "Cryptographic Hash Chain Valid & Tamper-Evident",
    },
    "ml": {
        "brand_title": "പുനർവാസ്-എഐ (PUNARVAS-AI)",
        "brand_subtitle": "ശാശ്വത പുനരധിവാസ ഉപദേശക സംവിധാനം",
        "advisory_banner": "ഉപദേശക സ്വഭാവമുള്ളത് മാത്രം. ഔദ്യോഗിക ഗസറ്റ് വിജ്ഞാപനമോ ഉത്തരവോ അല്ല.",
        "advisory_notice_full": (
            "ഈ കമ്പ്യൂട്ടർ വിശകലനം തികച്ചും ഉപദേശക സ്വഭാവമുള്ളതാണ്. "
            "ഇത് നിയമപരമായ അംഗീകാരമോ, ഉടമസ്ഥതാ രേഖയോ, ഔദ്യോഗിക ഗസറ്റ് വിജ്ഞാപനമോ അല്ല."
        ),
        "gate_pass": "അംഗീകരിച്ചു (PASS)",
        "gate_fail": "നിരസിച്ചു (FAIL)",
        "gate_unknown": "തെളിവ് ലഭ്യമല്ല (UNKNOWN)",
        "gate_blocked": "തടസ്സപ്പെട്ടു (BLOCKED)",
        "pathway_township": "സംയോജിത മാതൃകാ ടൗൺഷിപ്പ്",
        "pathway_self": "സ്വയം പുനരധിവാസ സാമ്പത്തിക സഹായം (VLRS)",
        "pathway_insitu": "തദ്ദേശീയ ലഘൂകരണ പ്രവർത്തനങ്ങൾ",
        "pathway_excluded": "ഒഴിവാക്കപ്പെട്ടത് / സുരക്ഷിതം",
        "role_approver": "അംഗീകൃത ഉദ്യോഗസ്ഥൻ (ജില്ലാ കളക്ടർ / ചെയർപേഴ്സൺ ഡി.ഡി.എം.എ)",
        "role_dmo": "ദുരന്ത നിവാരണ ഓഫീസർ",
        "role_legal": "നിയമകാര്യ ഓഫീസർ",
        "role_revenue": "റവന്യൂ / ഭൂമി ഉദ്യോഗസ്ഥൻ",
        "role_gis": "ജി.ഐ.എസ് വിദഗ്ദ്ധൻ",
        "role_field": "ഫീൽഡ് പരിശോധകൻ",
        "discrepancy_alert": "ഭൂമി പരിശോധനാ പൊരുത്തക്കേട് കണ്ടെത്തി",
        "audit_valid": "ക്രിപ്റ്റോഗ്രാഫിക് ഹാഷ് ശൃംഖല ഭദ്രവും സുരക്ഷിതവുമാണ്",
    },
    "hi": {
        "brand_title": "पुनर्वास-एआई (PUNARVAS-AI)",
        "brand_subtitle": "सक्रिय स्थायी पुनर्वास निर्णय सहायता प्रणाली",
        "advisory_banner": "केवल सलाहकारी निर्णय सहायता — सभी परिणामों के लिए डीडीएमए समीक्षा और अधिकृत मानवीय अनुमोदन अनिवार्य है।",
        "advisory_notice_full": (
            "यह एल्गोरिदमिक विश्लेषण केवल सलाहकारी निर्णय सहायता है। "
            "यह कोई वैधानिक स्वीकृति, भू-स्वामित्व सत्यापन या राजपत्रित अधिसूचना नहीं है।"
        ),
        "gate_pass": "उत्तीर्ण (PASS)",
        "gate_fail": "अनुत्तीर्ण (FAIL)",
        "gate_unknown": "साक्ष्य अप्राप्त (UNKNOWN)",
        "gate_blocked": "अवरुद्ध (BLOCKED)",
        "pathway_township": "एकीकृत मॉडल टाउनशिप",
        "pathway_self": "स्वयं-पुनर्वास वित्तीय सहायता (मुख्यमंत्री पुनर्वास योजना)",
        "pathway_insitu": "यथास्थान जोखिम शमन",
        "pathway_excluded": "अपवर्जित / सुरक्षित",
        "role_approver": "अधिकृत अनुमोदनकर्ता (जिलाधिकारी / अध्यक्ष डीडीएमए)",
        "role_dmo": "आपदा प्रबंधन अधिकारी",
        "role_legal": "विधि एवं अधिकार अधिकारी",
        "role_revenue": "भूमि एवं राजस्व अधिकारी",
        "role_gis": "जीआईएस एवं भू-खतरा विश्लेषक",
        "role_field": "क्षेत्रीय सत्यापनकर्ता",
        "discrepancy_alert": "धरातलीय भू-अभिलेख विसंगति चिह्नित",
        "audit_valid": "क्रिप्टोग्राफिक हैश शृंखला अक्षुण्ण एवं छेड़छाड़-मुक्त",
    }
}


def translate_text(key: str, lang: str = "en") -> str:
    """Retrieve translated string with graceful English fallback."""
    lang_dict = LOCALIZATION_REGISTRY.get(lang, LOCALIZATION_REGISTRY["en"])
    return lang_dict.get(key, LOCALIZATION_REGISTRY["en"].get(key, key))


def get_supported_languages() -> List[str]:
    return list(LOCALIZATION_REGISTRY.keys())
