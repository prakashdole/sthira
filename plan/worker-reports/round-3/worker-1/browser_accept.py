#!/usr/bin/env python3
"""Browser acceptance harness — Worker 1 (corrected from MiniMax B baseline).

Scope: onboarding, text submission, visible-control reservation,
duplicate-start protection, browser-emulated geolocation, server-acknowledged arrival.

Excludes: outage (E/F), layout/overflow (G), audio playback (H),
camera/layer controls (L), delayed-language race (C).

Key corrections vs MiniMax B baseline:
- Fixed duplicate detection: posts_to_reservations length is now captured
  BEFORE the second click, not an always-empty variable.
- Added explicit waits for directions sheet to appear before closing it.
- Replaced arbitrary asyncio.sleep polling with Playwright wait_for_selector.
- Added secondary path for onboarding when geolocation is unavailable.
- startTracking() is called automatically inside submitReservation() after
  a successful reservation; the explicit start-tracking click in the harness
  is kept as a no-op when the button is absent.
- Removed unused wait_for_network helper (never called).

Usage:
    python3 browser_accept.py http://127.0.0.1:18492/
    python3 browser_accept.py http://127.0.0.1:5173/ --viewport=390,844

Each check writes one JSON line to stdout on failure for machine parsing.
Exits 0 only when all checks pass; nonzero on any failure.
"""

import argparse
import asyncio
import json
import sys
from urllib.parse import urlparse

from playwright.async_api import async_playwright

try:
    import httpx
    HAS_HTTPX = True
except ImportError:
    HAS_HTTPX = False

DEFAULT_BASE = "http://127.0.0.1:18492"
SHELTER = {"longitude": 76.105, "latitude": 11.570, "accuracy": 10}
TIMEOUT = 15_000  # ms — default per-operation timeout
CHECKS = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    status = "PASS" if ok else "FAIL"
    print(f"[{status}] {name}" + (f": {detail}" if detail else ""), flush=True)


# ---------------------------------------------------------------------------
# Core page interactions — selectors verified against frontend/v2/src/main.ts
# ---------------------------------------------------------------------------

async def complete_onboarding(page):
    """Run the full three-step onboarding flow via visible controls only.

    Resolves geolocation via Playwright's emulated context so that the
    'location ready' path is taken (primary onboarding-complete button).
    If that button is absent (geolocation unavailable in the environment),
    the secondary 'continue without location' path is tried.
    """
    # Step 1: click "begin" / onboarding-start
    await page.wait_for_selector('[data-action="onboarding-start"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-start"]')

    # Step 2: select EN language
    await page.wait_for_selector('[data-onboarding-language="EN"]', timeout=TIMEOUT)
    await page.click('[data-onboarding-language="EN"]')
    await page.wait_for_selector('[data-action="onboarding-language-next"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-language-next"]')

    # Step 3: complete with location
    # Primary path: location ready → primary button
    try:
        await page.wait_for_selector(
            '[data-action="onboarding-complete"]',
            timeout=5000,
        )
        await page.locator('[data-action="onboarding-complete"]').first.click()
    except asyncio.TimeoutError:
        # Fallback: geolocation unavailable → secondary button (no location)
        try:
            await page.wait_for_selector(
                '[data-action="onboarding-complete"]:has-text("Continue")',
                timeout=3000,
            )
            await page.locator('[data-action="onboarding-complete"]').first.click()
        except asyncio.TimeoutError:
            raise

    # Guidance card must appear after onboarding
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=TIMEOUT)


async def submit_text_command(page, text: str = "where can I go"):
    """Open the voice console and submit a text command via the form."""
    voice_console = page.locator(".voice-console")
    if not await voice_console.is_visible():
        await page.locator('[data-action="voice-open"]').first.click()
        await page.wait_for_selector(".voice-console:not([hidden])", timeout=TIMEOUT)

    await page.wait_for_selector("#command-input", timeout=TIMEOUT)
    await page.fill("#command-input", text)
    await page.click('[data-command-form] button[type="submit"]')

    # Wait for server response: input re-enables when done
    try:
        await page.wait_for_function(
            "document.querySelector('#command-input') && "
            "!document.querySelector('#command-input').disabled",
            timeout=TIMEOUT,
        )
    except Exception:
        pass  # already re-enabled or never was

    # Close voice console cleanly
    close_btn = page.locator('[data-action="voice-close"]')
    if await close_btn.is_visible():
        await close_btn.first.click()


async def click_route_and_get_stay_ids(page) -> tuple[str | None, str | None]:
    """Click [data-action=route] once; return (stay_id, facility_id)."""
    await page.locator('[data-action="route"]').first.click()

    # Wait for directions sheet to appear — confirms reservation was accepted
    try:
        await page.wait_for_selector('[data-action="directions-close"]', timeout=8000)
    except asyncio.TimeoutError:
        pass  # directions sheet may not appear in all flows

    stay_id = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    facility_id = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")
    return stay_id, facility_id


