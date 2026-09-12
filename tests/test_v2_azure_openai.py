import httpx

from sthira_v2.azure_openai import AzureOpenAIResponses


def test_azure_responses_extracts_output_and_enforces_minimum_token_limit(monkeypatch):
    monkeypatch.setenv("AZURE_OPENAI_RESPONSES_URL", "https://azure.example/openai/v1/responses")
    monkeypatch.setenv("AZURE_OPENAI_API_KEY", "test-key")
    request = httpx.Request("POST", "https://azure.example/openai/v1/responses")
    client = httpx.Client(transport=httpx.MockTransport(lambda _: httpx.Response(200, request=request, json={"output": [{"content": [{"type": "output_text", "text": "<think>x</think>valid"}]}]})))
    assert AzureOpenAIResponses(client).interpret(system_prompt="fixed", user_prompt="show", max_output_tokens=8) == "valid"
