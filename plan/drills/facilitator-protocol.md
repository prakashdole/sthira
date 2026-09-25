# Facilitator Protocol and Safety Stop Rule (Q01)

**Phase / Task**: Q01 — Regional scenarios, languages and participant-ready drill inputs  
**Target Audience**: Drill Facilitators, Safety Officers, System Evaluators  
**Applies to**: Controlled desk simulations, indoor usability drills, and regional system exercises (Q02)

---

## 1. MANDATORY FACILITATOR STOP RULE

> [!CAUTION]
> **SAFETY STOP RULE (MANDATORY)**
> Prior to handing any device or display to a participant, the facilitator MUST read the following statement verbatim in the participant's primary language:
> 
> *"This is a controlled training exercise using synthetic test scenarios. The alerts, warnings, shelter locations, and evacuation routes shown on this screen are simulations for technology evaluation only. They are NOT real emergency orders and NOT real government alerts. In any real emergency, follow official instructions from local emergency authorities or call 112."*
> 
> **Immediate Termination Triggers**:
> The facilitator MUST immediately terminate the exercise, collect the test device, and clear the screen if:
> 1. Any participant demonstrates distress, anxiety, or confusion regarding whether the alert is real.
> 2. A real-world emergency event (actual severe weather, earthquake, flash flood) occurs in or near the exercise facility.
> 3. Any device attempts to contact 112 or external emergency services outside of the approved simulated phone dialler test boundary.
> 4. Any network failure or cache corruption displays unlabelled or confusing guidance that cannot be immediately rectified.

---

## 2. Participant Rights & Ethical Safeguards

1. **Voluntary Participation**:
   - Every participant must provide written informed consent prior to participating.
   - Any participant may pause, skip tasks, or withdraw completely at any moment without penalty or inquiry.
2. **Protection of Minors and Vulnerable Persons**:
   - Minors may not participate in drills unless specific guardian consent and independent institutional review board (IRB) clearance have been obtained.
   - For older adults and persons with disabilities, a designated assistant or caregiver must be permitted to accompany the participant throughout the session.
3. **Strict Zero-Surveillance Rule**:
   - No continuous background tracking.
   - Foreground tracking requires explicit, informed opt-in consent and can be stopped at any time.
   - GPS coordinates and raw voice recordings are NEVER saved to disk, telemetry, or server logs. Raw audio retention is zero (`RETENTION=0`).
   - Arrival at any destination requires **explicit citizen button touch / keyboard confirmation**; automated geofenced check-in is strictly prohibited.

---

## 3. Session Facilitation Checklist

### Pre-Drill Setup
- [ ] Verify test binary / staging instance is running in **exercise isolation** mode with `STHIRA_DEMO_CONTEXT=1` or `SYNTHETIC_EXERCISE` headers enabled.
- [ ] Confirm the UI banner prominently displays `SYNTHETIC EXERCISE - NOT A REAL ALERT` across all screens.
- [ ] Ensure local test network / loopback proxy is functioning; confirm zero live government API endpoints are dialed.
- [ ] Test the selected regional language (`ml-IN` or `hi-IN`) and confirm fallback text is fully visible.

### During Drill Execution
- [ ] Observe participant interaction without prompting or coaching.
- [ ] Note confusion points, retry attempts, tap target misses, and voice command misunderstandings.
- [ ] Confirm arrival prompt appears as an advisory suggestion and requires explicit manual button confirmation.

### Post-Drill Debrief
- [ ] Reset client storage (`localStorage.clear()`, unregister service workers).
- [ ] Record structured, anonymized task metrics:
  - Task completion status (PASS / FAIL / ASSISTED)
  - Time to destination choice (seconds)
  - TTS intelligibility score (1–5)
  - Observed confusion or navigation hesitation
- [ ] Provide participants with a debriefing summary reiterating the synthetic nature of the exercise.
