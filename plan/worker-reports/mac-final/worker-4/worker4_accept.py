#!/usr/bin/env python3
"""Worker 4 — Browser regression: idempotency retry, stay restoration, outage exit codes.

Scope NOT covered by other workers:
- Lost reservation response + identical key/payload retry (server must commit first request)
- Pending/accepted stay restoration after page reload (sessionStorage binding)
- 503/504 outage recovery as focused regression (Worker 2 script had exit-code bug)

Excludes (covered by Workers 1/2/3):
- core onboarding, text command, guidance destination (Worker 1)
- duplicate-start protection, GPS arrival (Worker 1)
- language-switch stale-response guard (Worker 3)
- 503/504 outage injection + assistant unavailable shown (Worker 2, prototype_accept)

Usage:
    python3 worker4_accept.py http://127.0.0.1:18492/ [--json OUT]
    python3 worker4_accept.py http://127.0.0.1:18492/ --section=all

Sections: idempotency, restoration, outage
Exit 0 only if every executed check passes; nonzero on any failure.
"""

import argparse
import asyncio
import json
import sys
from typing import Optional

from playwright.async_api import async_playwright, TimeoutError as PWTimeout

DEFAULT_BASE = "http://127.0.0.1:18492"
SHELTER = {"longitude": 76.105, "latitude": 11.570, "accuracy": 10}
TIMEOUT = 15_000
CHECKS: list[dict] = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"check": name, "status": "PASS" if ok else "FAIL", "detail": str(detail)[:240]})
    icon = "PASS" if ok else "FAIL"
    print(f"[{icon}] {name}" + (f": {detail}" if detail else ""), flush=True)


async def complete_onboarding(page):
    await page.goto(args.base_url)
    await page.wait_for_load_state("networkidle")
    await page.click('[data-action="onboarding-start"]')
    await page.click('[data-onboarding-language="EN"]')
    await page.click('[data-action="onboarding-language-next"]')
    await page.locator('[data-action="onboarding-complete"]').first.click()
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=TIMEOUT)


async def click_route_and_capture_reservation(page) -> dict:
    """Click route and return reservation-related session state."""
    await page.locator('[data-action="route"]').first.click()
    try:
        await page.wait_for_selector('[data-action="directions-close"]', timeout=8000)
    except PWTimeout:
        pass

    state = await page.evaluate("""() => ({
        stay_id: sessionStorage.getItem('sthira_stay_id'),
        facility_id: sessionStorage.getItem('sthira_stay_facility_id'),
        reservation_id: sessionStorage.getItem('sthira_reservation_id'),
        pending_reservation: sessionStorage.getItem('pending_reservation'),
    })""")
    return state


async def section_idempotency(browser, base: str) -> None:
    """Verify: lost reservation response + identical key/payload retry.

    Flow:
    1. Intercept reservation POST, drop the response (simulate network loss).
    2. Verify pending_reservation is saved in sessionStorage.
    3. Release interception, reload page (which will retry).
    4. Verify server committed the FIRST request (idempotency — same stay_id).
    """
    s = "idempotency"
    ctx, page = await new_page(browser)
    await page.goto(base)
    await page.wait_for_load_state("networkidle")
    await onboard(page, base)

    response_dropped = False

    async def drop_reservation_response(route):
        nonlocal response_dropped
        response_dropped = True
        await route.abort("connection_timeout")

    page.on("response", lambda r: print(f"  [net] {r.method} {r.url} -> {r.status}", flush=True) if "/api/v3/reservations" in r.url else None)

    try:
        await page.route("**/api/v3/reservations", drop_reservation_response)

        await page.locator('[data-action="route"]').first.click()
        await asyncio.sleep(1.5)

        pending_raw = await page.evaluate("sessionStorage.getItem('pending_reservation')")
        has_pending = pending_raw is not None
        check(s, "pending_reservation saved after dropped response", has_pending,
              f"pending={pending_raw is not None}")

        if has_pending:
            pending = json.loads(pending_raw)
            key_before = pending.get("payload", {}).get("idempotency_key", "")
            check(s, "pending_reservation contains idempotency_key", bool(key_before),
                  f"key={key_before[:20]}...")

        response_dropped_check = response_dropped
        check(s, "reservation response was intercepted and dropped", response_dropped_check)

    finally:
        await page.unroute("**/api/v3/reservations")

    stay_before = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    fac_before = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")

    await page.reload(wait_until="networkidle")
    await asyncio.sleep(2)

    stay_after_retry = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    fac_after_retry = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")

    if stay_after_retry and fac_after_retry:
        check(s, "after retry: stay_id present (server committed first request)",
              bool(stay_after_retry))
        check(s, "after retry: facility_id matches", fac_after_retry == fac_before,
              f"before={fac_before} after={fac_after_retry}")
        check(s, "after retry: same stay_id on retry (idempotency replayed)",
              stay_after_retry == stay_before if stay_before else False,
              f"stay_before={stay_before} stay_after={stay_after_retry}")
    else:
        check(s, "after retry: stay_id present (server committed first request)", False,
              f"stay_after={stay_after_retry}")
        check(s, "after retry: facility_id matches", False,
              f"before={fac_before} after={fac_after_retry}")

    pending_after = await page.evaluate("sessionStorage.getItem('pending_reservation')")
    check(s, "pending_reservation cleared after successful retry", pending_after is None,
          f"pending_after={pending_after}")

    page_errors = getattr(page, "errors", [])
    check(s, "no uncaught page errors", len(page_errors) == 0, "; ".join(page.errors)[:200] if page.errors else "")

    await ctx.close()


