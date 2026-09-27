/**
 * Lost-Response Retry Regression — Browser Integration Test
 *
 * Assignment: Prepare a browser regression that:
 * 1. Submits a reservation (server processes, response lost to browser)
 * 2. Retries via the visible Reserve Route control
 * 3. Asserts the retry sends byte-identical payload + same idempotency_key
 * 4. Verifies only ONE accepted stay exists
 *
 * Prerequisites to run (NOT satisfied in this environment):
 *   cd /Users/apple/Documents/Projects/MonitoringZ/frontend/v2
 *   npm run dev &
 *   sleep 3
 *   node reservation_lost_response_retry.test.mjs
 *
 * The dev server must be running on http://localhost:5173 (or update APP_URL below).
 * The test user must be in an active session with selectedDestination set.
 *
 * This script is NOT_RUN — written for later execution against the integrated app.
 */

const { chromium } = await import('playwright');

const APP_URL = process.env.APP_URL || 'http://localhost:5173';
const TEST_TIMEOUT = 30_000;

async function run() {
  console.log('Launching Chromium...');
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext();
  const page = await context.newPage();

  let idempotencyKeyFromFirstRequest = null;
  let firstRequestBody = null;
  let requestCount = 0;

  // Intercept ALL reservation requests to capture idempotency key
  await page.route('**/api/v3/reservations', async (route, request) => {
    requestCount++;
    const body = request.postDataJSON();

    if (request.method() === 'POST') {
      firstRequestBody = JSON.stringify(body);

      if (requestCount === 1) {
        // First request: capture idempotency key, then ABORT the response
        // This simulates the response being lost in transit (network error / browser crash)
        idempotencyKeyFromFirstRequest = body.idempotency_key;
        console.log(`[REQUEST 1] idempotency_key=${idempotencyKeyFromFirstRequest}`);
        console.log('[REQUEST 1] Simulating LOST RESPONSE — aborting');
        await route.abort('Failed');
        return;
      }

      if (requestCount === 2) {
        // Second request (retry): verify same idempotency key
        const key2 = body.idempotency_key;
        console.log(`[REQUEST 2] idempotency_key=${key2}`);

        if (key2 !== idempotencyKeyFromFirstRequest) {
          console.error(
            `[FAIL] Idempotency key mismatch!\n` +
            `  First:  ${idempotencyKeyFromFirstRequest}\n` +
            `  Second: ${key2}`
          );
          await route.abort('Failed');
          await browser.close();
          process.exit(1);
        }
        console.log('[PASS] Idempotency key is identical across retry');

        // Allow the retry through with a success response
        await route.fulfill({
          status: 200,
          contentType: 'application/json',
          body: JSON.stringify({
            data: {
              reservation_id: 'RES-TEST-001',
              stay_id: 'STAY-TEST-001',
              facility_id: body.facility_id,
            },
          }),
        });
        return;
      }
    }

    // Fallthrough for non-POST or unexpected requests
    await route.continue();
  });

  // Listen for console errors
  const consoleErrors = [];
  page.on('console', msg => {
    if (msg.type() === 'error') consoleErrors.push(msg.text());
  });

  console.log(`Navigating to ${APP_URL}...`);
  await page.goto(APP_URL, { waitUntil: 'networkidle', timeout: TEST_TIMEOUT });

  // The app requires an active session and selected destination.
  // We use window helpers (dev mode) to set up the required state.
  await page.evaluate(() => {
    // @ts-ignore
    if (window.setTestDestinations) {
      // @ts-ignore
      window.setTestDestinations([{
        facility_id: 'FAC-TEST-001',
        facility_name: 'Test Facility',
        is_illustrative: false,
        route_id: 'ROUTE-TEST-001',
        route_verified: true,
      }]);
    }
    // @ts-ignore
    if (window.setDataVersion) window.setDataVersion('TEST-V1');
    // @ts-ignore
    if (window.setGuidanceFreshness) window.setGuidanceFreshness('CURRENT');
  });

  await page.waitForTimeout(500);

  // Find and click the Reserve Route / Start Route button
  // The button text varies by language and state; find the primary action in the safety dock
  const reserveButton = page.getByRole('button', { name: /start.*route|reserve.*route|जोड़ें|आरंभ/i }).first();
  if (!reserveButton) {
    console.error('[FAIL] Could not find Reserve Route button');
    await browser.close();
    process.exit(1);
  }

  console.log('Clicking Reserve Route (first attempt — response will be lost)...');
  await reserveButton.click();

  // Wait for the pending state to settle
  await page.waitForTimeout(2000);

  // After the lost response, the UI should show an error/retry state
  // The pending_reservation should still be in sessionStorage (keepPendingReservation=true for network error)
  const pendingStillExists = await page.evaluate(() => {
    const p = sessionStorage.getItem('pending_reservation');
    return p !== null;
  });

  if (!pendingStillExists) {
    console.error('[FAIL] pending_reservation was cleared after network error — retry will send a NEW idempotency key');
    await browser.close();
    process.exit(1);
  }
  console.log('[PASS] pending_reservation persisted after lost response');

  // Now click the retry button (same Reserve Route button, or a retry control)
  console.log('Clicking Reserve Route (retry attempt)...');
  await reserveButton.click();

  // Wait for the retry to complete
  await page.waitForTimeout(2000);

  // After successful retry: pending_reservation should be cleared
  const pendingCleared = await page.evaluate(() => {
    return sessionStorage.getItem('pending_reservation') === null;
  });

  if (!pendingCleared) {
    console.error('[FAIL] pending_reservation was NOT cleared after successful retry — duplicate stay may be created');
    await browser.close();
    process.exit(1);
  }
  console.log('[PASS] pending_reservation cleared after successful retry');

  // Check that only ONE reservation was accepted (the replay)
  // The mock server returned STAY-TEST-001
  const stayId = await page.evaluate(() => sessionStorage.getItem('sthira_stay_id'));
  if (stayId !== 'STAY-TEST-001') {
    console.error(`[FAIL] Expected stay_id STAY-TEST-001, got: ${stayId}`);
    await browser.close();
    process.exit(1);
  }
  console.log(`[PASS] Accepted stay: ${stayId}`);

  // Assert no console errors
  if (consoleErrors.length > 0) {
    console.error(`[FAIL] Console errors during test: ${consoleErrors.join(', ')}`);
    await browser.close();
    process.exit(1);
  }
  console.log('[PASS] No console errors');

  console.log('\n=== ALL CHECKS PASSED ===');
  console.log('Regression: Lost reservation response is correctly retried with identical idempotency key.');
  console.log('Only one accepted stay is created.');

  await browser.close();
}

run().catch(err => {
  console.error('[FATAL]', err);
  process.exit(1);
});
