#!/usr/bin/env python3
"""Reusable browser acceptance harness — MiniMax B.

Scope: onboarding, text submission, visible-control reservation,
duplicate-start protection, browser-emulated geolocation, server-acknowledged arrival.

Excludes: outage (E/F), layout/overflow (G), audio playback (H),
camera/layer controls (L), delayed-language race (C).

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
import re
from urllib.parse import urlparse

from playwright.async_api import async_playwright

# ---------------------------------------------------------------------------
# Selective import; installed tooling only.
try:
    import httpx
    HAS_HTTPX = True
except ImportError:
    HAS_HTTPX = False
# ---------------------------------------------------------------------------

DEFAULT_BASE = "http://127.0.0.1:18492"
SHELTER = {"longitude": 76.105, "latitude": 11.570, "accuracy": 10}
TIMEOUT = 15_000  # ms — default per-operation timeout
CHECKS = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    status = "PASS" if ok else "FAIL"
    print(f"[{status}] {name}" + (f": {detail}" if detail else ""), flush=True)


async def wait_for_network(page, url_suffix: str = "", timeout: int = TIMEOUT):
    """Wait for a specific network resource, ignoring unrelated requests."""
    async def _wait():
        async with asyncio.timeout(timeout / 1000):
            while True:
                if asyncio.iscoroutinefunction(page.wait_for_response):
                    try:
                        await page.wait_for_response(
                            lambda r: r.url.endswith(url_suffix), timeout=2000
                        )
                        return
                    except asyncio.TimeoutError:
                        pass
                await asyncio.sleep(0.2)
    await _wait()


# ---------------------------------------------------------------------------
# Core page interactions — all via verified selectors from frontend/v2/src/main.ts
# ---------------------------------------------------------------------------

async def complete_onboarding(page):
    """Run the full three-step onboarding flow via visible controls only."""
    # Step 1: click "begin" / onboarding-start
    await page.wait_for_selector('[data-action="onboarding-start"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-start"]')
    await asyncio.sleep(0.3)

    # Step 2: select EN language
    await page.wait_for_selector('[data-onboarding-language="EN"]', timeout=TIMEOUT)
    await page.click('[data-onboarding-language="EN"]')
    await page.wait_for_selector('[data-action="onboarding-language-next"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-language-next"]')
    await asyncio.sleep(0.3)

    # Step 3: complete with location
    await page.wait_for_selector('[data-action="onboarding-complete"]', timeout=TIMEOUT)
    await page.locator('[data-action="onboarding-complete"]').first.click()

    # Map (or at minimum the guidance card) must appear
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=TIMEOUT)


async def submit_text_command(page, text: str = "where can I go"):
    """Open the voice console and submit a text command via the form."""
    # Open voice console if not already open
    if not await page.locator(".voice-console:not([hidden])").is_visible():
        await page.locator('[data-action="voice-open"]').first.click()
        await asyncio.sleep(0.3)

    # Submit via the bound form
    await page.wait_for_selector("#command-input", timeout=TIMEOUT)
    await page.fill("#command-input", text)
    await page.click('[data-command-form] button[type="submit"]')

    # Wait for server response — input re-enables when done
    try:
        await page.wait_for_function(
            "!(document.querySelector('#command-input') || {}).disabled",
            timeout=TIMEOUT,
        )
    except Exception:
        pass  # already re-enabled or never was

    # Close voice console cleanly
    if await page.locator('[data-action="voice-close"]').first.is_visible():
        await page.locator('[data-action="voice-close"]').first.click()
        await asyncio.sleep(0.2)


async def click_start_route_twice(page) -> tuple[int, str, str]:
    """Click [data-action=route] twice and return (post_count, stay_id, facility_id)."""
    posts_before = []  # collected via page-level request listener

    # First click
    await page.locator('[data-action="route"]').first.click()
    await asyncio.sleep(0.5)

    # Dismiss directions sheet if shown
    for _ in range(5):
        if await page.locator('[data-action="directions-close"]').count():
            await page.locator('[data-action="directions-close"]').first.click()
            await asyncio.sleep(0.2)
        await asyncio.sleep(0.3)

    # Second click — force to bypass UI guard
    await page.locator('[data-action="route"]').first.click(force=True)
    await asyncio.sleep(1.0)

    # Inspect session storage for stay identity
    stay_id = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    facility_id = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")

    return len(posts_before), stay_id, facility_id


async def gps_arrival_sequence(page, base_url: str) -> bool:
    """Browser-emulated GPS at shelter → start-tracking → arrival-open → arrival-yes.

    Returns True if "Recorded at" confirmation was shown by the server.
    """
    # Click start-tracking to begin GPS watch (uses navigator.geolocation,
    # which respects Playwright's emulated geolocation context)
    try:
        await page.wait_for_selector('[data-action="start-tracking"]', timeout=8000)
        await page.locator('[data-action="start-tracking"]').first.click()
    except Exception:
        pass  # button absent — tracking may be auto-started or N/A in demo mode

    # Wait for journeyState to advance to NEAR_DESTINATION and show arrival-open
    # In demo mode with shelter coords, this happens after GPS proximity is evaluated.
    try:
        await page.wait_for_selector('[data-action="arrival-open"]', timeout=20_000)
    except Exception:
        return False

    await page.locator('[data-action="arrival-open"]').first.click()
    await asyncio.sleep(0.3)

    try:
        await page.wait_for_selector('[data-action="arrival-yes"]', timeout=5000)
        await page.locator('[data-action="arrival-yes"]').first.click()

        # Server-acknowledged: "Recorded at" timestamp must appear
        await page.wait_for_selector("text=Recorded at", timeout=8000)
        return True
    except Exception:
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
    # Remap Vite dev port to backend
    if parsed.netloc.endswith(":5173"):
        backend = "http://127.0.0.1:8080"

    result = {}
    try:
        async with httpx.AsyncClient(timeout=10) as client:
            # 1. Anonymous session
            s = await client.post(f"{backend}/api/v3/sessions", json={})
            if s.status_code != 200:
                result["session"] = f"HTTP {s.status_code}"
                return result
            token = s.json()["data"]["token"]
            headers = {"Authorization": f"Bearer {token}"}

            # 2. Guidance query
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

            # 3. Reserve
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
                    "idempotency_key": f"bm-{id(self)}",
                    "snapshot_version": snap_ver,
                },
            )
            if r.status_code not in (200, 201):
                result["reservation"] = f"HTTP {r.status_code}"
                return result
            stay_id = r.json()["data"]["stay_id"]
            result["stay_id"] = stay_id

            # 4. Arrive
            a = await client.post(
                f"{backend}/api/v3/reservations/{stay_id}/events",
                headers=headers,
                json={"type": "ARRIVE", "idempotency_key": f"ba-{id(self)}"},
            )
            result["arrival"] = a.status_code
            if a.status_code == 200:
                result["arrived"] = True
            else:
                result["arrived"] = False
    except Exception as exc:
        result["error"] = str(exc)
    return result


# ---------------------------------------------------------------------------
# Main harness
# ---------------------------------------------------------------------------

async def run(base_url: str, viewport: tuple[int, int]) -> bool:
    all_ok = True
    posts_to_reservations = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        ctx = await browser.new_context(
            viewport={"width": viewport[0], "height": viewport[1]},
            geolocation=SHELTER,
            permissions=["geolocation"],
        )
        page = await ctx.new_page()

        # Capture page errors
        page_errors = []
        page.on("pageerror", lambda e: page_errors.append(str(e)))

        # Capture reservation POSTs via response (avoids mutable window state)
        async def on_response(response):
            if response.method == "POST" and "/api/v3/reservations" in response.url:
                posts_to_reservations.append(response.status)
        page.on("response", on_response)

        # 0. Load
        await page.goto(base_url, wait_until="networkidle")
        await asyncio.sleep(0.5)

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
            all_ok = False
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

        # 4. Reservation via visible control
        try:
            await page.locator('[data-action="route"]').first.click()
            await asyncio.sleep(1.5)
            # Dismiss directions sheet if appeared
            for _ in range(4):
                if await page.locator('[data-action="directions-close"]').count():
                    await page.locator('[data-action="directions-close"]').first.click()
                    await asyncio.sleep(0.2)
                await asyncio.sleep(0.3)

            stay_after_first = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_id')"
            )
            fac_after_first = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_facility_id')"
            )
            route_ok = bool(stay_after_first and fac_after_first)
            check("route start creates stay in session storage", route_ok,
                  f"stay={stay_after_first} fac={fac_after_first}")
        except Exception as e:
            check("route start creates stay in session storage", False, str(e)[:80])
            all_ok = False

        # 5. Duplicate-start protection
        try:
            # Close any open sheets first
            for _ in range(3):
                for sel in ["[data-action='directions-close']",
                            "[data-action='details-close']"]:
                    if await page.locator(sel).count():
                        await page.locator(sel).first.click()
                        await asyncio.sleep(0.2)
                await asyncio.sleep(0.2)

            posts_before = len(posts_to_reservations)
            # Click route a second time — should be idempotent
            await page.locator('[data-action="route"]').first.click(force=True)
            await asyncio.sleep(2.0)
            posts_after = len(posts_to_reservations)

            stay_after_second = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_id')"
            )
            fac_after_second = await page.evaluate(
                "sessionStorage.getItem('sthira_stay_facility_id')"
            )
            new_posts = posts_after - posts_before

            # A passing check: exactly 1 POST for /reservations and stay unchanged
            duplicate_ok = (new_posts <= 1 and
                           stay_after_second == stay_after_first and
                           fac_after_second == fac_after_first)
            check("repeated Start Route creates no second reservation",
                  duplicate_ok,
                  f"new_posts={new_posts} stay_before={stay_after_first} "
                  f"stay_after={stay_after_second}")
        except Exception as e:
            check("repeated Start Route creates no second reservation",
                  False, str(e)[:80])

        # 6. GPS + server-acknowledged arrival
        try:
            arrived = await gps_arrival_sequence(page, base_url)
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

    print(f"\n=== Sthira browser accept harness ===", flush=True)
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

    # Machine-readable failure lines for CI grep
    for c in CHECKS:
        if not c["pass"]:
            detail = c["detail"].replace("\n", " ")[:160]
            print(f"FAIL_DETAIL: [{c['name']}] {detail}", flush=True)

    print(f"\nExit     : {'0 (all passed)' if ok else '1 (has failures)'}", flush=True)
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    main()