async def section_restoration(browser, base: str) -> None:
    """Verify: pending/accepted stay restoration and destination binding after reload."""
    s = "restoration"
    ctx, page = await new_page(browser)
    await page.goto(base)
    await page.wait_for_load_state("networkidle")
    await onboard(page, base)

    await click_route_and_capture_reservation(page)
    await asyncio.sleep(0.5)

    stay_before = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    fac_before = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")
    pending_before = await page.evaluate("sessionStorage.getItem('pending_reservation')")

    check(s, "reservation: stay_id stored in sessionStorage", bool(stay_before),
          f"stay={stay_before}")
    check(s, "reservation: facility_id stored in sessionStorage", bool(fac_before),
          f"fac={fac_before}")
    check(s, "reservation: no pending after success", pending_before is None,
          f"pending={pending_before}")

    destination_text_before = await page.evaluate("""() => {
        const dest = document.querySelector('.destination h2, .destination h3, [data-testid=\"destination-name\"]');
        return dest ? dest.textContent : null;
    }""")

    await page.reload(wait_until="networkidle")
    await asyncio.sleep(1.5)

    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=8000)

    stay_restored = await page.evaluate("sessionStorage.getItem('sthira_stay_id')")
    fac_restored = await page.evaluate("sessionStorage.getItem('sthira_stay_facility_id')")
    route_btn_disabled = await page.evaluate("""() => {
        const btn = document.querySelector('[data-action=\"route\"]');
        return btn ? btn.disabled : null;
    }""")

    check(s, "after reload: stay_id restored from sessionStorage", stay_restored == stay_before,
          f"before={stay_before} after={stay_restored}")
    check(s, "after reload: facility_id restored from sessionStorage", fac_restored == fac_before,
          f"before={fac_before} after={fac_restored}")
    check(s, "after reload: route button is not disabled (stay is active)", route_btn_disabled is False,
          f"disabled={route_btn_disabled}")

    try:
        await page.wait_for_selector('[data-action="directions-close"]', timeout=3000)
        directions_visible = True
    except PWTimeout:
        directions_visible = False
    check(s, "after reload: directions sheet auto-opened for restored stay", directions_visible,
          "directions sheet expected but not visible" if not directions_visible else "")

    page_errors = getattr(page, "errors", [])
    check(s, "no uncaught page errors on restoration", len(page_errors) == 0,
          "; ".join(page.errors)[:200] if page.errors else "")

    await ctx.close()


async def section_outage_fixed(browser, base: str) -> None:
    """Focused regression: 503/504 outage messaging and recovery.

    This is the same scenario Worker 2's script covered, but Worker 2's
    script had a bug: sys.exit(0) always returned 0 regardless of failures.
    prototype_accept.py already covers this correctly, but we re-run to
    confirm the exit-code behavior and verify the recovery is clean.
    """
    s = "outage"
    ctx, page = await new_page(browser)
    await page.goto(base)
    await page.wait_for_load_state("networkidle")
    await onboard(page, base)

    EN_ASSISTANT_UNAVAILABLE = "Voice Map Control is unavailable. The displayed route and emergency call option still work."

    async def failing_503(route):
        await route.fulfill(status=503, content_type="application/json",
                            body='{"error":{"code":"MODEL_UNAVAILABLE"}}')

    async def failing_504(route):
        await route.fulfill(status=504, content_type="application/json",
                            body='{"error":{"code":"MODEL_UNAVAILABLE"}}')

    for code, injector in [(503, failing_503), (504, failing_504)]:
        await page.route("**/api/v3/voice/process", injector)

        if not await page.locator("#command-input").is_visible():
            await page.locator('[data-action="voice-open"]').first.click()
        await page.wait_for_selector("#command-input", state="visible", timeout=5000)
        await page.fill("#command-input", "Show my route")
        await page.click('[data-command-form] button[type="submit"]')

        try:
            await page.wait_for_function(
                "!(document.querySelector('#command-input') || {}).disabled",
                timeout=12000,
            )
        except Exception:
            pass

        error_elem = page.locator(".command-error")
        try:
            await error_elem.wait_for(timeout=6000)
            error_visible = True
        except Exception:
            error_visible = False

        check(s, f"{code}: command-error element visible during {code}", error_visible)

        if error_visible:
            error_text = (await error_elem.inner_text()).strip()
            has_unavailable = "unavailable" in error_text.lower()
            check(s, f"{code}: error text contains 'unavailable'", has_unavailable,
                  error_text[:120])
            has_not_found = "not found" in error_text.lower()
            check(s, f"{code}: error text does NOT contain 'not found' (no misleading fallback)",
                  not has_not_found, error_text[:120])
        else:
            check(s, f"{code}: error text contains 'unavailable'", False, "error element absent")

        await page.unroute("**/api/v3/voice/process")

    await page.route("**/api/v3/voice/process", failing_503)
    if not await page.locator("#command-input").is_visible():
        await page.locator('[data-action="voice-open"]').first.click()
    await page.wait_for_selector("#command-input", state="visible", timeout=5000)
    await page.fill("#command-input", "where can I go")
    await page.click('[data-command-form] button[type="submit"]')

    try:
        await page.wait_for_function(
            "!(document.querySelector('#command-input') || {}).disabled",
            timeout=12000,
        )
    except Exception:
        pass

    error_elem = page.locator(".command-error")
    recovery_has_error = False
    try:
        await error_elem.wait_for(timeout=3000)
        recovery_has_error = await error_elem.is_visible()
    except Exception:
        pass

    check(s, "recovery: after unroute, no command-error shown", not recovery_has_error)

    page_errors = getattr(page, "errors", [])
    check(s, "no uncaught page errors throughout outage test",
          len(page_errors) == 0, "; ".join(page.errors)[:200] if page.errors else "")

    await ctx.close()


