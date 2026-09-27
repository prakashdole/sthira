"""Round-3 prototype browser acceptance (headless Chromium, visible controls).

Instrumentation is limited to network interception (page.route, including
rewriting a real backend response into a stated fixture), browser geolocation
emulation, Chromium's fake microphone, and browser-API wrappers installed via
init scripts (getUserMedia/MediaRecorder/Audio) that record what the app does.
No application window setters are used. The map section loads the dev page
with ?sthira-test-hooks=1 only to READ the MapLibre instance (pitch, terrain,
layout visibility, rendered features). The reservation section reads the
disposable database named by STHIRA_ACCEPT_DB (psql) to count stays; without
it those checks FAIL rather than pass silently.
Synthetic stack only; this does not prove real microphones, audible speech
quality, Safari or real models.

Usage: python3 prototype_accept.py BASE_URL SECTION [SECTION...] [--json OUT]
Sections: core outage language reservation recorder audio audio-denied dialog map layout
Exit 0 only if every executed check passes.
"""
import asyncio, base64, hashlib, json, os, struct, subprocess, sys
from playwright.async_api import async_playwright, TimeoutError as PWTimeout

SHELTER = {"longitude": 76.105, "latitude": 11.570, "accuracy": 10}
EN_ASSISTANT_UNAVAILABLE = "Voice Map Control is unavailable. The displayed route and emergency call option still work."
RESULTS = []


def rec(section, name, ok, detail=""):
    RESULTS.append({"section": section, "check": name, "status": "PASS" if ok else "FAIL", "detail": str(detail)[:240]})
    print(("PASS" if ok else "FAIL"), f"[{section}]", name, str(detail)[:240], flush=True)


async def new_page(browser, init_script=None, **ctx_kw):
    ctx_kw.setdefault("viewport", {"width": 1280, "height": 860})
    ctx_kw.setdefault("geolocation", SHELTER)
    ctx_kw.setdefault("permissions", ["geolocation"])
    ctx = await browser.new_context(**ctx_kw)
    if init_script:
        await ctx.add_init_script(init_script)
    page = await ctx.new_page()
    page.errors = []
    page.on("pageerror", lambda e: page.errors.append(str(e)))
    return ctx, page


async def onboard(page, base, lang="EN", query=""):
    await page.goto(base + query)
    await page.wait_for_load_state("networkidle")
    await page.click('[data-action="onboarding-start"]')
    await page.click(f'[data-onboarding-language="{lang}"]')
    await page.click('[data-action="onboarding-language-next"]')
    await page.locator('[data-action="onboarding-complete"]').first.click()
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=15000)


async def open_console(page):
    if not await page.locator("#command-input").is_visible():
        await page.locator('[data-action="voice-open"]').first.click()
    await page.wait_for_selector("#command-input", state="visible", timeout=5000)


async def submit(page, text):
    """Submit via the visible form; wait until the input is enabled again."""
    await open_console(page)
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/voice/process"), timeout=15000) as info:
        await page.fill("#command-input", text)
        await page.click('[data-command-form] button[type="submit"]')
    resp = await info.value
    await page.wait_for_function("!document.querySelector('#command-input')?.disabled", timeout=15000)
    return resp


async def console_state(page):
    return await page.evaluate("""() => ({
        result: document.querySelector('.command-result p')?.textContent ?? null,
        errors: [...document.querySelectorAll('.voice-console .command-error')].map(e => e.textContent),
        text: document.querySelector('.voice-console')?.innerText ?? ''
    })""")


async def section_core(browser, base):
    s = "core"
    ctx, page = await new_page(browser)
    await page.goto(base)
    await page.wait_for_load_state("networkidle")
    rec(s, "test hooks absent without opt-in", await page.evaluate(
        "typeof window.setActiveStayId === 'undefined' && typeof window.sendVoiceOrText === 'undefined'"))
    await onboard(page, base)
    rec(s, "guidance destination rendered from backend", await page.locator("text=Demo Safe Facility").count() > 0)
    resp = await submit(page, "where can I go")
    body = await resp.json()
    tmpl = (body.get("data") or {}).get("template", {}).get("text", "")
    st = await console_state(page)
    rec(s, "text command: 200 and backend template text shown, no error",
        resp.status == 200 and tmpl and st["result"] == tmpl and not st["errors"], f"status={resp.status} result={st['result']!r} errors={st['errors']}")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


