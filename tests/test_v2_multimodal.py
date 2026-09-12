import pytest

from sthira_v2.multimodal import (
    ISLAssetStatus,
    TTSAdapter,
    TTSArtifactGate,
    TTSState,
    pending_isl_asset,
    validate_localization_catalog,
)


def test_tts_fails_closed_until_exact_artifact_gate_and_purges_cancelled_instruction():
    adapter = TTSAdapter(TTSArtifactGate(None, None, None, None, None, TTSState.BLOCKED_EXTERNAL, ()))
    with pytest.raises(RuntimeError, match="artifact gate"):
        adapter.synthesize("Demo instruction", language="en-IN", instruction_version="v1")

    ready = TTSAdapter(TTSArtifactGate("approved-demo-model", "rev-1", "approved-license", "runtime", "hardware", TTSState.READY, ("en-IN", "ml-IN")))
    entry = ready.cache_approved_audio("Demo instruction", b"audio", language="en-IN", instruction_version="v1")
    assert entry.content_hash
    assert ready.purge_instruction("v1") == 1
    assert ready.purge_instruction("v1") == 0
    with pytest.raises(ValueError, match="approved"):
        ready.cache_approved_audio("Demo", b"audio", language="hi-IN", instruction_version="v2")


def test_isl_is_honestly_pending_and_text_fallback_is_present():
    asset = pending_isl_asset()
    assert isinstance(asset, ISLAssetStatus)
    assert asset.state == "PENDING_APPROVAL"
    assert asset.asset_id is None
    assert asset.evidence_class == "SYNTHETIC_DEMO"
    assert asset.transcript


def test_localization_catalog_requires_complete_language_keys():
    required = {"headline", "summary", "route", "call"}
    validate_localization_catalog({"en-IN": {key: key for key in required}, "ml-IN": {key: key for key in required}}, required)
    with pytest.raises(ValueError, match="ml-IN"):
        validate_localization_catalog({"en-IN": {key: key for key in required}, "ml-IN": {"headline": "x"}}, required)
