"""Narrow Bedrock adapter for the validated Voice Map Control boundary."""

from __future__ import annotations

import re
import os
from dataclasses import dataclass
from typing import Any, Protocol

from sthira_v2.config import V2Settings

NEMOTRON_MODEL_ID = "nvidia.nemotron-super-3-120b"
_THINK_RE = re.compile(r"<think>.*?</think>", re.DOTALL | re.IGNORECASE)


class BedrockRuntime(Protocol):
    def converse(self, **kwargs: Any) -> dict[str, Any]: ...


@dataclass(frozen=True)
class NemotronResponse:
    text: str
    stop_reason: str | None


class NemotronUnavailable(RuntimeError):
    """Raised when this optional demo interpreter is intentionally disabled."""


class BedrockNemotron:
    """Call Bedrock Converse without giving the model any operational authority."""

    def __init__(self, settings: V2Settings, client: BedrockRuntime | None = None) -> None:
        self._settings = settings
        self._client = client

    @property
    def enabled(self) -> bool:
        return self._settings.nemotron_enabled

    def _runtime_client(self) -> BedrockRuntime:
        if self._client is not None:
            return self._client
        try:
            import boto3
            from botocore.config import Config
        except ImportError as exc:  # keeps non-AI local development usable
            raise NemotronUnavailable("boto3 is not installed") from exc
        bearer_token = os.getenv("AWS_BEARER_TOKEN_BEDROCK")
        bearer_prefix = os.getenv("STHIRA_BEDROCK_API_KEY_PREFIX", "")
        if bearer_token and bearer_prefix and not bearer_token.startswith(bearer_prefix):
            os.environ["AWS_BEARER_TOKEN_BEDROCK"] = f"{bearer_prefix}{bearer_token}"
        return boto3.client(
            "bedrock-runtime",
            region_name=self._settings.aws_region,
            config=Config(connect_timeout=10, read_timeout=60, retries={"max_attempts": 2}),
        )

    def interpret(self, *, system_prompt: str, user_prompt: str, max_tokens: int = 200, temperature: float = 0.6) -> NemotronResponse:
        """Return text only; callers must validate it before any map action is applied."""
        if not self.enabled:
            raise NemotronUnavailable("Nemotron is disabled; set STHIRA_NEMOTRON_ENABLED=true to enable it")
        response = self._runtime_client().converse(
            modelId=self._settings.nemotron_model_id,
            system=[{"text": system_prompt}],
            messages=[{"role": "user", "content": [{"text": user_prompt}]}],
            inferenceConfig={"maxTokens": max_tokens, "temperature": temperature},
        )
        blocks = response["output"]["message"]["content"]
        raw_text = "".join(block.get("text", "") for block in blocks)
        return NemotronResponse(
            text=_THINK_RE.sub("", raw_text).strip(),
            stop_reason=response.get("stopReason"),
        )