async def new_page(browser, **ctx_kw):
    ctx_kw.setdefault("viewport", {"width": 1280, "height": 860})
    ctx_kw.setdefault("geolocation", SHELTER)
    ctx_kw.setdefault("permissions", ["geolocation"])
    ctx = await browser.new_context(**ctx_kw)
    page = await ctx.new_page()
    page.errors: list[str] = []
    page.on("pageerror", lambda e: page.errors.append(str(e)))
    return ctx, page


async def onboard(page, base):
    await page.goto(base)
    await page.wait_for_load_state("networkidle")
    await page.click('[data-action="onboarding-start"]')
    await page.click('[data-onboarding-language="EN"]')
    await page.click('[data-action="onboarding-language-next"]')
    await page.locator('[data-action="onboarding-complete"]').first.click()
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=15000)


SECTIONS = {
    "idempotency": section_idempotency,
    "restoration": section_restoration,
    "outage": section_outage_fixed,
}


async def main(base: str, sections: list[str], out: Optional[str]):
    async with async_playwright() as p:
        browser = await p.chromium.launch(
            headless=True,
            args=["--autoplay-policy=no-user-gesture-required"]
        )
        for name in sections:
            try:
                await SECTIONS[name](browser, base)
            except PWTimeout as e:
                check(name, "section completed without timeout", False, str(e)[:120])
            except Exception as e:
                check(name, f"section {name} raised exception", False, str(e)[:120])
        await browser.close()

    if out:
        with open(out, "w") as f:
            json.dump(CHECKS, f, ensure_ascii=False, indent=1)

    fails = [r for r in CHECKS if r["status"] != "PASS"]
    passed = len(CHECKS) - len(fails)
    print(f"\n{passed} PASS, {len(fails)} FAIL", flush=True)
    for c in CHECKS:
        if c["status"] != "PASS":
            detail = c["detail"].replace("\n", " ")[:160]
            print(f"  FAIL_DETAIL: [{c['check']}] {detail}", flush=True)

    return 1 if fails or not CHECKS else 0


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Worker 4 acceptance: idempotency + restoration + outage")
    parser.add_argument("base_url", nargs="?", default=DEFAULT_BASE)
    parser.add_argument("--section", default="all",
                        help="Comma-separated sections or 'all' (default: all)")
    parser.add_argument("--json", dest="out", default=None,
                        help="Write results JSON to file")
    args = parser.parse_args()

    sections = list(SECTIONS.keys()) if args.section == "all" else args.section.split(",")
    invalid = [s for s in sections if s not in SECTIONS]
    if invalid:
        print(f"Unknown sections: {invalid}. Available: {list(SECTIONS.keys())}", flush=True)
        sys.exit(1)

    print(f"\n=== Worker 4 acceptance ===", flush=True)
    print(f"Base URL  : {args.base_url}", flush=True)
    print(f"Sections  : {sections}", flush=True)
    print(f"Started   : {__import__('datetime').datetime.now().isoformat()}", flush=True)
    print("-" * 50, flush=True)

    ok = asyncio.run(main(args.base_url, sections, args.out))

    print("-" * 50, flush=True)
    print(f"Exit      : {'0 (all passed)' if ok else '1 (has failures)'}", flush=True)
    sys.exit(ok)