async def section_outage(browser, base):
    s = "outage"
    ctx, page = await new_page(browser)
    await onboard(page, base)
    def failing(code):
        async def fail(route):
            await route.fulfill(status=code, content_type="application/json", body='{"error":{"code":"MODEL_UNAVAILABLE"}}')
        return fail
    for code in (503, 504):
        await page.route("**/api/v3/voice/process", failing(code))
        resp = await submit(page, "Show my route")
        st = await console_state(page)
        rec(s, f"{code}: assistant-unavailable shown, no place-not-found, no success",
            resp.status == code and st["errors"] == [EN_ASSISTANT_UNAVAILABLE]
            and "not found" not in st["text"].lower() and "Destination choices" not in st["text"],
            f"errors={st['errors']} result={st['result']!r}")
        await page.unroute("**/api/v3/voice/process")
    resp = await submit(page, "where can I go")
    body = await resp.json()
    tmpl = (body.get("data") or {}).get("template", {}).get("text", "")
    st = await console_state(page)
    rec(s, "recovery: next request 200 applies and clears the error",
        resp.status == 200 and st["result"] == tmpl and not st["errors"], f"status={resp.status} result={st['result']!r} errors={st['errors']}")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


EN_MIC_STOPPED = "Microphone input stopped. Check permission or enter a command below."
EN_TEMPLATE = "Destination choices are displayed on screen."
HI_TEMPLATE = "गंतव्य विकल्प स्क्रीन पर प्रदर्शित हैं।"


def wav_silence(seconds, rate=16000):
    """PCM 16-bit mono WAV matching the mock TTS settings (16 kHz, 16 bit, 1 ch)."""
    data = b"\x00\x00" * int(seconds * rate)
    header = b"RIFF" + struct.pack("<I", 36 + len(data)) + b"WAVEfmt " + struct.pack("<IHHIIHH", 16, 1, 1, rate, rate * 2, 2, 16) + b"data" + struct.pack("<I", len(data))
    return header + data


LONG_WAV = wav_silence(6)
LONG_SHA = hashlib.sha256(LONG_WAV).hexdigest()
LONG_B64 = base64.b64encode(LONG_WAV).decode()


async def long_audio_route(route):
    """Real backend response; only the approved audio bytes are replaced by a 6 s clip
    whose byte_size and SHA-256 are recomputed so the app's integrity check still applies."""
    resp = await route.fetch()
    body = await resp.json()
    audio = (body.get("data") or {}).get("audio")
    if audio:
        audio.update(audio_b64=LONG_B64, byte_size=len(LONG_WAV), checksum_sha256=LONG_SHA, audio_id=LONG_SHA)
    await route.fulfill(response=resp, json=body)


def rewrite_actions(actions):
    async def handler(route):
        resp = await route.fetch()
        body = await resp.json()
        body["data"]["validated_proposal"]["actions"] = actions
        await route.fulfill(response=resp, json=body)
    return handler


def db_scalar(sql):
    db = os.environ.get("STHIRA_ACCEPT_DB")
    if not db:
        return None
    out = subprocess.run(["psql", "-X", "-At", "-d", db, "-c", sql], capture_output=True, text=True, timeout=10)
    return out.stdout.strip() if out.returncode == 0 else None


async def settle(page, ms=300):
    await page.wait_for_timeout(ms)


# ---------------------------------------------------------------- language

async def section_language(browser, base):
    """Old-language response delayed past a language switch and a fresh request."""
    s = "language"
    ctx, page = await new_page(browser)
    await onboard(page, base)
    await open_console(page)
    release = asyncio.Event()
    seen = []

    async def hold_first(route):
        body = json.loads(route.request.post_data or "{}")
        seen.append(body.get("language"))
        if len(seen) == 1:
            await release.wait()
        await route.continue_()

    await page.route("**/api/v3/voice/process", hold_first)
    await page.fill("#command-input", "where can I go")
    await page.click('[data-command-form] button[type="submit"]')
    await page.wait_for_function("document.querySelector('#command-input')?.disabled === true", timeout=5000)
    await page.click('[data-language="HI"]')
    rec(s, "switch while pending re-enables input immediately", await page.locator("#command-input").is_enabled())
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/voice/process"), timeout=15000) as fresh:
        await page.fill("#command-input", "where can I go")
        await page.click('[data-command-form] button[type="submit"]')
    fresh_resp = await fresh.value
    await page.wait_for_function("!document.querySelector('#command-input')?.disabled", timeout=15000)
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/voice/process"), timeout=15000) as late:
        release.set()
    late_resp = await late.value
    await settle(page, 800)
    st = await console_state(page)
    rec(s, "requests carried en-IN then hi-IN", seen == ["en-IN", "hi-IN"], seen)
    rec(s, "late en-IN 200 response ignored; fresh hi-IN template stays displayed",
        late_resp.status == 200 and fresh_resp.status == 200 and st["result"] == HI_TEMPLATE and EN_TEMPLATE not in st["text"] and not st["errors"],
        f"result={st['result']!r} errors={st['errors']}")
    await page.unroute("**/api/v3/voice/process")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- reservation

