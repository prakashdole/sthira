#!/usr/bin/env python3
"""
layout_accept.py — Python Playwright layout/overflow acceptance checks.

Scope: responsive layout, document overflow, modal bounds, text wrapping,
keyboard focus order, reduced-motion at 375/1024/1440 viewport widths.

NOT_RUN — execution deferred per assignment.
Requires: cd frontend/v2 && npm install && npx playwright install --with-deps
Dev server must be running on http://localhost:5173 (or update BASE_URL).

Usage:
    python3 layout_accept.py http://localhost:5173
    python3 layout_accept.py http://localhost:5173 --viewport=390,844
"""

import argparse
import asyncio
import sys
from typing import Callable

from playwright.async_api import async_playwright

DEFAULT_BASE = "http://localhost:5173"
TIMEOUT = 15_000  # ms

CHECKS: list[dict] = []


def check(name: str, ok: bool, detail: str = "") -> None:
    CHECKS.append({"name": name, "pass": ok, "detail": detail})
    status = "PASS" if ok else "FAIL"
    print(f"[{status}] {name}" + (f": {detail}" if detail else ""), flush=True)


# ─── Onboarding ─────────────────────────────────────────────────────────────────

async def complete_onboarding(page):
    """Full three-step onboarding via verified data-action selectors (main.ts)."""
    # Step 1: click begin/voice
    await page.wait_for_selector('[data-action="onboarding-start"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-start"]')
    await asyncio.sleep(0.3)

    # Step 2: select EN language
    await page.wait_for_selector('[data-onboarding-language="EN"]', timeout=TIMEOUT)
    await page.click('[data-onboarding-language="EN"]')
    await page.wait_for_selector('[data-action="onboarding-language-next"]', timeout=TIMEOUT)
    await page.click('[data-action="onboarding-language-next"]')
    await asyncio.sleep(0.3)

    # Step 3: skip location (continue without location)
    # Either "continue without location" button (data-action="onboarding-complete") or location-request
    try:
        await page.wait_for_selector('[data-action="onboarding-complete"]', timeout=3000)
        await page.locator('[data-action="onboarding-complete"]').first.click()
    except Exception:
        # fallback: try location-request then complete
        await page.click('[data-action="location-request"]')
        await asyncio.sleep(0.3)
        await page.click('[data-action="onboarding-complete"]')

    # Emergency card (guidance panel) must appear
    await page.wait_for_selector('[data-testid="emergency-card"]', timeout=TIMEOUT)


async def open_source_details_modal(page):
    """Open the source details modal using data-action=details selector (main.ts:1449)."""
    await page.click('[data-action="details"]')
    await page.wait_for_selector("dialog.modal[open]", timeout=TIMEOUT)


# ─── Layout checks ─────────────────────────────────────────────────────────────

async def check_document_no_overflow(page, vp_width: int, vp_height: int) -> bool:
    """scrollWidth must not exceed innerWidth at this viewport."""
    result = await page.evaluate("""
        () => ({
            scrollWidth: document.documentElement.scrollWidth,
            innerWidth: window.innerWidth,
        })
    """)
    sw = result["scrollWidth"]
    iw = result["innerWidth"]
    ok = sw <= iw
    check(f"document_no_overflow_{vp_width}x{vp_height}", ok,
          f"scrollWidth={sw} innerWidth={iw}")
    return ok


async def check_guidance_panel_bounds(page, vp_width: int, vp_height: int) -> bool:
    """guidance-panel bottom must not exceed viewport height (within 2px margin)."""
    panel = page.locator(".guidance-panel")
    if not await panel.is_visible():
        check(f"guidance_panel_bounds_{vp_width}x{vp_height}", False, "panel not visible")
        return False
    box = await panel.bounding_box()
    if box is None:
        check(f"guidance_panel_bounds_{vp_width}x{vp_height}", False, "bounding_box=None")
        return False
    ok = box["y"] + box["height"] <= vp_height + 2
    check(f"guidance_panel_bounds_{vp_width}x{vp_height}", ok,
          f"panel bottom={box['y']+box['height']} viewport={vp_height}")
    return ok


