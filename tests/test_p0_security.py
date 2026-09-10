"""P0 security, outbox retry, and degraded-mode mutation proofs (REM-001, REM-002, REM-005, REM-007)."""

from fastapi.testclient import TestClient

from sthira.api.app import app
from sthira.core.identity import PROTOTYPE_PASSWORD, get_prototype_user, issue_access_token
from sthira.core.outbox import OutboxStatus, TransactionalOutbox
from sthira.modules.live_ops.service import degraded_mode_controller, step_up_auth_manager
from tests.authutil import authed_client, issue_step_up


def test_unauthenticated_non_public_route_is_401():
    client = TestClient(app)
    resp = client.get("/api/v1/programme/PRG-KL-WYD-2024")
    assert resp.status_code == 401


def test_health_and_login_remain_public():
    client = TestClient(app)
    assert client.get("/health").status_code == 200
    bad = client.post("/api/v1/auth/login", json={"username": "collector.wayanad", "password": "wrong"})
    assert bad.status_code == 401
    ok = client.post(
        "/api/v1/auth/login",
        json={"username": "collector.wayanad", "password": PROTOTYPE_PASSWORD},
    )
    assert ok.status_code == 200
    assert ok.json()["data"]["token_type"] == "bearer"
    assert ok.json()["data"]["user"]["username"] == "collector.wayanad"


def test_expired_and_forged_tokens_are_rejected():
    client = TestClient(app)
    expired = issue_access_token(get_prototype_user("collector.wayanad"), ttl_seconds=-10)
    resp = client.get("/api/v1/catalog/sources", headers={"Authorization": f"Bearer {expired}"})
    assert resp.status_code == 401
    resp = client.get("/api/v1/catalog/sources", headers={"Authorization": "Bearer totally-forged"})
    assert resp.status_code == 401


def test_forged_mfa_prefix_cannot_approve():
    client = authed_client()
    resp = client.post(
        "/api/v1/governance/approvals",
        json={
            "entity_type": "SITE_SELECTION",
            "entity_id": "SITE-FORGE-01",
            "entity_version": "v1.0",
            "approving_officer_name": "Attacker",
            "approving_officer_designation": "Nobody",
            "statutory_authority_basis": "none",
            "approval_order_number": "FORGED",
            "step_up_token": "MFA-STEPUP-anything",
        },
    )
    assert resp.status_code == 403
    assert "OFFICIALLY_APPROVED" not in resp.text


def test_step_up_is_bound_to_principal_action_and_consumed():
    user = get_prototype_user("collector.wayanad")
    tok = step_up_auth_manager.issue_step_up_token(user, "APPROVE_DECISION", valid_seconds=60)
    assert tok.nonce
    assert step_up_auth_manager.verify_step_up(tok.token_id, user.user_id, "APPROVE_DECISION") is True
    assert step_up_auth_manager.verify_step_up(tok.token_id, user.user_id, "APPROVE_DECISION") is False
    other = step_up_auth_manager.issue_step_up_token(user, "APPROVE_DECISION", valid_seconds=60)
    assert step_up_auth_manager.verify_step_up(other.token_id, "other_user", "APPROVE_DECISION") is False
    assert step_up_auth_manager.verify_step_up(other.token_id, user.user_id, "BREAK_GLASS") is False
    assert step_up_auth_manager.verify_step_up(other.token_id, user.user_id, "APPROVE_DECISION") is True


def test_outbox_retries_failed_then_dead_letters_and_reconciles():
    box = TransactionalOutbox()
    failing = {"on": True}

    def flaky(msg):
        if failing["on"]:
            raise RuntimeError("broker down")

    box.register_handler("notify", flaky)
    msg = box.enqueue("notify", {"k": 1}, idempotency_key="k1")
    msg.max_retries = 3

    assert box.relay_pending() == 0
    assert msg.status == OutboxStatus.FAILED
    assert box.relay_pending() == 0
    assert msg.status == OutboxStatus.FAILED
    assert box.relay_pending() == 0
    assert msg.status == OutboxStatus.DEAD_LETTER
    assert box.get_dead_letters()[0].message_id == msg.message_id

    failing["on"] = False
    assert box.reconcile_dead_letters() == 1
    assert box.relay_pending() == 1
    assert msg.status == OutboxStatus.PUBLISHED


def test_degraded_mode_blocks_approval_mutation():
    client = authed_client()
    client.post(
        "/api/v1/recovery/degraded-mode",
        json={"engage": True, "reason": "broker outage drill", "actor_id": "ops"},
    )
    try:
        step = client.post("/api/v1/auth/step-up", json={"action": "APPROVE_DECISION"})
        assert step.status_code == 503
        resp = client.post(
            "/api/v1/governance/approvals",
            json={
                "entity_type": "SITE_SELECTION",
                "entity_id": "SITE-DEG-01",
                "entity_version": "v1.0",
                "approving_officer_name": "Collector",
                "approving_officer_designation": "DDMA",
                "statutory_authority_basis": "DM Act",
                "approval_order_number": "X",
                "step_up_token": "unused",
            },
        )
        assert resp.status_code == 503
    finally:
        client.post("/api/v1/recovery/degraded-mode", json={"engage": False, "actor_id": "ops"})
        assert degraded_mode_controller.is_degraded is False


def test_wrong_role_with_valid_step_up_cannot_approve():
    analyst = get_prototype_user("analyst.wayanad")
    client = authed_client("analyst.wayanad")
    token = issue_step_up(analyst)
    resp = client.post(
        "/api/v1/governance/approvals",
        json={
            "entity_type": "SITE_SELECTION",
            "entity_id": "SITE-ROLE-01",
            "entity_version": "v1.0",
            "approving_officer_name": "Analyst",
            "approving_officer_designation": "GIS",
            "statutory_authority_basis": "none",
            "approval_order_number": "NO",
            "step_up_token": token,
        },
    )
    assert resp.status_code == 403