STAYS_SQL = "select count(*) from stays"


async def section_reservation(browser, base):
    """Lost response after server commit -> identical retry -> one stay -> GPS near -> arrival ack."""
    s = "reservation"
    before = db_scalar(STAYS_SQL)
    ctx, page = await new_page(browser)
    await onboard(page, base)
    posts = []
    page.on("request", lambda r: posts.append(r.post_data) if r.method == "POST" and r.url.endswith("/api/v3/reservations") else None)
    committed = []

    async def commit_then_drop(route):
        resp = await route.fetch()  # the server receives and commits the reservation
        committed.append(resp.status)
        await route.abort("failed")  # ... and the client never sees the response

    await page.route("**/api/v3/reservations", commit_then_drop)
    await page.locator('[data-action="route"]').first.click()
    await page.wait_for_function("!document.querySelector('[data-action=\"route\"]')?.textContent.includes('Reserving')", timeout=10000)
    await page.unroute("**/api/v3/reservations")
    panel = await page.locator('[data-testid="emergency-card"]').inner_text()
    rec(s, "server committed the first attempt (201) while the client lost the response", committed == [201], committed)
    rec(s, "uncertain outcome is visible to the citizen", "not confirmed" in panel, panel[:200])
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/reservations"), timeout=10000) as info:
        await page.locator('[data-action="route"]').first.click()
    retry = await info.value
    retry_body = await retry.json()
    stay_id = (retry_body.get("data") or {}).get("stay_id")
    same = len(posts) == 2 and posts[0] == posts[1] and json.loads(posts[0]).get("idempotency_key")
    rec(s, "retry is byte-identical with the same idempotency key", bool(same), f"posts={len(posts)}")
    safe_id = stay_id if stay_id and stay_id.replace("-", "").isalnum() else None
    fac = db_scalar(f"select facility_id from stays where stay_id = '{safe_id}'") if safe_id else None
    rec(s, "retry replays the committed stay (200 not 201; stay at FACDEMO-1)",
        retry.status == 200 and fac == "FACDEMO-1", f"status={retry.status} stay={stay_id} db_facility={fac}")
    after = db_scalar(STAYS_SQL)
    rec(s, "database holds exactly one new stay", before is not None and after is not None and int(after) - int(before) == 1, f"before={before} after={after}")
    await settle(page, 500)
    route_label = await page.locator('[data-action="route"]').first.inner_text()
    rec(s, "route shows active after acknowledgement", "Route active" in route_label, route_label)
    n = len(posts)
    if await page.locator('[data-action="directions-close"]').count():
        await page.locator('[data-action="directions-close"]').first.click()
    await page.locator('[data-action="route"]').first.click()
    await settle(page, 1200)
    rec(s, "repeated Start Route sends no new reservation", len(posts) == n, f"posts={len(posts)}")
    rec(s, "destination stays bound to the accepted facility",
        "Demo Safe Facility" in await page.locator(".destination h2").inner_text())
    await page.wait_for_selector('[data-action="arrival-open"]', timeout=10000)
    await page.locator('[data-action="arrival-open"]').first.click()
    async with page.expect_response(lambda r: "/events" in r.url and r.request.method == "POST", timeout=10000) as ev:
        await page.click('[data-action="arrival-yes"]')
    arrive = await ev.value
    await page.wait_for_selector("text=Recorded at", timeout=8000)
    state = db_scalar(f"select state from stays where stay_id = '{safe_id}'") if safe_id else None
    rec(s, "arrival recorded after server acknowledgement (ARRIVE 200, stay ARRIVED)", arrive.status == 200 and state == "ARRIVED", f"status={arrive.status} db_state={state}")
    await page.reload()
    await page.wait_for_load_state("networkidle")
    if await page.locator('[data-action="onboarding-start"]').count():
        await onboard(page, base)
    n = len(posts)
    await page.locator('[data-action="route"]').first.click()
    await settle(page, 1200)
    rec(s, "after reload the accepted stay is restored, no new allocation",
        len(posts) == n and "Demo Safe Facility" in await page.locator(".destination h2").inner_text(), f"posts={len(posts)}")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- recorder