async def check_modal_visible_and_bounded(page, vp_width: int, vp_height: int) -> bool:
    """Source details modal must be visible and fully within viewport."""
    try:
        await open_source_details_modal(page)
    except Exception as e:
        check(f"modal_bounded_{vp_width}x{vp_height}", False, f"cannot open: {e}")
        return False

    modal = page.locator("dialog.modal[open]")
    if not await modal.is_visible():
        check(f"modal_bounded_{vp_width}x{vp_height}", False, "modal not visible")
        return False

    box = await modal.bounding_box()
    if box is None:
        check(f"modal_bounded_{vp_width}x{vp_height}", False, "bounding_box=None")
        return False

    left_ok = box["x"] >= -2
    right_ok = box["x"] + box["width"] <= vp_width + 2
    top_ok = box["y"] >= 0
    bottom_ok = box["y"] + box["height"] <= vp_height + 2

    ok = left_ok and right_ok and top_ok and bottom_ok
    check(f"modal_bounded_{vp_width}x{vp_height}", ok,
          f"left={box['x']} right={box['x']+box['width']} top={box['y']} bottom={box['y']+box['height']}")
    return ok


async def check_modal_internal_scroll(page, vp_width: int, vp_height: int) -> bool:
    """Modal must scroll internally when content overflows (scrollHeight > clientHeight)."""
    try:
        await open_source_details_modal(page)
    except Exception as e:
        check(f"modal_scroll_{vp_width}x{vp_height}", False, f"cannot open: {e}")
        return False

    modal = page.locator("dialog.modal[open]")
    result = await modal.evaluate("""
        (el) => ({ scrollHeight: el.scrollHeight, clientHeight: el.clientHeight })
    """)
    sh = result["scrollHeight"]
    ch = result["clientHeight"]
    ok = sh > ch
    check(f"modal_scroll_{vp_width}x{vp_height}", ok,
          f"scrollHeight={sh} clientHeight={ch}")
    return ok


async def check_action_buttons_wrap(page, vp_width: int, vp_height: int) -> bool:
    """primary-action and secondary-action buttons must be visible and not cause overflow."""
    buttons = page.locator(".primary-action, .secondary-action")
    count = await buttons.count()
    if count == 0:
        check(f"action_wrap_{vp_width}x{vp_height}", False, "no action buttons found")
        return False

    all_ok = True
    for i in range(count):
        btn = buttons.nth(i)
        if not await btn.is_visible():
            continue
        box = await btn.bounding_box()
        if box is None:
            continue
        # Right edge must not exceed viewport width + 2px
        right_ok = box["x"] + box["width"] <= vp_width + 2
        if not right_ok:
            check(f"action_wrap_{vp_width}x{vp_height}", False,
                  f"button[{i}] right={box['x']+box['width']} > {vp_width}")
            all_ok = False
    if all_ok:
        check(f"action_wrap_{vp_width}x{vp_height}", True,
              f"{count} buttons visible and within bounds")
    return all_ok


async def check_quick_actions_no_overflow(page, vp_width: int, vp_height: int) -> bool:
    """quick-actions buttons must not cause horizontal overflow."""
    # Real selector: .quick-actions button[data-action] (main.ts:1530-1534)
    buttons = page.locator(".quick-actions button[data-action]")
    count = await buttons.count()
    if count == 0:
        check(f"quick_actions_overflow_{vp_width}x{vp_height}", False, "no quick-action buttons")
        return False

    all_ok = True
    for i in range(count):
        btn = buttons.nth(i)
        if not await btn.is_visible():
            continue
        box = await btn.bounding_box()
        if box is None:
            continue
        right_ok = box["x"] + box["width"] <= vp_width + 2
        if not right_ok:
            check(f"quick_actions_overflow_{vp_width}x{vp_height}", False,
                  f"quick-action[{i}] right={box['x']+box['width']} > {vp_width}")
            all_ok = False
    if all_ok:
        check(f"quick_actions_overflow_{vp_width}x{vp_height}", True,
              f"{count} quick-actions within bounds")
    return all_ok


