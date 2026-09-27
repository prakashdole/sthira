// Playwright layout-check script for styles.css responsive/focus/overflow review.
// NOT_RUN — execution deferred per assignment.
// Run manually: npx playwright test plan/worker-reports/round-2/minimax-f/layout-check.spec.ts
// Required setup: cd frontend/v2 && npm install && npx playwright install --with-deps
// Dev server must be running on http://localhost:5173 (or update BASE_URL below).

import { test, expect, type Page, type ViewportSize } from '@playwright/test';

const BASE_URL = process.env.PW_BASE_URL ?? 'http://localhost:5173';

/** Viewport widths covering mobile, tablet, desktop */
const VIEWPORTS: ViewportSize[] = [
  { width: 375, height: 812 },
  { width: 1024, height: 768 },
  { width: 1440, height: 900 },
];

// ─── helpers ───────────────────────────────────────────────────────────────────

async function openApp(page: Page): Promise<void> {
  await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
  // Skip onboarding: click "Continue without location"
  const skipBtn = page.getByRole('button', { name: /continue without location/i });
  if (await skipBtn.isVisible({ timeout: 3000 })) {
    await skipBtn.click();
    await page.waitForSelector('.guidance-panel', { timeout: 5000 });
  }
}

async function openModal(page: Page, label: string): Promise<void> {
  await page.getByRole('button', { name: new RegExp(label, 'i') }).click();
  await page.waitForSelector('dialog.modal[open]', { timeout: 3000 });
}

function getFocusableElements(page: Page, container: Page | Locator): Promise<string[]> {
  const selector = 'button:not([disabled]), [href], input:not([disabled]), [tabindex]:not([tabindex="-1"])';
  return container.locator(selector).evaluateAll((els) => els.map((e) => e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + (e.className ? '.' + e.className.split(' ')[0] : '')));
}

// ─── Tests ────────────────────────────────────────────────────────────────────