RECORDER_INIT = """(() => {
  window.__streams = []; window.__gumCalls = 0; window.__gumDelays = []; window.__recorders = [];
  window.__failStart = false; window.__noMime = false; window.__gumReject = false;
  const md = navigator.mediaDevices; const realGUM = md.getUserMedia.bind(md);
  md.getUserMedia = async (c) => {
    window.__gumCalls++; const d = window.__gumDelays.shift() || 0;
    if (window.__gumReject) throw new DOMException('instrumented permission denial', 'NotAllowedError');
    const st = await realGUM(c); window.__streams.push(st);
    if (d) await new Promise((r) => setTimeout(r, d));
    return st;
  };
  const Real = window.MediaRecorder;
  class Recorder extends Real {
    constructor(st, o) { super(st, o); window.__recorders.push(this); }
    start(...a) { if (window.__failStart) throw new DOMException('instrumented start failure', 'NotSupportedError'); return super.start(...a); }
    static isTypeSupported(m) { return window.__noMime ? false : Real.isTypeSupported(m); }
  }
  window.MediaRecorder = Recorder;
})();"""

REC_STATE = """() => ({
  live: window.__streams.flatMap((st) => st.getTracks()).filter((t) => t.readyState === 'live').length,
  gum: window.__gumCalls,
  recorders: window.__recorders.map((r) => r.state),
  pressed: document.querySelector('[data-action="voice-listen"]')?.getAttribute('aria-pressed') ?? null,
  errors: [...document.querySelectorAll('.voice-console .command-error')].map((e) => e.textContent),
})"""


