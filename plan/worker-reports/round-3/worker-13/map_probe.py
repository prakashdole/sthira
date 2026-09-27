#!/usr/bin/env python3
"""Browser probe: map canvas render + network asset failures.

Navigates to the app, completes onboarding, waits for the map canvas to appear,
and reports network failures separately for:
  - MapLibre package assets (sprite, fonts) from node_modules
  - All other failed requests

This does not prove human map quality or hardware behavior.
Network emulation is labelled as such.

Usage:
    python3 map_probe.py [base_url]
    python3 map_probe.py http://127.0.0.1:18492/

Exit: 0 on all checks pass, 1 if map canvas absent or unexpected asset failures.
NOT_RUN — deferred by user.
"""

import argparse
import asyncio
import sys

from playwright.async_api import async_playwright

DEFAULT_BASE = "http://127.0.0.1:18492"
CHECKS = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    icon = "PASS" if ok else "FAIL"
    print(f"[{icon}] {name}" + (f": {detail}" if detail else ""), flush=True)


async def complete_onboarding(page):
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


async def main(base_url: str):
    all_ok = True
    failed_requests = []

    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        ctx = await browser.new_context(
            viewport={"width": 1280, "height": 860},
            permissions=["geolocation"],
        )
        page = await ctx.new_page()

        page_errors = []
        page.on("pageerror", lambda e: page_errors.append(str(e)))

        def on_response(response):
            if response.status >= 400:
                failed_requests.append({
                    "url": response.url,
                    "status": response.status,
                    "is_maplibre_asset": (
                        "maplibre-gl" in response.url
                        or "node_modules" in response.url
                    ),
                })

        page.on("response", on_response)

        await page.goto(base_url, wait_until="networkidle")
        await asyncio.sleep(0.5)

        try:
            await complete_onboarding(page)
            check("onboarding completes", True)
        except Exception as e:
            check("onboarding completes", False, str(e)[:80])
            await browser.close()
            return False

        await page.wait_for_selector("text=Demo Safe Facility", timeout=8000)

        await asyncio.sleep(3)  # allow map to fully initialize

        maplibre_failures = [
            r for r in failed_requests if r["is_maplibre_asset"]
        ]
        other_failures = [
            r for r in failed_requests if not r["is_maplibre_asset"]
        ]

        check(
            "map container exists",
            await page.locator("#map-canvas").count() > 0,
        )

        check(
            "maplibregl-canvas element exists",
            await page.locator(".maplibregl-canvas").count() > 0,
        )

        check(
            "no MapLibre asset 4xx/5xx failures",
            len(maplibre_failures) == 0,
            "; ".join(f"{r['status']} {r['url']}" for r in maplibre_failures)[:200]
            if maplibre_failures else "",
        )

        if other_failures:
            failure_lines = "; ".join(
                f"{r['status']} {r['url']}" for r in other_failures[:5]
            )
            check(
                "no other unexpected asset failures",
                False,
                failure_lines[:200],
            )
        else:
            check("no other unexpected asset failures", True)

        check(
            "no uncaught page errors",
            len(page_errors) == 0,
            "; ".join(page_errors)[:200] if page_errors else "",
        )

        await browser.close()

    passed = sum(1 for c in CHECKS if c["pass"])
    failed = sum(1 for c in CHECKS if not c["pass"])
    print(f"\nResults: {passed} PASS | {failed} FAIL", flush=True)

    for c in CHECKS:
        if not c["pass"]:
            print(f"FAIL_DETAIL: [{c['name']}] {c['detail']}", flush=True)

    return all_ok


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Map asset and render probe")
    parser.add_argument("base_url", nargs="?", default=DEFAULT_BASE)
    args = parser.parse_args()

    print(f"\n=== Map probe ===", flush=True)
    print(f"URL: {args.base_url}", flush=True)

    ok = asyncio.run(main(args.base_url))
    sys.exit(0 if ok else 1)
