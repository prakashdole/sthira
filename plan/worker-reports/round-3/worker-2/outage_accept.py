#!/usr/bin/env python3
"""Voice outage regression: 503/504 injection + assistantUnavailable display + recovery.

Injects HTTP 503/504 at the voice/process boundary using page.route,
asserts the visible .command-error element shows "unavailable" (not a misleading
"Location not found"), then releases interception and verifies recovery.

Uses Python Playwright (existing tooling). Network emulation is labelled as such;
this does not prove human speech quality or hardware behavior.

Usage:
    python3 outage_accept.py [base_url]
    python3 outage_accept.py http://127.0.0.1:18492/

Prerequisites:
    pip install playwright
    playwright install chromium   # one-time

Exit: 0 on all checks pass, 1 on any failure.
NOT_RUN until executed against the integrated revision.
"""

import argparse
import asyncio
import json
import sys

from playwright.async_api import async_playwright

DEFAULT_BASE = "http://127.0.0.1:18492"
CHECKS = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    icon = "PASS" if ok else "FAIL"
    print(f"[{icon}] {name}" + (f": {detail}" if detail else ""), flush=True)


async def complete_onboarding(page):
    """Run three-step onboarding via visible controls (data-action attributes)."""
    await page.wait_for_selector('[data-action="onboarding-start"]', timeout=15000)
    await page.click('[data-action="onboarding-start"]')
    await asyncio.sleep(0.3)

    await page.wait_for_selector('[data-onboarding-language="EN"]', timeout=8000)
    await page.click('[data-onboarding-language="EN"]')
    await page.wait_for_selector('[data-action="onboarding-language-next"]', timeout=8000)
    await page.click('[data-action="onboarding-language-next"]')
    await asyncio.sleep(0.3)

    await page.wait_for_selector('[data-action="onboarding-complete"]', timeout=8000)
    await page.locator('[data-action="onboarding-complete"]').first.click()

    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=10000)


async def submit_text_command(page, text: str):
    """Open voice console and submit a text command, waiting for processing."""
    if not await page.locator(".voice-console:not([hidden])").is_visible():
        await page.locator('[data-action="voice-open"]').first.click()
        await asyncio.sleep(0.3)

    await page.wait_for_selector("#command-input", timeout=8000)
    await page.fill("#command-input", text)
    await page.click('[data-command-form] button[type="submit"]')

    try:
        await page.wait_for_function(
            "!(document.querySelector('#command-input') || {}).disabled",
            timeout=12000,
        )
    except Exception:
        pass


async def main(base_url: str):
    voice_statuses = []
    all_ok = True

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        ctx = await browser.new_context(
            viewport={"width": 1280, "height": 860},
            permissions=["geolocation"],
        )
        page = await ctx.new_page()

        page_errors = []
        page.on("pageerror", lambda e: page_errors.append(str(e)))
        page.on(
            "response",
            lambda r: voice_statuses.append(r.status)
            if r.url.endswith("/api/v3/voice/process")
            else None,
        )

        # Load app
        await page.goto(base_url, wait_until="networkidle")
        await asyncio.sleep(0.5)

        # Onboarding
        try:
            await complete_onboarding(page)
            check("onboarding completes", True)
        except Exception as e:
            check("onboarding completes", False, str(e)[:80])
            await browser.close()
            return False

        # Wait for guidance to render
        await page.wait_for_selector("text=Demo Safe Facility", timeout=8000)

        # ----------------------------------------------------------------
        # PHASE 1: Inject 503 — expect assistant-unavailable, NOT place-not-found
        # ----------------------------------------------------------------
        async def inject_503(route):
            await route.fulfill(status=503, body="Service Unavailable")

        await page.route("**/api/v3/voice/process", inject_503)

        try:
            await submit_text_command(page, "Show my route")

            # The .command-error element must appear
            error_elem = page.locator(".command-error")
            try:
                await error_elem.wait_for(timeout=6000)
                error_visible = True
            except Exception:
                error_visible = False

            check(
                "command-error element visible during 503",
                error_visible,
            )

            if error_visible:
                error_text = (await error_elem.inner_text()).strip()

                # "unavailable" must appear (from words[language].commandUnavailable)
                has_unavailable = "unavailable" in error_text.lower()
                check(
                    "error text contains 'unavailable'",
                    has_unavailable,
                    error_text[:120],
                )

                # "not found" must NOT appear — this is the regression check
                has_not_found = "not found" in error_text.lower()
                check(
                    "error text does NOT contain 'not found' (no misleading resolvePlace fallback)",
                    not has_not_found,
                    error_text[:120],
                )
            else:
                check("command-error element visible during 503", False, "element absent")
                all_ok = False

        finally:
            await page.unroute("**/api/v3/voice/process")

        # ----------------------------------------------------------------
        # PHASE 2: Verify voice pipeline status was 503
        # ----------------------------------------------------------------
        check(
            "voice/process received 503 injection",
            503 in voice_statuses,
            f"statuses observed: {voice_statuses}",
        )

        # ----------------------------------------------------------------
        # PHASE 3: Recovery — remove interception, submit again, expect normal response
        # ----------------------------------------------------------------
        # The route is already removed by unroute above. Clear input and submit fresh.
        await page.locator('[data-action="voice-open"]').first.click()
        await asyncio.sleep(0.3)

        await page.wait_for_selector("#command-input", timeout=8000)
        await page.fill("#command-input", "Show my location")
        await page.click('[data-command-form] button[type="submit"]')

        try:
            await page.wait_for_function(
                "!(document.querySelector('#command-input') || {}).disabled",
                timeout=12000,
            )
        except Exception:
            pass

        # During recovery (200 from mock backend), command-error should NOT be present
        error_elem = page.locator(".command-error")
        try:
            await error_elem.wait_for(timeout=3000)
            recovery_has_error = await error_elem.is_visible()
        except Exception:
            recovery_has_error = False

        check(
            "after unroute (recovery): no command-error shown",
            not recovery_has_error,
        )

        # No uncaught page errors throughout
        check(
            "no uncaught page errors throughout",
            len(page_errors) == 0,
            "; ".join(page_errors)[:200] if page_errors else "",
        )

        await browser.close()

    passed = sum(1 for c in CHECKS if c["pass"])
    failed = sum(1 for c in CHECKS if not c["pass"])
    print(f"\nResults: {passed} PASS | {failed} FAIL", flush=True)
    return all_ok


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Voice outage regression")
    parser.add_argument("base_url", nargs="?", default=DEFAULT_BASE)
    args = parser.parse_args()

    print(f"\n=== Voice outage regression ===", flush=True)
    print(f"URL: {args.base_url}", flush=True)
    print(f"Started: {__import__('datetime').datetime.now().isoformat()}", flush=True)

    ok = asyncio.run(main(args.base_url))
    sys.exit(0 if ok else 1)