async def section_recorder(browser, base):
    s = "recorder"
    ctx, page = await new_page(browser, init_script=RECORDER_INIT, permissions=["geolocation", "microphone"])
    audio_posts = []

    def on_request(r):
        if r.method == "POST" and r.url.endswith("/api/v3/voice/process"):
            body = json.loads(r.post_data or "{}")
            if (body.get("input") or {}).get("kind") == "audio":
                audio_posts.append(body["input"].get("content_type"))
    page.on("request", on_request)
    await onboard(page, base)
    await open_console(page)
    listen = page.locator('[data-action="voice-listen"]')
    state = lambda: page.evaluate(REC_STATE)

    # a. normal record -> stop -> one audio request of an allow-listed type, mic released
    await listen.click()
    await page.wait_for_function("document.querySelector('[data-action=\"voice-listen\"]')?.getAttribute('aria-pressed') === 'true'", timeout=5000)
    await settle(page, 1000)
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/voice/process"), timeout=15000) as info:
        await listen.click()
    resp = await info.value
    await page.wait_for_function("!document.querySelector('#command-input')?.disabled", timeout=15000)
    st = await state()
    rec(s, "a. record/stop submits one webm/opus request, 200, mic released",
        audio_posts == ["audio/webm;codecs=opus"] and resp.status == 200 and st["live"] == 0 and st["pressed"] == "false",
        f"posts={audio_posts} status={resp.status} {st}")

    # b. cancel while microphone access is still pending
    n_rec, n_posts = len(st["recorders"]), len(audio_posts)
    await page.evaluate("window.__gumDelays.push(1500)")
    await listen.click()
    await settle(page, 200)
    await page.click('[data-action="voice-close"]')
    await settle(page, 2200)
    st = await state()
    rec(s, "b. cancel before getUserMedia resolves: stream stopped, no recorder, nothing sent",
        st["live"] == 0 and len(st["recorders"]) == n_rec and len(audio_posts) == n_posts, st)
    await open_console(page)

    # c. cancel then immediately restart: the old recorder's late onstop must not submit or touch the new one
    await listen.click()
    await page.wait_for_function("document.querySelector('[data-action=\"voice-listen\"]')?.getAttribute('aria-pressed') === 'true'", timeout=5000)
    await settle(page, 800)
    n_posts = len(audio_posts)
    await page.evaluate("""() => {
      document.querySelector('[data-action="voice-close"]').click();
      document.querySelector('[data-action="voice-open"]').click();
      document.querySelector('[data-action="voice-listen"]').click();
    }""")
    await settle(page, 1500)
    st = await state()
    rec(s, "c. quick cancel+restart: old audio not sent, new recording active with one live track",
        len(audio_posts) == n_posts and st["pressed"] == "true" and st["live"] == 1 and st["recorders"][-1] == "recording", f"posts={audio_posts[n_posts:]} {st}")
    async with page.expect_response(lambda r: r.url.endswith("/api/v3/voice/process"), timeout=15000):
        await listen.click()
    await page.wait_for_function("!document.querySelector('#command-input')?.disabled", timeout=15000)
    st = await state()
    rec(s, "c. stopping the new recording submits exactly one request", len(audio_posts) == n_posts + 1 and st["live"] == 0, f"posts={audio_posts[n_posts:]} {st}")

    # e. language switch while recording
    n_posts = len(audio_posts)
    await listen.click()
    await page.wait_for_function("document.querySelector('[data-action=\"voice-listen\"]')?.getAttribute('aria-pressed') === 'true'", timeout=5000)
    await settle(page, 600)
    await page.click('[data-language="HI"]')
    await settle(page, 1500)
    st = await state()
    rec(s, "e. language switch while recording discards it and releases the mic",
        len(audio_posts) == n_posts and st["live"] == 0 and st["pressed"] == "false", st)
    await page.click('[data-language="EN"]')
    await open_console(page)

    # f. recorder error event
    await listen.click()
    await page.wait_for_function("document.querySelector('[data-action=\"voice-listen\"]')?.getAttribute('aria-pressed') === 'true'", timeout=5000)
    await settle(page, 500)
    await page.evaluate("window.__recorders.at(-1).dispatchEvent(new Event('error'))")
    await settle(page, 1500)
    st = await state()
    rec(s, "f. recorder error: mic released, nothing sent, citizen told the mic stopped",
        len(audio_posts) == n_posts and st["live"] == 0 and st["pressed"] == "false" and EN_MIC_STOPPED in st["errors"], st)

    async def clear_message():  # a language switch clears the console error line
        await page.click('[data-language="HI"]')
        await page.click('[data-language="EN"]')
        await open_console(page)

    # g. start() failure
    await clear_message()
    await page.evaluate("window.__failStart = true")
    await listen.click()
    await settle(page, 800)
    await page.evaluate("window.__failStart = false")
    st = await state()
    rec(s, "g. start() failure: mic released, not listening, message shown",
        st["live"] == 0 and st["pressed"] == "false" and EN_MIC_STOPPED in st["errors"], st)

    # h. no backend-accepted recorder MIME type
    await clear_message()
    gum_before = st["gum"]
    await page.evaluate("window.__noMime = true")
    await listen.click()
    await settle(page, 800)
    await page.evaluate("window.__noMime = false")
    st = await state()
    rec(s, "h. unsupported MIME: microphone never requested, message shown",
        st["gum"] == gum_before and st["pressed"] == "false" and EN_MIC_STOPPED in st["errors"], st)

    # i. permission denied (getUserMedia rejects)
    await clear_message()
    await page.evaluate("window.__gumReject = true")
    await listen.click()
    await settle(page, 800)
    await page.evaluate("window.__gumReject = false")
    st = await state()
    rec(s, "i. permission denied: not listening, message shown", st["live"] == 0 and st["pressed"] == "false" and EN_MIC_STOPPED in st["errors"], st)
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- audio

AUDIO_INIT = """(() => {
  const Real = window.Audio; window.__audios = [];
  function Instrumented(src) { const a = new Real(src); window.__audios.push(a); return a; }
  Instrumented.prototype = Real.prototype; window.Audio = Instrumented;
})();"""
AUDIO_STATE = "() => window.__audios.map((a) => ({ paused: a.paused, t: Math.round(a.currentTime * 10) / 10, ended: a.ended }))"


