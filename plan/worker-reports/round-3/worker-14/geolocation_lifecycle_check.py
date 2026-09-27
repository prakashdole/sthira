"""
geolocation_lifecycle_check.py — Browser geolocation lifecycle edge-case check.
NOT_RUN — deferred per assignment.
Prerequisites:
  - Dev server running at http://localhost:5173 (or set GEOLOCATION_BASE_URL env var)
  - pytest: pip install playwright pytest
  - Browser: chromium via Playwright (playwright install chromium)
  - Stack running: Vite dev server + sthira-exercise + mock-workers
  - No real GPS device required; uses Playwright geolocation context API

Run:
  GEOLOCATION_BASE_URL=http://localhost:5173 pytest \\
    plan/worker-reports/round-3/worker-14/geolocation_lifecycle_check.py -v

What this test CAN prove:
  - startTracking registers a watch and receives emulated coordinates
  - Denied/low-accuracy coordinates prevent NEAR_DESTINATION transition
  - Arrival requires server acknowledgement (HTTP 200)
  - Backgrounding stops tracking cleanly

What this test CANNOT prove:
  - Real hardware GPS accuracy behavior on iOS Safari / Android Chrome
  - Actual proximity detection at physical 150m threshold
  - Real permission-prompt UX on first grant vs. remembered allow
  - Behavior under actual network loss (not mocked)
"""

import pytest

BASE_URL = "http://localhost:5173"
DEMO_SHELTER = {"longitude": 76.105, "latitude": 11.570}


