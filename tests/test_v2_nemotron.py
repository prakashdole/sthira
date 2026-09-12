from sthira_v2.config import RuntimeProfile, V2Settings
from sthira_v2.nemotron import BedrockNemotron, NemotronUnavailable
from tests.authutil import authed_client


def settings(*, enabled: bool) -> V2Settings:
    return V2Settings(RuntimeProfile.DEMO, None, None, None, None, enabled, "us-east-1", "nvidia.nemotron-super-3-120b")


class FakeBedrock:
    def __init__(self):
        self.request = None

    def converse(self, **kwargs):
        self.request = kwargs
        return {"output": {"message": {"content": [{"text": '<think>private</think>{"status":"OK"}'}]}}, "stopReason": "end_turn"}


def test_nemotron_uses_converse_without_stop_sequences_and_strips_thinking():
    client = FakeBedrock()
    response = BedrockNemotron(settings(enabled=True), client).interpret(system_prompt="fixed", user_prompt="show alert")
    assert response.text == '{"status":"OK"}'
    assert client.request["modelId"] == "nvidia.nemotron-super-3-120b"
    assert client.request["inferenceConfig"] == {"maxTokens": 200, "temperature": 0.6}
    assert "stopSequences" not in client.request["inferenceConfig"]


def test_nemotron_is_opt_in():
    try:
        BedrockNemotron(settings(enabled=False), FakeBedrock()).interpret(system_prompt="fixed", user_prompt="show alert")
    except NemotronUnavailable:
        pass
    else:
        raise AssertionError("disabled integration must not call Bedrock")


def test_ai_provider_status_never_invokes_the_model():
    response = authed_client().get("/api/v2/ai/provider/status")
    assert response.status_code == 200
    data = response.json()["data"]
    assert data["provider"] == "AZURE_OPENAI"
    assert data["configured_model_id"] == "gpt-4.1-mini"
    assert data["requires_validated_map_actions"] is True