async def section_audio(browser, base):
    """Launched with autoplay allowed: valid autoplay must play; invalidation must stop it."""
    s = "audio"
    ctx, page = await new_page(browser, init_script=AUDIO_INIT)
    await onboard(page, base)
    await page.route("**/api/v3/voice/process", long_audio_route)
    audios = lambda: page.evaluate(AUDIO_STATE)

    await submit(page, "where can I go")
    await settle(page, 900)
    a = await audios()
    rec(s, "valid autoplay starts and keeps playing (claiming it does not stop it)", len(a) == 1 and not a[0]["paused"] and a[0]["t"] > 0.3, a)

    await submit(page, "where can I go")
    await settle(page, 900)
    a = await audios()
    rec(s, "a new request stops the earlier clip; the new clip plays", len(a) == 2 and a[0]["paused"] and not a[1]["paused"] and a[1]["t"] > 0.3, a)

    await page.click('[data-language="HI"]')
    await settle(page)
    a = await audios()
    rec(s, "language switch stops the playing clip", a[-1]["paused"] and not a[-1]["ended"], a)

    await page.locator('[data-action="listen"]').first.click()
    await settle(page)
    a2 = await audios()
    modal = await page.locator("dialog.modal[open]").count()
    rec(s, "replay after language switch is refused (no new clip, unavailable panel)", len(a2) == len(a) and modal == 1, f"clips={len(a2)} modal={modal}")
    await page.locator('[data-action="audio-close"]').click()

    await submit(page, "where can I go")
    await settle(page, 900)
    a = await audios()
    rec(s, "fresh request in the new language plays", not a[-1]["paused"], a[-1])

    await page.locator('[data-action="listen"]').first.click()
    await settle(page, 900)
    b = await audios()
    rec(s, "tap replay of current audio plays once and stops the previous clip", len(b) == len(a) + 1 and b[-2]["paused"] and not b[-1]["paused"], b[-2:])

    await ctx.set_offline(True)
    await settle(page, 500)
    b = await audios()
    rec(s, "going offline stops the playing clip", b[-1]["paused"], b[-1])
    await ctx.set_offline(False)
    await page.unroute("**/api/v3/voice/process")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


async def section_audio_denied(browser, base):
    """Launched with --autoplay-policy=user-gesture-required and a response delayed past the
    click's transient activation: the browser itself must deny autoplay; tap then plays."""
    s = "audio-denied"
    ctx, page = await new_page(browser, init_script=AUDIO_INIT)
    await onboard(page, base)

    async def slow_long(route):
        await asyncio.sleep(6)
        await long_audio_route(route)
    await page.route("**/api/v3/voice/process", slow_long)
    await submit(page, "where can I go")
    await settle(page, 600)
    a = await page.evaluate(AUDIO_STATE)
    st = await console_state(page)
    rec(s, "browser denied autoplay: clip created but never played, no error shown", len(a) == 1 and a[0]["paused"] and a[0]["t"] == 0 and not st["errors"], f"{a} errors={st['errors']}")
    await page.unroute("**/api/v3/voice/process")
    await page.locator('[data-action="listen"]').first.click()
    await settle(page, 900)
    a = await page.evaluate(AUDIO_STATE)
    rec(s, "tap-to-play replays the approved audio under a user gesture", len(a) == 2 and not a[1]["paused"] and a[1]["t"] > 0.3, a)
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- dialog

FOCUS = "() => { const e = document.activeElement; return e ? (e.getAttribute('data-action') || e.id || e.tagName) : null; }"
OPEN_DIALOGS = "() => document.querySelectorAll('dialog.modal[open]').length"


