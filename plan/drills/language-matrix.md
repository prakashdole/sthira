# Regional Language and Reviewer Matrix (Q01)

**Phase / Task**: Q01 — Regional scenarios, languages and participant-ready drill inputs  
**Status**: DRAFT_INPUTS_QUALIFIED (Synthetic baseline verified; external native-speaker signoff required for live launch)  
**Governance**: Rules 9, 10, 11 (GEMINI.md / rules.md); O03 / O11 (open-decisions.md)

---

## 1. Supported Service Languages vs Planned Scope

| Language Code | Language Name | Primary States | ASR Status | TTS Status | Evaluation Status | ISL Media Policy |
| --- | --- | --- | --- | --- | --- | --- |
| `ml-IN` | Malayalam | Kerala (KL) | **ENABLED** (IndicConformer) | **ENABLED** (Indic Parler-TTS) | PASS (28/28 eval tests, synthetic n=15) | Approved video media only; no text mislabeling |
| `hi-IN` | Hindi | Himachal Pradesh (HP), Uttarakhand (UK) | **ENABLED** (IndicConformer) | **ENABLED** (Indic Parler-TTS) | PASS (28/28 eval tests, synthetic n=4) | Approved video media only; no text mislabeling |
| `en-IN` | Indian English | Pan-India | Fallback text | Fallback text | Clean fallback (non-voice UI) | Approved video media only |
| `as-IN` | Assamese | Assam (AS) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `mr-IN` | Marathi | Maharashtra (MH) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `ta-IN` | Tamil | Tamil Nadu (TN) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `kn-IN` | Kannada | Karnataka (KA) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `or-IN` | Odia | Odisha (OD) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `ne-IN` | Nepali | Sikkim (SK) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |
| `bn-IN` | Bengali | West Bengal (WB) | PLANNED (O03 open) | PLANNED (O03 open) | Gaps reported truthfully | Media required upon enablement |

---

## 2. Indian Sign Language (ISL) Standards & Boundaries

Per Standing Engineering Rules and Rule 11:
1. **Zero Text-as-Sign**: English or vernacular text strings MUST NEVER be labelled as "Sign Language". Text captions are an assistive visual channel, not sign language.
2. **Approved Visual Media**: ISL instructions must use pre-approved, certified signed video media recorded by certified ISL interpreters (conforming to ISLRTC standards).
3. **No Machine Synthesis of Signs**: No automated avatar generation or synthetic hand animation is permitted in the production emergency path without qualified human certification.

---

## 3. Qualified Reviewer Qualifications & Consent Protocol

1. **Reviewer Qualifications**:
   - Qualified native-speaker linguist or certified disaster communications officer.
   - For accessibility & ISL: certified ISL interpreter or disability accessibility expert (conforming to GIGW 3.0 / WCAG 2.2 AAA emergency requirements).
2. **Privacy & Data Minimization**:
   - Explicit written consent is required before logging reviewer observations.
   - Reviewer identity is pseudonymized in public artifacts (e.g. `REV-ML-01`, `REV-HI-02`).
   - Contact information is kept in secure access-controlled records, never committed to git repositories.
3. **No Unsolicited Outreach**:
   - Strict prohibition against unsolicited outreach to government officials, emergency dispatchers (112), or vulnerable citizens.
   - Recruitment of children or dependent populations for drills is strictly prohibited without explicit institutional review board (IRB) / guardian authorization.