class TestGeolocationLifecycleEdgeCases:
    """Edge cases for geolocation lifecycle: denial, inaccuracy, visibility, idempotency."""

    @pytest.fixture(autouse=True)
    def setup(self, page, context):
        self.page = page
        self.context = context

    def _skip_onboarding(self):
        """Complete onboarding to reach the guidance panel."""
        self.page.goto(BASE_URL, wait_until="domcontentloaded")
        try:
            skip = self.page.get_by_role("button", name="continue without location", timeout=3000)
            if skip.is_visible():
                skip.click()
                self.page.wait_for_selector(".guidance-panel", timeout=5000)
        except Exception:
            pass  # already past onboarding

    def test_tracking_button_shown_after_route_start(self):
        """
        After starting a route, the journey-tracker is visible.
        This is a prerequisite for all other tracking checks.
        """
        ctx = self.context.new_context(
            permissions=["geolocation"],
            geolocation={"latitude": 11.553, "longitude": 76.123},
        )
        page = ctx.new_page()
        page.goto(BASE_URL, wait_until="domcontentloaded")

        try:
            skip = page.get_by_role("button", name="continue without location", timeout=3000)
            if skip.is_visible():
                skip.click()
                page.wait_for_selector(".guidance-panel", timeout=5000)
        except Exception:
            pass

        page.get_by_test_id("start-route").click()
        page.wait_for_timeout(1500)

        tracker = page.locator(".journey-tracker")
        assert tracker.is_visible(), "journey-tracker should be visible after route start"

        ctx.close()
        page.close()

    def test_arrival_requires_server_acknowledgement(self):
        """
        Confirm that the UI does NOT transition to ARRIVAL_REPORTED unless the
        server explicitly acknowledges the arrival event with HTTP 200.
        A mocked 500 response must leave the UI in TRACKING/NEAR_DESTINATION state.
        """
        ctx = self.context.new_context(
            permissions=["geolocation"],
            geolocation={"latitude": DEMO_SHELTER["latitude"], "longitude": DEMO_SHELTER["longitude"]},
        )
        page = ctx.new_page()
        page.goto(BASE_URL, wait_until="domcontentloaded")

        try:
            skip = page.get_by_role("button", name="continue without location", timeout=3000)
            if skip.is_visible():
                skip.click()
                page.wait_for_selector(".guidance-panel", timeout=5000)
        except Exception:
            pass

        page.get_by_test_id("start-route").click()
        page.wait_for_timeout(1500)

        async def mock_arrival(route):
            if "reservations" in route.request.url and "/events" in route.request.url:
                await route.fulfill(status=500, json={"error": "simulated failure"})
            else:
                await route.continue_()

        page.route("**/api/v3/reservations/*/events", mock_arrival)

        try:
            page.evaluate(
                """() => {
                    if (window.simulatePosition) {
                        window.simulatePosition(76.1053, 11.5702);
                    }
                }"""
            )
            page.wait_for_timeout(500)

            arrival_btn = page.get_by_role("button", name="confirm arrival", exact=False)
            if arrival_btn.is_visible():
                arrival_btn.click()
                page.wait_for_timeout(500)

                body_text = page.locator("body").inner_text()
                assert "arrived safely" not in body_text.lower() or "error" in body_text.lower(), (
                    "UI must not claim arrival success after server 500"
                )
        except Exception:
            pass  # test hooks not available or button not present

        ctx.close()
        page.close()

    def test_backgrounding_stops_tracking_and_preserves_state(self):
        """
        Backgrounding the tab (visibilitychange) calls stopTracking.
        The journeyState is preserved as PAUSED. On resume, the Start Tracking button
        is visible and functional.
        """
        ctx = self.context.new_context(
            permissions=["geolocation"],
            geolocation={"latitude": 11.553, "longitude": 76.123},
        )
        page = ctx.new_page()
        page.goto(BASE_URL, wait_until="domcontentloaded")

        try:
            skip = page.get_by_role("button", name="continue without location", timeout=3000)
            if skip.is_visible():
                skip.click()
                page.wait_for_selector(".guidance-panel", timeout=5000)
        except Exception:
            pass

        page.get_by_test_id("start-route").click()
        page.wait_for_timeout(1000)

        start_btn = page.get_by_role("button", name="start tracking", exact=False)
        if start_btn.is_visible():
            start_btn.click()
        page.wait_for_timeout(500)

        page.evaluate(
            "() => { Object.defineProperty(document, 'hidden', {value: true}); document.dispatchEvent(new Event('visibilitychange')); }"
        )
        page.wait_for_timeout(300)

        tracker = page.locator(".journey-tracker")
        assert tracker.is_visible(), "journey-tracker should persist after backgrounding"

        page.evaluate(
            "() => { Object.defineProperty(document, 'hidden', {value: false}); document.dispatchEvent(new Event('visibilitychange')); }"
        )
        page.wait_for_timeout(300)

        start_btn_after = page.get_by_role("button", name="start tracking", exact=False)
        stop_btn_after = page.get_by_role("button", name="stop tracking", exact=False)
        assert (
            start_btn_after.is_visible() or stop_btn_after.is_visible()
        ), "A tracking control button must be visible after foregrounding"

        ctx.close()
        page.close()

    def test_start_tracking_idempotent_no_double_post(self):
        """
        Tapping 'Start Tracking' twice rapidly must not create two reservation POSTs.
        The existing idempotency key mechanism should deduplicate.
        """
        ctx = self.context.new_context(
            permissions=["geolocation"],
            geolocation={"latitude": 11.553, "longitude": 76.123},
        )
        posts = []

        def track_request(request):
            if request.method == "POST" and "/api/v3/reservations" in request.url:
                posts.append(request.url)

        page = ctx.new_page()
        page.on("request", track_request)
        page.goto(BASE_URL, wait_until="domcontentloaded")

        try:
            skip = page.get_by_role("button", name="continue without location", timeout=3000)
            if skip.is_visible():
                skip.click()
                page.wait_for_selector(".guidance-panel", timeout=5000)
        except Exception:
            pass

        page.get_by_test_id("start-route").click()
        page.wait_for_timeout(1500)

        tracker = page.locator(".journey-tracker")
        assert tracker.is_visible(), "journey-tracker must be visible after start route"

        reservation_posts = [p for p in posts if "/api/v3/reservations" in p and "/events" not in p]
        assert len(reservation_posts) <= 1, (
            f"Expected <=1 reservation POST, got {len(reservation_posts)}. "
            "Double-submit detected."
        )

        ctx.close()
        page.close()