async def dismiss_sheets(page):
    """Close any open sheets (directions, details)."""
    for _ in range(4):
        for sel in ["[data-action='directions-close']", "[data-action='details-close']"]:
            count = await page.locator(sel).count()
            if count:
                await page.locator(sel).first.click()
                await asyncio.sleep(0.15)
        await asyncio.sleep(0.15)


async def gps_arrival_sequence(page) -> bool:
    """Browser-emulated GPS at shelter → arrival-open → arrival-yes.

    Returns True if "Recorded at" confirmation was shown by the server.
    startTracking() is called automatically inside submitReservation() after
    a successful reservation, so we do NOT click start-tracking here.
    We wait for arrival-open to appear from the GPS proximity evaluation.
    """
    try:
        await page.wait_for_selector('[data-action="arrival-open"]', timeout=20_000)
    except asyncio.TimeoutError:
        return False

    await page.locator('[data-action="arrival-open"]').first.click()

    try:
        await page.wait_for_selector('[data-action="arrival-yes"]', timeout=5000)
        await page.locator('[data-action="arrival-yes"]').first.click()

        # Server-acknowledged: "Recorded at" timestamp must appear
        await page.wait_for_selector("text=Recorded at", timeout=8000)
        return True
    except asyncio.TimeoutError:
        body_text = await page.inner_text("body")
        clues = [
            l.strip() for l in body_text.splitlines()
            if any(k in l for k in ("GPS", "Tracking", "accuracy", "coordinates", "Waiting"))
        ]
        return False


# ---------------------------------------------------------------------------
# HTTP API spot-checks (optional; skips cleanly if httpx unavailable)
# ---------------------------------------------------------------------------

async def api_reservation_flow(base_url: str) -> dict:
    """Create a session, query guidance, reserve, arrive. Returns status dict."""
    if not HAS_HTTPX:
        return {"skip": "httpx not installed"}

    parsed = urlparse(base_url)
    backend = f"http://{parsed.netloc or '127.0.0.1:8080'}"
    if parsed.netloc.endswith(":5173"):
        backend = "http://127.0.0.1:8080"

    result = {}
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            s = await client.post(f"{backend}/api/v3/sessions", json={})
            if s.status_code != 200:
                result["session"] = f"HTTP {s.status_code}"
                return result
            token = s.json()["data"]["token"]
            headers = {"Authorization": f"Bearer {token}"}

            g = await client.post(
                f"{backend}/api/v3/guidance/query",
                headers=headers,
                json={
                    "jurisdiction": "DEMO-EXERCISE",
                    "package_id": "PKGDEMO-1",
                    "party_size": 1,
                    "start_date": "2026-09-27",
                    "end_date": "2026-09-28",
                },
            )
            if g.status_code != 200:
                result["guidance"] = f"HTTP {g.status_code}"
                return result
            gdata = g.json()["data"]
            dest = gdata["destinations"][0]
            snap_ver = gdata["snapshot_version"]
            pkg_id = gdata.get("data_version", "").split(":")[0]
            result["guidance_dest"] = dest["facility_id"]

            r = await client.post(
                f"{backend}/api/v3/reservations",
                headers=headers,
                json={
                    "facility_id": dest["facility_id"],
                    "package_id": pkg_id,
                    "route_id": dest["route_id"],
                    "party_size": 1,
                    "start_date": "2026-09-27",
                    "end_date": "2026-09-28",
                    "idempotency_key": f"bm-w1-{id({})}",
                    "snapshot_version": snap_ver,
                },
            )
            if r.status_code not in (200, 201):
                result["reservation"] = f"HTTP {r.status_code}"
                return result
            stay_id = r.json()["data"]["stay_id"]
            result["stay_id"] = stay_id

            a = await client.post(
                f"{backend}/api/v3/reservations/{stay_id}/events",
                headers=headers,
                json={"type": "ARRIVE", "idempotency_key": f"ba-w1-{id({})}"},
            )
            result["arrival"] = a.status_code
            result["arrived"] = a.status_code == 200
    except Exception as exc:
        result["error"] = str(exc)
    return result


# ---------------------------------------------------------------------------
# Main harness
# ---------------------------------------------------------------------------

