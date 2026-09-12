"""Azure OpenAI Responses adapter for validated Voice Map Control output."""

from __future__ import annotations

import os
import re

import httpx
from dotenv import load_dotenv

load_dotenv()

_THINK_RE = re.compile(r"<think>.*?</think>", re.DOTALL | re.IGNORECASE)


class AzureOpenAIUnavailable(RuntimeError):
    pass


class AzureOpenAIResponses:
    def __init__(self, client: httpx.Client | None = None) -> None:
        self._url = os.getenv("AZURE_OPENAI_RESPONSES_URL", "")
        self._key = os.getenv("AZURE_OPENAI_API_KEY", "")
        self._deployment = os.getenv("AZURE_OPENAI_DEPLOYMENT", "gpt-4.1-mini")
        self._client = client

    @property
    def configured(self) -> bool:
        return bool(self._url and self._key)

    def interpret(self, *, system_prompt: str, user_prompt: str, max_output_tokens: int = 200, temperature: float = 0.6) -> str:
        if not self.configured:
            raise AzureOpenAIUnavailable("Azure OpenAI is not configured")
        request = {
            "model": self._deployment,
            "instructions": system_prompt,
            "input": user_prompt,
            "max_output_tokens": max(16, max_output_tokens),
            "temperature": temperature,
        }
        if self._client is None:
            with httpx.Client(timeout=httpx.Timeout(60.0, connect=10.0)) as client:
                response = client.post(self._url, headers={"api-key": self._key}, json=request)
        else:
            response = self._client.post(self._url, headers={"api-key": self._key}, json=request)
        response.raise_for_status()
        payload = response.json()
        text = payload.get("output_text") or "".join(
            block.get("text", "")
            for message in payload.get("output", [])
            for block in message.get("content", [])
            if block.get("type") == "output_text"
        )
        return _THINK_RE.sub("", text).strip()