async def check_voice_suggestions_touch_target(page, vp_width: int, vp_height: int) -> bool:
    """voice-suggestions buttons must have height >= 44px touch target."""
    suggestions = page.locator(".voice-suggestions button")
    count = await suggestions.count()
    if count == 0:
        # Open voice console to reveal suggestions (main.ts:1545)
        try:
            await page.locator('[data-action="voice-open"]').first.click()
            await asyncio.sleep(0.5)
        except Exception:
            pass
        count = await suggestions.count()

    if count == 0:
        check(f"voice_suggestions_touch_{vp_width}x{vp_height}", None,
              "voice-suggestions not visible (may be absent in demo data)")
        return True  # Not a failure — suggestions may not be in demo data

    first = suggestions.first()
    if not await first.is_visible():
        check(f"voice_suggestions_touch_{vp_width}x{vp_height}", None,
              "first suggestion not visible")
        return True

    box = await first.bounding_box()
    height = box["height"] if box else 0
    ok = height >= 44
    check(f"voice_suggestions_touch_{vp_width}x{vp_height}", ok,
          f"first suggestion height={height}")
    return ok


async def check_keyboard_focus_order(page, vp_width: int, vp_height: int) -> bool:
    """Tab key must move focus to interactive elements; at least one element reachable."""
    # Open voice console to have more interactive elements
    try:
        voice_btn = page.locator('[data-action="voice-open"]').first
        if await voice_btn.is_visible():
            await voice_btn.click()
            await asyncio.sleep(0.3)
    except Exception:
        pass

    focused: list[str] = []
    for _ in range(15):
        await page.keyboard.press("Tab")
        tag = await page.evaluate("""
            () => {
                const el = document.activeElement;
                if (!el) return '';
                return el.tagName.toLowerCase()
                    + (el.id ? '#' + el.id : '')
                    + (el.className && typeof el.className === 'string'
                       ? '.' + el.className.split(' ')[0] : '');
            }
        """)
        if tag and tag not in focused:
            focused.append(tag)

    ok = len(focused) > 0
    check(f"keyboard_focus_{vp_width}x{vp_height}", ok,
          f"focused {len(focused)} unique elements: {focused}")
    return ok


async def check_destination_text_wraps(page, vp_width: int, vp_height: int) -> bool:
    """destination h2 must not overflow horizontally (overflow-wrap:anywhere in CSS)."""
    h2 = page.locator(".destination h2").first()
    lede = page.locator(".destination .lede, .destination .destination-label").first()

    h2_ok = await h2.is_visible() if await h2.count() > 0 else False
    lede_ok = await lede.is_visible() if await lede.count() > 0 else False

    if not h2_ok and not lede_ok:
        check(f"destination_wrap_{vp_width}x{vp_height}", None, "destination elements not visible")
        return True

    all_ok = True
    if h2_ok:
        result = await h2.evaluate("""
            (el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth })
        """)
        sw = result["scrollWidth"]
        cw = result["clientWidth"]
        ok = sw <= cw + 1
        check(f"destination_wrap_{vp_width}x{vp_height}_h2", ok,
              f"h2 scrollWidth={sw} clientWidth={cw}")
        all_ok = all_ok and ok

    if lede_ok:
        result = await lede.evaluate("""
            (el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth })
        """)
        sw = result["scrollWidth"]
        cw = result["clientWidth"]
        ok = sw <= cw + 1
        check(f"destination_wrap_{vp_width}x{vp_height}_lede", ok,
              f"lede scrollWidth={sw} clientWidth={cw}")
        all_ok = all_ok and ok

    return all_ok


async def check_map_disclaimer_clickable(page, vp_width: int, vp_height: int) -> bool:
    """map-disclaimer anchor must have pointer-events: auto."""
    disclaimer = page.locator(".map-disclaimer")
    if not await disclaimer.is_visible():
        check(f"map_disclaimer_clickable_{vp_width}x{vp_height}", None,
              "map-disclaimer not visible")
        return True

    link = disclaimer.locator("a").first
    if not await link.is_visible():
        check(f"map_disclaimer_clickable_{vp_width}x{vp_height}", None, "link not visible")
        return True

    pe = await link.evaluate("window.getComputedStyle(el).pointerEvents")
    ok = pe == "auto"
    check(f"map_disclaimer_clickable_{vp_width}x{vp_height}", ok, f"pointer-events={pe}")
    return ok