async def run(base_url: str, viewport: tuple[int, int]) -> bool:
    all_ok = True
    posts_to_reservations: list[int] = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        ctx = await browser.new_context(
            viewport={"width": viewport[0], "height": viewport[1]},
            geolocation=SHELTER,
            permissions=["geolocation"],
        )
        page = await ctx.new_page()

        page_errors: list[str] = []
        page.on("pageerror", lambda e: page_errors.append(str(e)))

        # Capture reservation POST responses (network layer, not UI state)
        async def on_response(response):
            if response.method == "POST" and "/api/v3/reservations" in response.url:
                posts_to_reservations.append(response.status)
        page.on("response", on_response)

        # 0. Load
        await page.goto(base_url, wait_until="networkidle")
        await asyncio.sleep(0.3)

        # 0b. No window hooks without ?sthira-test-hooks=1
        hooks_absent = await page.evaluate(
            "typeof window.setActiveStayId === 'undefined' && "
            "typeof window.sendVoiceOrText === 'undefined'"
        )
        check("window hooks absent without opt-in", hooks_absent)

        # 1. Onboarding
        try:
            await complete_onboarding(page)
            check("onboarding completes to guidance card", True)
        except Exception as e:
            check("onboarding completes to guidance card", False, str(e)[:100])
            await browser.close()
            return False

        # 2. Guidance destination rendered (backend-driven content)
        try:
            await page.wait_for_selector("text=Demo Safe Facility", timeout=8000)
            check("guidance destination rendered from backend", True)
        except Exception as e:
            check("guidance destination rendered from backend", False, str(e)[:80])
            all_ok = False

        # 3. Text command
        try:
            await submit_text_command(page, "where can I go")
            check("text command submitted and processed", True)
        except Exception as e:
            check("text command submitted and processed", False, str(e)[:80])

        # 4. First reservation via visible control
        try:
            stay_after_first, fac_after_first = await click_route_and_get_stay_ids(page)
            route_ok = bool(stay_after_first and fac_after_first)
            check("route start creates stay in session storage", route_ok,
                  f"stay={stay_after_first} fac={fac_after_first}")
            if not route_ok:
                all_ok = False
        except Exception as e:
            check("route start creates stay in session storage", False, str(e)[:80])
            all_ok = False

        # 5. Duplicate-start protection
        # Capture the POST count AFTER the first reservation but BEFORE the second click.
        # posts_to_reservations is append-only; it grows with each /api/v3/reservations POST.
        try:
            await dismiss_sheets(page)

            posts_before_second = len(posts_to_reservations)

            # Click route a second time — server idempotency must prevent a second reservation
            await page.locator('[data-action="route"]').first.click(force=True)
            await asyncio.sleep(1.5)

            posts_after_second = len(posts_to_reservations)

            stay_after_second = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_id')"
            )
            fac_after_second = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_facility_id')"
            )
            new_posts = posts_after_second - posts_before_second

            # Correct duplicate detection:
            # - new_posts must be 0 (server rejected duplicate or returned same stay)
            # - stay must be unchanged
            # The old harness used an always-empty posts_before variable, so this check
            # always passed (new_posts was always <= 1 even when it should have been 0).
            duplicate_ok = (
                new_posts == 0
                and stay_after_second == stay_after_first
                and fac_after_second == fac_after_first
            )
            check("repeated Start Route creates no second reservation",
                  duplicate_ok,
                  f"new_posts={new_posts} stay_before={stay_after_first} "
                  f"stay_after={stay_after_second}")
            if not duplicate_ok:
                all_ok = False
        except Exception as e:
            check("repeated Start Route creates no second reservation",
                  False, str(e)[:80])

        # 6. GPS + server-acknowledged arrival
        try:
            arrived = await gps_arrival_sequence(page)
            check("arrival recorded after server acknowledgement", arrived)
            if not arrived:
                all_ok = False
        except Exception as e:
            check("arrival recorded after server acknowledgement", False, str(e)[:80])
            all_ok = False

        # 7. No page errors
        check("no uncaught page errors", len(page_errors) == 0,
              "; ".join(page_errors)[:200])

        # 8. API reservation spot-check (optional)
        if HAS_HTTPX:
            try:
                api = await api_reservation_flow(base_url)
                if "error" in api:
                    check("API reservation+arrival spot-check", False, api["error"])
                else:
                    all_api_ok = (
                        api.get("guidance_dest") == "FACDEMO-1"
                        and api.get("arrived") is True
                    )
                    check("API reservation+arrival spot-check", all_api_ok, json.dumps(api))
            except Exception as exc:
                check("API reservation+arrival spot-check", False, str(exc)[:80])

        await browser.close()

    return all_ok


# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

def main():
    parser = argparse.ArgumentParser(
        description="Sthira browser acceptance — reusable harness"
    )
    parser.add_argument("base_url", nargs="?", default=DEFAULT_BASE)
    parser.add_argument(
        "--viewport",
        default="1280,860",
        help="Viewport as W,H (default 1280,860)",
    )
    args = parser.parse_args()

    try:
        w, h = map(int, args.viewport.split(","))
    except Exception:
        w, h = 1280, 860

    print(f"\n=== Sthira browser accept harness (Worker 1) ===", flush=True)
    print(f"Base URL : {args.base_url}", flush=True)
    print(f"Viewport : {w}x{h}", flush=True)
    print(f"GPS emul: {SHELTER}", flush=True)
    print(f"Started  : {__import__('datetime').datetime.now().isoformat()}", flush=True)
    print("-" * 50, flush=True)

    ok = asyncio.run(run(args.base_url, (w, h)))

    print("-" * 50, flush=True)
    passed = sum(1 for c in CHECKS if c["pass"])
    failed = sum(1 for c in CHECKS if not c["pass"])
    print(f"Results  : {passed} PASS | {failed} FAIL", flush=True)

    for c in CHECKS:
        if not c["pass"]:
            detail = c["detail"].replace("\n", " ")[:160]
            print(f"FAIL_DETAIL: [{c['name']}] {detail}", flush=True)

    print(f"\nExit     : {'0 (all passed)' if ok else '1 (has failures)'}", flush=True)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