async def section_dialog(browser, base):
    s = "dialog"
    ctx, page = await new_page(browser)
    await onboard(page, base)
    ok_cycles = 0
    for i in range(4):
        await page.locator('[data-action="details"]').focus()
        await page.keyboard.press("Enter")
        await settle(page, 150)
        opened = await page.evaluate(OPEN_DIALOGS) == 1 and await page.evaluate(FOCUS) == "details-close"
        # unrelated re-render while open (layer panel toggle replaces the whole DOM)
        await page.locator('[data-action="toggle-layers"]').click()
        await page.locator('[data-action="toggle-layers"]').click()
        await page.locator('[data-action="details-close"]').focus()
        await page.keyboard.press("Escape")
        await settle(page, 150)
        closed = await page.evaluate(OPEN_DIALOGS) == 0 and await page.evaluate(FOCUS) == "details"
        ok_cycles += opened and closed
    rec(s, "4 open/re-render/Escape cycles: focus moves into the dialog and returns to its opener", ok_cycles == 4, f"ok={ok_cycles}/4")

    await page.locator('[data-action="voice-open"]').nth(1).focus()
    await page.keyboard.press("Enter")
    await settle(page, 150)
    await page.locator('[data-action="assist-open"]').focus()
    await page.keyboard.press("Enter")
    await settle(page, 150)
    await page.keyboard.press("Escape")
    await settle(page, 150)
    one = (await page.evaluate(OPEN_DIALOGS), await page.locator(".voice-console:not([hidden])").count(), await page.evaluate(FOCUS))
    rec(s, "one Escape closes only the topmost overlay (no duplicate global handlers)", one == (0, 1, "assist-open"), one)
    await page.keyboard.press("Escape")
    await settle(page, 150)
    idx = await page.evaluate("() => [...document.querySelectorAll('[data-action=\"voice-open\"]')].indexOf(document.activeElement)")
    two = (await page.locator(".voice-console:not([hidden])").count(), idx)
    rec(s, "second Escape closes the voice console; focus returns to the same opener instance", two == (0, 1), two)

    await page.evaluate("() => window.scrollTo(0, 0)")
    await page.locator('[data-action="isl"]').click()
    await page.locator('[data-action="isl-close"]').click()
    await settle(page, 150)
    rec(s, "close button (pointer) also returns focus to the opener", await page.evaluate(FOCUS) == "isl")
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- map

MAP_STATE = """() => { const m = window.map; if (!m) return null; return {
  pitch: Math.round(m.getPitch()), terrain: !!m.getTerrain(),
  pressed: document.querySelector('[data-action="toggle-3d"]')?.getAttribute('aria-pressed'),
  hazard: m.getLayer('hazard-fill') ? m.getLayoutProperty('hazard-fill', 'visibility') ?? 'visible' : 'missing',
}; }"""


async def section_map(browser, base):
    s = "map"
    ctx, page = await new_page(browser)
    await onboard(page, base, query="?sthira-test-hooks=1")
    await page.wait_for_selector(".map-loading[hidden]", state="attached", timeout=30000)
    rendered = await page.evaluate("""() => { const m = window.map; const c = m.getCanvas();
      return { w: c.width, h: c.height, route: m.queryRenderedFeatures({ layers: ['approved-route'] }).length }; }""")
    rec(s, "map canvas rendered with approved-route features", rendered["w"] > 0 and rendered["route"] > 0, rendered)
    ms = lambda: page.evaluate(MAP_STATE)
    for i in range(2):
        await page.locator('[data-action="toggle-3d"]').click()
        await settle(page, 1300)
        a = await ms()
        await page.locator('[data-action="recenter"]').click()
        await settle(page, 1300)
        b = await ms()
        rec(s, f"cycle {i + 1}: 3D control -> pitch 65 + terrain + pressed; recenter -> flat, no terrain, not pressed",
            (a["pitch"], a["terrain"], a["pressed"]) == (65, True, "true") and (b["pitch"], b["terrain"], b["pressed"]) == (0, False, "false"), f"{a} {b}")
    await page.locator('[data-action="toggle-3d"]').click()
    await settle(page, 1300)
    await page.route("**/api/v3/voice/process", rewrite_actions([{"type": "RECENTER"}]))
    await submit(page, "recenter")
    await settle(page, 1300)
    c = await ms()
    rec(s, "voice RECENTER while tilted leaves pitch, terrain and 3D control in agreement", (c["pitch"], c["terrain"], c["pressed"]) == (0, False, "false"), c)
    await page.unroute("**/api/v3/voice/process")
    await page.locator('[data-action="voice-close"]').click()
    await page.locator('[data-action="toggle-layers"]').click()
    toggles = []
    for _ in range(2):
        await page.locator('[data-action="toggle-red-zones"]').click()
        await settle(page, 600)
        m = await ms()
        drawn = await page.evaluate("() => window.map.queryRenderedFeatures({ layers: ['hazard-fill'] }).length")
        pressed = await page.locator('[data-action="toggle-red-zones"]').get_attribute("aria-pressed")
        toggles.append((m["hazard"], pressed, drawn > 0))
    rec(s, "red-zone control shows then hides the hazard layer (default hidden); drawn features and aria-pressed agree",
        toggles == [("visible", "true", True), ("none", "false", False)], toggles)
    rec(s, "no uncaught page errors", not page.errors, "; ".join(page.errors))
    await ctx.close()