async def check_rescue_action_wraps(page, vp_width: int, vp_height: int) -> bool:
    """rescue-action strong must not overflow (white-space:normal in CSS)."""
    rescue = page.locator(".rescue-action").first()
    if not await rescue.is_visible():
        check(f"rescue_wrap_{vp_width}x{vp_height}", None, "rescue-action not visible")
        return True

    strong = rescue.locator("strong").first()
    if not await strong.is_visible():
        check(f"rescue_wrap_{vp_width}x{vp_height}", None, "strong not visible")
        return True

    result = await strong.evaluate("""
        (el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth })
    """)
    sw = result["scrollWidth"]
    cw = result["clientWidth"]
    ok = sw <= cw + 1
    check(f"rescue_wrap_{vp_width}x{vp_height}", ok,
          f"strong scrollWidth={sw} clientWidth={cw}")
    return ok


async def check_modal_dd_wraps(page, vp_width: int, vp_height: int) -> bool:
    """Modal dd elements must wrap long CAP authority strings."""
    try:
        await open_source_details_modal(page)
    except Exception as e:
        check(f"modal_dd_wrap_{vp_width}x{vp_height}", False, f"cannot open: {e}")
        return False

    dd = page.locator("dialog.modal[open] dd").first
    if not await dd.is_visible():
        check(f"modal_dd_wrap_{vp_width}x{vp_height}", None, "dd not visible")
        return True

    result = await dd.evaluate("""
        (el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth })
    """)
    sw = result["scrollWidth"]
    cw = result["clientWidth"]
    ok = sw <= cw + 1
    check(f"modal_dd_wrap_{vp_width}x{vp_height}", ok,
          f"dd scrollWidth={sw} clientWidth={cw}")
    return ok


async def check_journey_badge_wraps(page, vp_width: int, vp_height: int) -> bool:
    """journey-status-badge must wrap long state text."""
    badge = page.locator(".journey-status-badge").first()
    count = await badge.count()
    if count == 0 or not await badge.is_visible():
        check(f"journey_badge_wrap_{vp_width}x{vp_height}", None, "badge not present")
        return True

    result = await badge.evaluate("""
        (el) => ({ scrollWidth: el.scrollWidth, clientWidth: el.clientWidth })
    """)
    sw = result["scrollWidth"]
    cw = result["clientWidth"]
    ok = sw <= cw + 1
    check(f"journey_badge_wrap_{vp_width}x{vp_height}", ok,
          f"badge scrollWidth={sw} clientWidth={cw}")
    return ok


async def check_reduced_motion(page) -> bool:
    """prefers-reduced-motion: CSS transitionDuration must be 0s."""
    await page.emulate_media({"reducedMotion": "reduce"})
    try:
        result = await page.evaluate("""
            () => {
                const el = document.querySelector('.voice-launch');
                if (!el) return null;
                return window.getComputedStyle(el).transitionDuration;
            }
        """)
        if result is None:
            check("reduced_motion", None, "voice-launch element not found")
            return True
        ok = result == "0s"
        check("reduced_motion", ok, f"transitionDuration={result}")
        return ok
    finally:
        await page.emulate_media({"reducedMotion": "no-preference"})


# ─── Viewport definitions ───────────────────────────────────────────────────────

VIEWPORTS = [
    (375, 812, "mobile"),
    (1024, 768, "tablet"),
    (1440, 900, "desktop"),
]


# ─── Main runner ────────────────────────────────────────────────────────────────

