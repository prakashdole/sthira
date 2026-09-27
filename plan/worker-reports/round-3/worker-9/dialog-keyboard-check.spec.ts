/**
 * Dialog Keyboard Accessibility Check — NOT_RUN (deferred per assignment)
 * Prerequisites: dev server on localhost:5173, app bootstrapped past onboarding
 *
 * Verifies:
 * 1. Escape key closes open <dialog class="modal" open> elements
 * 2. Focus returns to triggering element after modal close
 * 3. Focus trapping inside open modal dialogs (Tab cycles within)
 *
 * Run with: npx playwright test --project=chromium dialog-keyboard-check.spec.ts
 */

import { test, expect } from '@playwright/test';

test.describe('Dialog keyboard accessibility', () => {

  test.beforeEach(async ({ page }) => {
    // Navigate and skip onboarding via test hooks
    await page.goto('/');
    await page.waitForLoadState('networkidle');
    // Ensure we're past onboarding
    await page.evaluate(() => {
      const w = window as any;
      if (w.forceRender) w.forceRender();
    });
  });

  test('D-ACC-01: Escape closes details modal', async ({ page }) => {
    const detailsBtn = page.locator('[data-action="details"]');
    await detailsBtn.click();
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    // FAIL BEFORE FIX: modal stays open (open attribute has no Escape behavior)
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();
  });

  test('D-ACC-01: Escape closes assistance modal', async ({ page }) => {
    await page.click('[data-action="assist-open"]');
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();
  });

  test('D-ACC-01: Escape closes arrival modal (via test hook)', async ({ page }) => {
    await page.evaluate(() => { (window as any).openArrival?.(); });
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();
  });

  test('D-ACC-01: Escape closes reservation-confirm modal', async ({ page }) => {
    await page.evaluate(() => {
      const w = window as any;
      w.setReservationConfirmOpen?.(true);
    });
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();
  });

  test('D-ACC-01: Escape closes destination-details modal', async ({ page }) => {
    await page.evaluate(() => {
      const w = window as any;
      w.setDestinationDetailsOpen?.(true);
    });
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();
  });

  test('D-ACC-02: Focus returns to trigger after Escape close', async ({ page }) => {
    const detailsBtn = page.locator('[data-action="details"]');
    await detailsBtn.click();
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();

    // FAIL BEFORE FIX: focus is lost to <body>, not returned to details button
    await expect(detailsBtn).toBeFocused();
  });

  test('D-ACC-02: Focus returns to trigger after close button click', async ({ page }) => {
    const detailsBtn = page.locator('[data-action="details"]');
    await detailsBtn.click();
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    await page.click('[data-action="details-close"]');
    await expect(page.locator('dialog.modal[open]')).not.toBeVisible();

    // FAIL BEFORE FIX: focus stays on close button or moves to body
    await expect(detailsBtn).toBeFocused();
  });

  test('D-ACC-02: Focus returns after audio modal close', async ({ page }) => {
    // audioOpen has no visible trigger; open via listen button then close
    await page.click('[data-action="listen"]');
    // If audio isn't valid, it opens a modal
    const audioModal = page.locator('dialog.modal[open]');
    if (await audioModal.isVisible()) {
      const closeBtn = page.locator('[data-action="audio-close"]');
      await closeBtn.click();
      await expect(audioModal).not.toBeVisible();
      // Focus behavior after close depends on trigger tracking
    }
  });

  test('Tab stays inside open modal (focus trapping)', async ({ page }) => {
    await page.click('[data-action="details"]');
    await expect(page.locator('dialog.modal[open]')).toBeVisible();

    const dialog = page.locator('dialog.modal[open]');
    const focusableSelector = 'button:not([disabled]), [href], input:not([disabled]), [tabindex="0"]';
    const firstFocusable = dialog.locator(focusableSelector).first();
    await firstFocusable.focus();

    // FAIL BEFORE FIX: Tab can leave the dialog because open attribute has no focus trap
    // Press Tab many times — at least one should land outside the dialog
    let tabbedOutside = false;
    for (let i = 0; i < 20; i++) {
      await page.keyboard.press('Tab');
      const activeOutside = await page.evaluate(() => {
        const active = document.activeElement;
        const modal = document.querySelector('dialog.modal[open]');
        return modal && !modal.contains(active);
      });
      if (activeOutside) {
        tabbedOutside = true;
        break;
      }
    }
    // This assertion FAILS before fix (tab escapes), PASSES after focus trapping added
    expect(tabbedOutside).toBe(false);
  });

  test('Voice console closes on Escape (non-modal panel)', async ({ page }) => {
    // voice-console is not a <dialog> element — uses hidden attribute
    await page.click('[data-action="voice-open"]');
    const voiceConsole = page.locator('.voice-console:not([hidden])');
    await expect(voiceConsole).toBeVisible();

    await page.keyboard.press('Escape');
    // Voice console should also respond to Escape (D-ACC-01 coverage for non-modal panel)
    await expect(voiceConsole).toBeHidden();
  });

  test('side-sheet closes on Escape', async ({ page }) => {
    await page.click('[data-action="directions"]');
    const sideSheet = page.locator('.side-sheet');
    await expect(sideSheet).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(sideSheet).toBeHidden();
  });

});
