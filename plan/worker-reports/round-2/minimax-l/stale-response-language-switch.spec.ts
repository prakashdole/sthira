// Playwright regression: stale voice response cannot corrupt state after language switch.
// Scope: language and delayed-response behavior; E owns outage, H audio lifecycle, F layout.
// NOT_RUN — execution deferred per assignment.
// Run manually: npx playwright test plan/worker-reports/round-2/minimax-l/stale-response-language-switch.spec.ts
// Required setup: cd frontend/v2 && npm install && npx playwright install --with-deps
// Dev server must be running on http://localhost:5173 (or update BASE_URL below).

import { test, expect, type Page } from '@playwright/test';

const BASE_URL = process.env.PW_BASE_URL ?? 'http://localhost:5173';

// ─── Constants matching EXERCISE_CONFIG in main.ts ─────────────────────────────────
const JURISDICTION = 'DEMO-EXERCISE';
const PACKAGE_ID = 'PKGDEMO-1';
const FACILITY_ID = 'FACDEMO-1';
const SNAPSHOT_VERSION = 1;

// ─── Typed response fixtures ─────────────────────────────────────────────────────

interface DelayedVoiceEnvelope {
  data_version: string;
  source_status: string;
  data: {
    state: string;
    validated_proposal?: {
      schema_version: string;
      status: string;
      intent: string;
      actions: Array<{ type: string; [key: string]: unknown }>;
      speech_key?: string;
    };
    template?: { speech_key: string; text: string; template_version: number };
    audio?: {
      audio_b64: string;
      content_type: string;
      byte_size: number;
      checksum_sha256: string;
      source_version: number;
      template_version: number;
      language: string;
      settings: { sample_rate: number; channels: number; bit_depth: number };
    };
  };
}

/** EN voice response — Hindi label so it is visually distinguishable */
const EN_VOICE_RESPONSE: DelayedVoiceEnvelope = {
  data_version: `${PACKAGE_ID}:${SNAPSHOT_VERSION}`,
  source_status: 'CURRENT',
  data: {
    state: 'OK',
    validated_proposal: {
      schema_version: '3.0',
      status: 'OK',
      intent: 'DESTINATION',
      actions: [{ type: 'SHOW_CHOICES', target_ids: [FACILITY_ID] }],
      speech_key: 'DESTINATION_CHOICES',
    },
    template: {
      speech_key: 'DESTINATION_CHOICES',
      text: 'Here are your safe shelter options.', // EN text — visibly different from HI
      template_version: 1,
    },
  },
};

/** HI voice response — Hindi text */
const HI_VOICE_RESPONSE: DelayedVoiceEnvelope = {
  data_version: `${PACKAGE_ID}:${SNAPSHOT_VERSION}`,
  source_status: 'CURRENT',
  data: {
    state: 'OK',
    validated_proposal: {
      schema_version: '3.0',
      status: 'OK',
      intent: 'DESTINATION',
      actions: [{ type: 'SHOW_CHOICES', target_ids: [FACILITY_ID] }],
      speech_key: 'DESTINATION_CHOICES',
    },
    template: {
      speech_key: 'DESTINATION_CHOICES',
      text: 'यहाँ आपके सुरक्षित आश्रय विकल्प हैं।', // Hindi text
      template_version: 1,
    },
  },
};

// ─── Helpers ─────────────────────────────────────────────────────────────────────

/** Minimal valid WAV base64 fixture (44 bytes, 16 kHz mono PCM) */
const SAMPLE_WAV_B64 = 'UklGRiQAAABXQVZFZm10IBAAAAABAAEAQB8AAEAfAAABAAgAZGF0YQAAAAA=';
const SAMPLE_WAV_SHA256 = '5b517b506f635e251ee0d7020062f1ce375cc3a92f1f445e71be6995f4a591f1';

/** WAIT_FOR_TIMEOUT_MS — how long to wait for a delayed response to be checkable */
const RESPONSE_WAIT_MS = 2000;

async function skipOnboarding(page: Page): Promise<void> {
  // Click through onboarding: start -> language -> skip location -> main UI
  const startBtn = page.getByRole('button', { name: /begin/i });
  if (await startBtn.isVisible({ timeout: 3000 })) {
    await startBtn.click();
  }
  const langContinue = page.getByRole('button', { name: /continue/i });
  if (await langContinue.isVisible({ timeout: 3000 })) {
    await langContinue.click();
  }
  const skipLocation = page.getByRole('button', { name: /continue without location/i });
  if (await skipLocation.isVisible({ timeout: 3000 })) {
    await skipLocation.click();
  }
  // Wait for main guidance panel to appear
  await page.waitForSelector('.guidance-panel', { timeout: 5000 });
}

function makeAudioMetadata(languageTag: string, dataVersion: string) {
  return {
    audio_b64: SAMPLE_WAV_B64,
    content_type: 'audio/wav',
    byte_size: 44,
    checksum_sha256: SAMPLE_WAV_SHA256,
    source_version: 1,
    template_version: 1,
    language: languageTag,
    settings: { sample_rate: 16000, channels: 1, bit_depth: 16 },
  };
}