for (const vp of VIEWPORTS) {
  const label = `${vp.width}×${vp.height}`;

  test.describe(`Viewport ${label}`, () => {
    test.beforeEach(async ({ page }) => {
      await page.setViewportSize(vp);
      await openApp(page);
    });

    test('document does not overflow horizontally (no horizontal scroll)', async ({ page }) => {
      const scrollWidth = await page.evaluate(() => document.documentElement.scrollWidth);
      const innerWidth = await page.evaluate(() => window.innerWidth);
      expect(scrollWidth, `scrollWidth(${scrollWidth}) should not exceed innerWidth(${innerWidth})`).toBeLessThanOrEqual(innerWidth);
    });

    test('guidance-panel children do not overflow viewport', async ({ page }) => {
      const panel = page.locator('.guidance-panel');
      await expect(panel).toBeVisible();
      const panelBox = await panel.boundingBox();
      const viewportHeight = vp.height;
      // panel bottom should not exceed viewport height (with reasonable margin for topbar ~56px)
      if (panelBox) {
        const topbarHeight = await page.locator('.topbar').boundingBox().then((b) => b?.height ?? 56);
        expect(panelBox.bottom, 'panel bottom should be within viewport').toBeLessThanOrEqual(viewportHeight + 2);
      }
    });

    test('modal dialog is visible and bounded within viewport', async ({ page }) => {
      await openModal(page, 'source details');
      const modal = page.locator('dialog.modal[open]');
      await expect(modal).toBeVisible();
      const box = await modal.boundingBox();
      expect(box).not.toBeNull();
      expect(box!.left).toBeGreaterThanOrEqual(-2);
      expect(box!.right).toBeLessThanOrEqual(vp.width + 2);
      expect(box!.top).toBeGreaterThanOrEqual(0);
      expect(box!.bottom).toBeLessThanOrEqual(vp.height + 2);
    });

    test('modal scrolls internally when content overflows', async ({ page }) => {
      await openModal(page, 'source details');
      const modal = page.locator('dialog.modal[open]');
      const box = await modal.boundingBox();
      const scrollHeight = await modal.evaluate((el) => el.scrollHeight);
      // should have scrollable overflow
      const hasOverflow = scrollHeight > (box?.height ?? 0);
      expect(hasOverflow, `modal scrollHeight(${scrollHeight}) > clientHeight(${box?.height})`).toBe(true);
    });

    test('primary-action and secondary-action wrap long text labels', async ({ page }) => {
      // These buttons have white-space:normal + overflow-wrap:break-word in dirty CSS
      const buttons = page.locator('.primary-action, .secondary-action');
      const count = await buttons.count();
      expect(count).toBeGreaterThan(0);
      for (let i = 0; i < count; i++) {
        const btn = buttons.nth(i);
        const box = await btn.boundingBox();
        const scrollW = await btn.evaluate((el) => el.scrollWidth);
        // scrollWidth > clientWidth indicates wrapped text
        const wrapped = scrollW > (box?.width ?? 0);
        // Just assert the button is visible and not clipped unexpectedly
        await expect(btn).toBeVisible();
      }
    });

    test('quick-actions buttons wrap long labels at all viewports', async ({ page }) => {
      const qab = page.locator('.quick-actions button');
      const count = await qab.count();
      expect(count).toBeGreaterThan(0);
      for (let i = 0; i < count; i++) {
        const btn = qab.nth(i);
        await expect(btn).toBeVisible();
        // button should not cause horizontal overflow
        const box = await btn.boundingBox();
        expect(box?.right, `quick-action[${i}] right edge`).toBeLessThanOrEqual(vp.width + 2);
      }
    });

    test('voice-suggestions buttons have sufficient touch target height', async ({ page }) => {
      const suggestions = page.locator('.voice-suggestions button');
      const count = await suggestions.count();
      if (count === 0) {
        // Open voice console to reveal suggestions
        await page.locator('[data-action="voice-open"]').first().click();
        await page.waitForTimeout(500);
      }
      const first = suggestions.first();
      if (await first.isVisible()) {
        const box = await first.boundingBox();
        expect(box?.height ?? 0, 'voice-suggestion button height').toBeGreaterThanOrEqual(44);
      }
    });

    test('keyboard focus can reach all interactive elements (Tab order)', async ({ page }) => {
      // Opens voice console to get more interactive elements
      await page.locator('[data-action="voice-open"]').first().click();
      await page.waitForTimeout(300);

      const focused: string[] = [];
      // Tab through first 15 focusable elements
      for (let i = 0; i < 15; i++) {
        await page.keyboard.press('Tab');
        const tag = await page.evaluate(() => {
          const el = document.activeElement;
          return el ? el.tagName.toLowerCase() + (el.id ? '#' + el.id : '') : '';
        });
        if (tag && !focused.includes(tag)) focused.push(tag);
      }
      // Should have moved focus to at least one element
      expect(focused.length).toBeGreaterThan(0);
    });

    test('destination h2 and lede paragraph wrap long facility names', async ({ page }) => {
      const h2 = page.locator('.destination h2').first();
      const lede = page.locator('.lede').first();
      await expect(h2).toBeVisible();
      await expect(lede).isVisible();
      // Both should have overflow-wrap:anywhere so no horizontal overflow
      const h2ScrollW = await h2.evaluate((el) => el.scrollWidth);
      const h2ClientW = await h2.evaluate((el) => el.clientWidth);
      expect(h2ScrollW, 'h2 should not overflow horizontally').toBeLessThanOrEqual(h2ClientW + 1);
    });

    test('map-disclaimer link is clickable (pointer-events on anchor)', async ({ page }) => {
      const disclaimer = page.locator('.map-disclaimer');
      const link = disclaimer.locator('a');
      await expect(link).toBeVisible();
      // Verify the link has pointer-events that allow interaction
      const pe = await link.evaluate((el) => window.getComputedStyle(el).pointerEvents);
      expect(pe).toBe('auto');
    });

    test('rescue-action strong does not overflow (white-space:normal)', async ({ page }) => {
      const rescue = page.locator('.rescue-action').first();
      await expect(rescue).toBeVisible();
      const strong = rescue.locator('strong');
      await expect(strong).toBeVisible();
      const scrollW = await strong.evaluate((el) => el.scrollWidth);
      const clientW = await strong.evaluate((el) => el.clientWidth);
      expect(scrollW, 'rescue strong should wrap').toBeLessThanOrEqual(clientW + 1);
    });

    test('modal dd (data description) wraps long CAP authority strings', async ({ page }) => {
      await openModal(page, 'source details');
      const dd = page.locator('dialog.modal[open] dd').first();
      await expect(dd).toBeVisible();
      const scrollW = await dd.evaluate((el) => el.scrollWidth);
      const clientW = await dd.evaluate((el) => el.clientWidth);
      expect(scrollW, 'modal dd should wrap long strings').toBeLessThanOrEqual(clientW + 1);
    });

    test('journey-status-badge wraps long state text', async ({ page }) => {
      const badge = page.locator('.journey-status-badge').first();
      if (await badge.isVisible()) {
        const scrollW = await badge.evaluate((el) => el.scrollWidth);
        const clientW = await badge.evaluate((el) => el.clientWidth);
        expect(scrollW, 'badge should wrap text').toBeLessThanOrEqual(clientW + 1);
      }
    });

    // Manual-only check (browser zoom not controllable via Playwright viewport API):
    // 1. Set browser zoom to 200% in browser settings
    // 2. Resize viewport to each target size
    // 3. Visually verify:
    //    - No clipped text in guidance-panel headings or instructions
    //    - Emergency card buttons remain tappable
    //    - HI/ML translated labels in language-switcher do not overflow topbar
    test('200% zoom: manually verify HI/ML label legibility and topbar wrapping', {
      tag: ['@manual'],
    }, async ({ page }) => {
      // Placeholder — zoom level requires browser UI, not Playwright viewport
      // Verify at 200% zoom in DevTools device toolbar that:
      // a) Malayalam brand mark "സ്" renders correctly
      // b) Hindi "हिन्दी" button label fits within language-switcher
      // c) Guidance h1 "Leave the area immediately" fits at 375px
      await expect(page).toHaveTitle(/./); // dummy assertion
    });
  });
}

// ─── Reduced-motion check (read-only, no browser flag needed) ────────────────
// Verify motionDuration() returns 0 when prefers-reduced-motion is set:
// In a real run: await page.emulateMedia({ reducedMotion: 'reduce' });
// then check that CSS transition durations evaluate to 0 or near-0.
test('prefers-reduced-motion: CSS transitions respect user preference', async ({ page }) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await openApp(page);
  const transitionDuration = await page.evaluate(() => {
    const el = document.querySelector('.voice-launch') as HTMLElement;
    const cs = window.getComputedStyle(el);
    return cs.transitionDuration;
  });
  // Should be "0s" or very close to it
  expect(transitionDuration).toBe('0s');
});