async def run_layout_checks(base_url: str, viewport: tuple[int, int, str] | None = None):
    """Run all layout checks for specified viewport(s)."""
    async with async_playwright() as p:
        browser = await p.chromium.launch(headless=True)
        page = await browser.new_page()

        targets = [viewport] if viewport else VIEWPORTS

        for vp_width, vp_height, vp_name in targets:
            print(f"\n=== Viewport: {vp_width}x{vp_height} ({vp_name}) ===", flush=True)
            await page.set_viewport_size({"width": vp_width, "height": vp_height})

            # Load app and complete onboarding
            await page.goto(base_url, wait_until="domcontentloaded")
            await complete_onboarding(page)
            await asyncio.sleep(0.3)

            # Run checks
            await check_document_no_overflow(page, vp_width, vp_height)
            await check_guidance_panel_bounds(page, vp_width, vp_height)
            await check_modal_visible_and_bounded(page, vp_width, vp_height)
            await check_modal_internal_scroll(page, vp_width, vp_height)
            await check_action_buttons_wrap(page, vp_width, vp_height)
            await check_quick_actions_no_overflow(page, vp_width, vp_height)
            await check_voice_suggestions_touch_target(page, vp_width, vp_height)
            await check_keyboard_focus_order(page, vp_width, vp_height)
            await check_destination_text_wraps(page, vp_width, vp_height)
            await check_map_disclaimer_clickable(page, vp_width, vp_height)
            await check_rescue_action_wraps(page, vp_width, vp_height)
            await check_modal_dd_wraps(page, vp_width, vp_height)
            await check_journey_badge_wraps(page, vp_width, vp_height)

        # Reduced-motion check (single run, no viewport dependency)
        print("\n=== Reduced motion ===", flush=True)
        await page.set_viewport_size({"width": 1440, "height": 900})
        await page.goto(base_url, wait_until="domcontentloaded")
        await complete_onboarding(page)
        await check_reduced_motion(page)

        await browser.close()

        # Summary
        total = len(CHECKS)
        passed = sum(1 for c in CHECKS if c["pass"] is True)
        failed = sum(1 for c in CHECKS if c["pass"] is False)
        skipped = sum(1 for c in CHECKS if c["pass"] is None)
        print(f"\n=== Summary: {passed}/{total} passed, {failed} failed, {skipped} skipped ===", flush=True)
        return failed == 0


# ─── Manual-only checks (cannot be automated via Playwright) ────────────────────

MANUAL_CHECKS = """
MANUAL_ONLY_CHECKS (cannot be automated — Playwright cannot control browser zoom level):

1. 200% zoom — HI/ML label legibility and topbar wrapping:
   - Set browser zoom to 200% in browser settings or DevTools device toolbar
   - Resize to 375x812 viewport
   - Verify:
     a) Malayalam brand mark "സ്" renders correctly in topbar
     b) Hindi "हिन्दी" button label fits within language-switcher without clipping
     c) Guidance h1 "Leave the area immediately" fits at 375px without horizontal scroll
     d) Emergency card buttons remain tappable (min 44px touch target at zoom)

2. Keyboard dialog semantics (worker 9): ARIA dialog role, focus trap inside modal,
   Escape key closes modal — belongs to worker 9 scope.

3. Browser zoom ≠ device toolbar scaling: The Playwright viewport API sets CSS pixel
   dimensions. Device toolbar "zoom" in Chrome DevTools applies device pixel ratio
   scaling which is different from CSS viewport scaling. Do not equate them.
"""


async def main():
    parser = argparse.ArgumentParser(description="Layout acceptance (NOT_RUN — deferred)")
    parser.add_argument("base_url", nargs="?", default=DEFAULT_BASE)
    parser.add_argument("--viewport", help="width,height (e.g. 390,844)")
    args = parser.parse_args()

    viewport = None
    if args.viewport:
        try:
            w, h = map(int, args.viewport.split(","))
            viewport = (w, h, "custom")
        except Exception:
            print(f"Invalid viewport format: {args.viewport}", file=sys.stderr)
            sys.exit(1)

    print("NOT_RUN — execution deferred per assignment", flush=True)
    print(f"Target: {args.base_url}", flush=True)
    if viewport:
        print(f"Viewport: {viewport[0]}x{viewport[1]}", flush=True)
    else:
        print(f"Viewports: {[f'{w}x{h}' for w,h,n in VIEWPORTS]}", flush=True)
    print(MANUAL_CHECKS, flush=True)

    # Structural validation only (no browser launch)
    # When unblocked: replace False with await run_layout_checks(...)
    ok = False
    sys.exit(0 if ok else 1)


if __name__ == "__main__":
    asyncio.run(main())