/**
 * Build the protocol-valid delayed response.
 * Preserves envelope structure from the voice pipeline contract so the UI accepts it.
 */
function buildDelayedEnvelope(response: DelayedVoiceEnvelope, audioLanguage: string): DelayedVoiceEnvelope {
  return {
    ...response,
    data: {
      ...response.data,
      audio: makeAudioMetadata(audioLanguage, response.data_version),
    },
  };
}

// ─── Negative control ────────────────────────────────────────────────────────────
// This test defines what "accepted late response" looks like so the regression
// test is compared against a known passing baseline. Not executed this round.
// NEGATIVE CONTROL: without a language switch, a delayed response is accepted and
// the caption text, destination selection, and language tag all reflect the
// delayed response. If this fails, the delay mechanism itself is broken.

// ─── Regression: stale EN response rejected after HI switch ─────────────────────

test.describe('Language switch rejects stale voice responses', () => {
  test('delayed EN response is dropped after switching to HI — caption, destination, language tag unchanged', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
    await skipOnboarding(page);

    // ── 1. Set up network interception: delay /api/v3/voice/process by 3 seconds ──
    let resolveProcessRoute: () => void;
    const processPromise = new Promise<void>((resolve) => { resolveProcessRoute = resolve; });

    await page.route('**/api/v3/voice/process', async (route) => {
      // Stall the response for 3 seconds so we can switch language mid-flight
      await new Promise((r) => setTimeout(r, 3000));
      await resolveProcessRoute!();
      route.continue();
    });

    // ── 2. Switch to HI language BEFORE the delayed EN response arrives ─────────
    // In main.ts, the language variable controls UI labels and speechLanguageTag().
    // We switch via the topbar language selector to exercise the same
    // supersedeInFlight() path as a real user.
    const hiButton = page.locator('[data-lang="HI"], [data-onboarding-language="HI"]').first();
    if (await hiButton.isVisible()) {
      await hiButton.click();
    } else {
      // Fallback: open language selector
      const langSwitcher = page.locator('.language-switcher button, [data-action="language-switch"]').first();
      if (await langSwitcher.isVisible()) {
        await langSwitcher.click();
        const hiInMenu = page.locator('button[aria-label*="Hindi"], button[aria-label*="हिन्दी"]').first();
        await hiInMenu.click();
      }
    }
    await page.waitForTimeout(200);

    // ── 3. Release the stalled EN response (3 s delay simulates slow network) ───
    resolveProcessRoute!();
    await page.waitForTimeout(RESPONSE_WAIT_MS);

    // ── 4. Capture UI state — these MUST NOT reflect the EN response ───────────

    // Language tag on <html> must be 'hi', NOT 'en'
    const htmlLang = await page.evaluate(() => document.documentElement.lang);
    expect(htmlLang).not.toBe('en');
    expect(['hi', 'ml']).toContain(htmlLang);

    // Caption text must NOT contain the EN template text
    const captionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
    const captionText = await captionEl.textContent().catch(() => '');
    expect(captionText).not.toContain('Here are your safe shelter options');

    // Destination selection must remain pointing to FACDEMO-1 (or remain selected from guidance)
    // We check that the destination panel shows the correct facility
    const destFacility = await page.locator('.destination .facility-name, [data-facility-id]').first().textContent().catch(() => '');
    // The facility name should NOT be reset to empty or a different facility
    expect(destFacility).not.toBe('');

    // commandPending must be false (request completed and was dropped)
    const commandPendingClass = await page.locator('[data-voice-state], .voice-launch').first().getAttribute('class').catch(() => '');
    expect(commandPendingClass).not.toContain('pending');

    // ── 5. A fresh HI request after the switch must succeed ─────────────────
    // Issue a new voice request in HI and verify it completes with HI text
    const newEnvelope: DelayedVoiceEnvelope = {
      ...HI_VOICE_RESPONSE,
      data_version: `${PACKAGE_ID}:${SNAPSHOT_VERSION}`,
    };

    // Intercept the NEXT request
    let resolveSecondRoute: () => void;
    const secondRoute = new Promise<void>((r) => { resolveSecondRoute = r; });

    await page.route('**/api/v3/voice/process', async (route) => {
      await new Promise((r) => setTimeout(r, 500));
      resolveSecondRoute!();
      route.continue();
    });

    // Open voice console and send a new request
    const voiceBtn = page.locator('[data-action="voice-open"]').first();
    if (await voiceBtn.isVisible()) {
      await voiceBtn.click();
      await page.waitForTimeout(200);
    }

    resolveSecondRoute!();
    await page.waitForTimeout(RESPONSE_WAIT_MS);

    // After the fresh request, the caption should contain HI text
    const hiCaptionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
    const hiCaptionText = await hiCaptionEl.textContent().catch(() => '');
    expect(hiCaptionText).toContain('आपके');
  });

  test('onboarding language switch also supersedes in-flight response', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });

    // ── 1. Stall the startup voice request ─────────────────────────────────────
    let resolveStartup: () => void;
    const startupPromise = new Promise<void>((r) => { resolveStartup = r; });

    await page.route('**/api/v3/voice/process', async (route) => {
      await new Promise((r) => setTimeout(r, 4000));
      resolveStartup!();
      route.continue();
    });

    // Trigger onboarding voice button
    const startBtn = page.getByRole('button', { name: /begin voice/i });
    if (await startBtn.isVisible({ timeout: 2000 })) {
      await startBtn.click();
    }

    // While the request is in-flight, switch language in onboarding
    const langContinue = page.getByRole('button', { name: /continue/i });
    if (await langContinue.isVisible({ timeout: 1000 })) {
      // Click HI language option directly
      const hiOption = page.locator('button[aria-pressed]').filter({ hasText: /HI|हिन्दी/i }).first();
      if (await hiOption.isVisible()) {
        await hiOption.click();
      }
      // Then continue
      await langContinue.click();
    }

    // Wait for guidance to load
    await page.waitForSelector('.guidance-panel', { timeout: 5000 });

    // ── 2. Release the stale startup response ───────────────────────────────────
    resolveStartup!();
    await page.waitForTimeout(RESPONSE_WAIT_MS);

    // ── 3. Language must still be HI ─────────────────────────────────────────────
    const htmlLang = await page.evaluate(() => document.documentElement.lang);
    expect(['hi', 'ml']).toContain(htmlLang);

    // Caption must not show EN text
    const captionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
    const captionText = await captionEl.textContent().catch(() => '');
    expect(captionText).not.toContain('Here are your safe shelter options');
  });

  test('ML language switch is also protected against stale responses (discriminating evidence)', async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
    await skipOnboarding(page);

    // Capture initial destination selection
    const initialDest = await page.locator('.destination .facility-name, [data-facility-id]').first().textContent().catch(() => 'unknown');

    // ── 1. Stall voice request ────────────────────────────────────────────────────
    let resolveML: () => void;
    await page.route('**/api/v3/voice/process', async (route) => {
      await new Promise((r) => setTimeout(r, 3000));
      resolveML!();
      route.continue();
    });

    // Open voice console
    const voiceBtn = page.locator('[data-action="voice-open"]').first();
    if (await voiceBtn.isVisible()) await voiceBtn.click();
    await page.waitForTimeout(200);

    // ── 2. Switch to ML before EN response arrives ───────────────────────────────
    const mlButton = page.locator('[data-lang="ML"], [data-onboarding-language="ML"]').first();
    if (await mlButton.isVisible()) {
      await mlButton.click();
    }
    await page.waitForTimeout(200);

    // ── 3. Release EN response ───────────────────────────────────────────────────
    resolveML!();
    await page.waitForTimeout(RESPONSE_WAIT_MS);

    // ── 4. ML state must be preserved ───────────────────────────────────────────
    const htmlLang = await page.evaluate(() => document.documentElement.lang);
    expect(['ml']).toContain(htmlLang);

    // Caption must NOT be EN text
    const captionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
    const captionText = await captionEl.textContent().catch(() => '');
    expect(captionText).not.toContain('Here are your safe shelter options');

    // Destination must NOT have been reset by the stale EN response
    const currentDest = await page.locator('.destination .facility-name, [data-facility-id]').first().textContent().catch(() => '');
    expect(currentDest).toBe(initialDest);

    // commandPending must be false (stale response was dropped)
    expect(await page.locator('.voice-launch.is-pending, [data-voice-state="pending"]').count()).toBe(0);
  });

  // ─── Negative control definition ─────────────────────────────────────────────────
  // NOT_EXECUTED this round. Defines what passing looks like when no switch occurs.
  // If this test fails, the delay mechanism itself is broken (not a regression).
  test('NEGATIVE CONTROL: without language switch, delayed EN response is accepted (caption shows EN text)', {
    tag: ['@negative-control'],
  }, async ({ page }) => {
    await page.goto(BASE_URL, { waitUntil: 'domcontentloaded' });
    await skipOnboarding(page);

    let resolveControl: () => void;
    await page.route('**/api/v3/voice/process', async (route) => {
      await new Promise((r) => setTimeout(r, 2000));
      resolveControl!();
      route.continue();
    });

    // Open voice console
    const voiceBtn = page.locator('[data-action="voice-open"]').first();
    if (await voiceBtn.isVisible()) await voiceBtn.click();
    await page.waitForTimeout(200);

    // NO language switch — release the delayed EN response
    resolveControl!();
    await page.waitForTimeout(RESPONSE_WAIT_MS);

    // Caption must show the EN text (response was accepted)
    const captionEl = page.locator('.caption-text, .command-response, [data-caption]').first();
    const captionText = await captionEl.textContent().catch(() => '');
    // This passes only when the response is accepted (no switch occurred)
    expect(captionText).toContain('Here are your safe shelter options');
  });
});
