from fastapi.testclient import TestClient

from sthira.api.app import app


def test_chat_returns_grounded_route_action_and_supports_follow_up():
    client = TestClient(app)
    first = client.post(
        "/api/v2/guidance/chat",
        json={
            "session_id": "browser-demo-session",
            "language": "EN",
            "messages": [{"role": "USER", "text": "How do I get to the shelter?"}],
        },
    )

    assert first.status_code == 200
    assert first.json()["map_action"] == "OPEN_DIRECTIONS"
    assert first.json()["source_status"] == "SYNTHETIC_DEMO"
    assert first.json()["provider"] == "LOCAL_GUIDANCE"

    follow_up = client.post(
        "/api/v2/guidance/chat",
        json={
            "session_id": "browser-demo-session",
            "language": "EN",
            "messages": [
                {"role": "USER", "text": "Show me the route"},
                {"role": "ASSISTANT", "text": first.json()["reply"]},
                {"role": "USER", "text": "Why?"},
            ],
        },
    )

    assert follow_up.status_code == 200
    assert follow_up.json()["map_action"] == "SHOW_ROUTE"
    assert "bridge" in follow_up.json()["reply"].lower()


def test_chat_handles_open_question_without_command_rejection():
    response = TestClient(app).post(
        "/api/v2/guidance/chat",
        json={
            "session_id": "browser-demo-session",
            "language": "EN",
            "messages": [{"role": "USER", "text": "Can you help me understand what is happening?"}],
        },
    )

    assert response.status_code == 200
    assert "general-purpose AI is temporarily unavailable" in response.json()["reply"]
    assert response.json()["map_action"] == "NONE"
    assert response.json()["provider"] == "LOCAL_GUIDANCE"


def test_chat_rejects_assistant_as_last_message():
    response = TestClient(app).post(
        "/api/v2/guidance/chat",
        json={
            "session_id": "browser-demo-session",
            "language": "EN",
            "messages": [{"role": "ASSISTANT", "text": "Hello"}],
        },
    )

    assert response.status_code == 422


def test_voice_status_reports_unconfigured_local_model(monkeypatch):
    monkeypatch.delenv("STHIRA_ASR_MODEL_DIR", raising=False)

    response = TestClient(app).get("/api/v2/voice/status")

    assert response.status_code == 200
    assert response.json()["data"]["provider"] == "LOCAL_AI4BHARAT"
    assert response.json()["data"]["ready"] is False
    assert response.json()["degraded"] is True