# ---------------------------------------------------------------- layout

# Each control is scrolled into view and hit-tested at its centre; returns the failures.
HIT_TEST = """() => {
  const sel = ['[data-action="route"]', '.quick-actions [data-action="directions"]', '.quick-actions [data-action="listen"]',
               '.quick-actions [data-action="isl"]', '.quick-actions [data-action="voice-open"]', '[data-testid="call-112"]', '.map-disclaimer a'];
  const bad = [];
  for (const q of sel) {
    const el = document.querySelector(q);
    if (!el) { bad.push(q + ' missing'); continue; }
    el.scrollIntoView({ block: 'center' });
    const r = el.getBoundingClientRect();
    const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    if (!r.width || !r.height || !el.contains(hit)) bad.push(q + ' -> ' + (hit ? hit.className || hit.tagName : 'nothing'));
  }
  // The disclaimer box itself must let map gestures through (pointer-events: none), its link must not.
  const d = document.querySelector('.map-disclaimer'); const dr = d.getBoundingClientRect();
  const under = document.elementFromPoint(dr.left + 2, dr.top + dr.height / 2);
  if (d.contains(under)) bad.push('map-disclaimer text intercepts the map');
  window.scrollTo(0, 0);
  return bad;
}"""


async def section_layout(browser, base):
    s = "layout"
    for lang in ("EN", "HI", "ML"):
        for w, h in ((375, 812), (1024, 768), (1440, 900)):
            ctx, page = await new_page(browser, viewport={"width": w, "height": h})
            await onboard(page, base, lang=lang)
            await settle(page, 500)
            sw = await page.evaluate("document.documentElement.scrollWidth")
            covered = await page.evaluate(HIT_TEST)
            rec(s, f"{lang} {w}px: route, quick actions, 112 and voice entry are visible and not covered", not covered, covered)
            await page.locator('[data-action="details"]').click()
            box = await page.locator("dialog.modal[open]").bounding_box()
            close = await page.locator('[data-action="details-close"]').bounding_box()
            fits = box and box["x"] >= 0 and box["x"] + box["width"] <= w + 0.5 and box["height"] <= h
            close_ok = close and close["y"] >= 0 and close["x"] + close["width"] <= w + 0.5
            rec(s, f"{lang} {w}px: no horizontal overflow; details dialog fits with reachable close",
                sw <= w and fits and close_ok and not page.errors, f"scrollWidth={sw} dialog={box} close={close}")
            await ctx.close()


SECTIONS = {
    "core": (section_core, []),
    "outage": (section_outage, []),
    "language": (section_language, []),
    "reservation": (section_reservation, []),
    "recorder": (section_recorder, ["--use-fake-device-for-media-stream", "--use-fake-ui-for-media-stream"]),
    "audio": (section_audio, ["--autoplay-policy=no-user-gesture-required"]),
    "audio-denied": (section_audio_denied, ["--autoplay-policy=user-gesture-required"]),
    "dialog": (section_dialog, []),
    "map": (section_map, []),
    "layout": (section_layout, []),
}


async def main(base, sections, out):
    async with async_playwright() as p:
        for name in sections:
            fn, args = SECTIONS[name]
            browser = await p.chromium.launch(headless=True, args=args)
            try:
                await fn(browser, base)
            except Exception as e:  # a crashed section is a failure, never a silent pass
                rec(name, "section completed", False, f"{type(e).__name__}: {str(e)[-400:]}")
            await browser.close()
    if out:
        with open(out, "w") as f:
            json.dump(RESULTS, f, ensure_ascii=False, indent=1)
    fails = [r for r in RESULTS if r["status"] != "PASS"]
    print(f"{len(RESULTS) - len(fails)} PASS, {len(fails)} FAIL")
    return 1 if fails or not RESULTS else 0


if __name__ == "__main__":
    args = sys.argv[1:]
    out = None
    if "--json" in args:
        i = args.index("--json"); out = args[i + 1]; del args[i:i + 2]
    unknown = [a for a in args[1:] if a not in SECTIONS]
    if len(args) < 2 or unknown:
        print(f"usage: BASE_URL SECTION... ({' '.join(SECTIONS)}); unknown={unknown}")
        sys.exit(2)
    sys.exit(asyncio.run(main(args[0], args[1:], out)))
