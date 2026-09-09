from fastapi.testclient import TestClient

from punarvas.api.app import app
from punarvas.core.identity import get_prototype_user, issue_access_token
from punarvas.core.contracts import UserContext
from punarvas.modules.live_ops.service import step_up_auth_manager


def authed_client(username: str = "collector.wayanad") -> TestClient:
    client = TestClient(app)
    token = issue_access_token(get_prototype_user(username))
    client.headers.update({"Authorization": f"Bearer {token}"})
    return client


def issue_step_up(user: UserContext, action: str = "APPROVE_DECISION") -> str:
    return step_up_auth_manager.issue_step_up_token(user, action).token_id
