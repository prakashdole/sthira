import pytest

from punarvas.modules.live_ops.service import degraded_mode_controller


@pytest.fixture(autouse=True)
def reset_degraded_mode():
    if degraded_mode_controller.is_degraded:
        degraded_mode_controller.disengage_degraded_mode("test_teardown")
    yield
    if degraded_mode_controller.is_degraded:
        degraded_mode_controller.disengage_degraded_mode("test_teardown")
