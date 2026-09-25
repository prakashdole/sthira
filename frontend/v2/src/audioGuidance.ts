export type TTSSettings = {
  sample_rate_hz?: number;
  channels?: number;
  bit_depth?: number;
  codec?: string;
};

export type AudioMetadata = {
  audio_b64: string;
  content_type?: string;
  byte_size?: number;
  checksum_sha256?: string;
  source_version: number;
  data_version: string;
  template_key?: string;
  template_version: number;
  language: string;
  settings?: TTSSettings;
};

export type VoiceResponseEnvelope = {
  data_version?: string;
  source_status?: string;
  data?: {
    data_version?: string;
    validated_proposal?: unknown;
    template?: { speech_key?: string; text?: string; template_version?: number };
    audio?: {
      audio_b64?: string;
      content_type?: string;
      byte_size?: number;
      checksum_sha256?: string;
      source_version?: number;
      template_version?: number;
      language?: string;
      settings?: TTSSettings;
    };
    state?: string;
  };
};

export type ProcessedVoiceOutcome =
  | { kind: 'CLARIFY'; clarification_ids: string[]; template_text?: string }
  | { kind: 'ERROR' }
  | {
      kind: 'OK';
      proposal: unknown;
      template_text?: string;
      audio?: AudioMetadata;
      guidanceSpeechExpected: boolean;
      captionUnavailable: boolean;
    };

/**
 * Validates audio provenance strictly against envelope data_version, positive
 * source_version, positive template_version, and matching template version.
 * Invented defaults (such as source_version = 1 or fabricated data_version) are rejected.
 */
export function buildAudioMetadata(
  rawAudio: unknown,
  rawTemplate: unknown,
  authoritativeDataVersion: string | undefined
): AudioMetadata | undefined {
  if (!rawAudio || typeof rawAudio !== 'object') return undefined;
  const audio = rawAudio as Record<string, unknown>;
  if (typeof audio.audio_b64 !== 'string' || audio.audio_b64.trim().length === 0) {
    return undefined;
  }

  // Must have authoritative non-empty data_version from envelope
  if (typeof authoritativeDataVersion !== 'string' || authoritativeDataVersion.trim().length === 0) {
    return undefined;
  }

  // source_version must be number > 0
  const sourceVersion = audio.source_version;
  if (typeof sourceVersion !== 'number' || sourceVersion <= 0 || !Number.isInteger(sourceVersion)) {
    return undefined;
  }

  // template_version must be number > 0
  const templateVersion = audio.template_version;
  if (typeof templateVersion !== 'number' || templateVersion <= 0 || !Number.isInteger(templateVersion)) {
    return undefined;
  }

  // language must be non-empty string
  const language = audio.language;
  if (typeof language !== 'string' || language.trim().length === 0) {
    return undefined;
  }

  // If template is present with a template_version, it must match audio.template_version
  if (rawTemplate && typeof rawTemplate === 'object') {
    const tpl = rawTemplate as Record<string, unknown>;
    if (typeof tpl.template_version === 'number' && tpl.template_version > 0) {
      if (tpl.template_version !== templateVersion) {
        return undefined; // Template version mismatch
      }
    }
  }

  const templateKey =
    rawTemplate && typeof rawTemplate === 'object' && typeof (rawTemplate as Record<string, unknown>).speech_key === 'string'
      ? ((rawTemplate as Record<string, unknown>).speech_key as string)
      : undefined;

  return {
    audio_b64: audio.audio_b64,
    content_type: typeof audio.content_type === 'string' ? audio.content_type : undefined,
    byte_size: typeof audio.byte_size === 'number' ? audio.byte_size : undefined,
    checksum_sha256: typeof audio.checksum_sha256 === 'string' ? audio.checksum_sha256 : undefined,
    source_version: sourceVersion,
    data_version: authoritativeDataVersion,
    template_key: templateKey,
    template_version: templateVersion,
    language: language.trim(),
    settings: audio.settings as TTSSettings | undefined,
  };
}

/**
 * Checks whether audio metadata is valid for replay in the active UI context:
 * - Freshness must be CURRENT
 * - Active UI language must match audio language
 * - Active UI data_version must match audio data_version
 * - Positive source_version and template_version required
 */
export function isAudioValidForReplay(
  audio: AudioMetadata | undefined,
  guidanceFreshness: string,
  currentDataVersion: string,
  activeLanguageTag: string
): boolean {
  if (!audio || !audio.audio_b64) return false;
  if (guidanceFreshness !== 'CURRENT') return false;
  if (!audio.language || audio.language !== activeLanguageTag) return false;
  if (!audio.data_version || audio.data_version !== currentDataVersion) return false;
  if (typeof audio.source_version !== 'number' || audio.source_version <= 0) return false;
  if (typeof audio.template_version !== 'number' || audio.template_version <= 0) return false;
  return true;
}

/**
 * Pure classifier for voice/process pipeline responses:
 * - Validates response state independently of optional speech
 * - Silent camera actions (ZOOM, PAN, RECENTER) and arrival confirmation open without speech requirement
 * - Non-OK states (CLARIFY, UNSUPPORTED, DATA_UNAVAILABLE, ERROR) do not execute map actions
 * - Absence of template text on guidance actions reports honest caption unavailability without fabricating text
 */
export function processVoiceEnvelope(envelope: VoiceResponseEnvelope | null | undefined): ProcessedVoiceOutcome {
  if (!envelope || !envelope.data) {
    return { kind: 'ERROR' };
  }

  const out = envelope.data;
  const proposal =
    out.validated_proposal && typeof out.validated_proposal === 'object'
      ? (out.validated_proposal as Record<string, unknown>)
      : undefined;

  const rawState = out.state || (proposal?.status as string | undefined) || 'ERROR';
  const pipelineState = String(rawState).toUpperCase();

  if (pipelineState === 'CLARIFY') {
    const clarificationIds = Array.isArray(proposal?.clarification_ids)
      ? (proposal.clarification_ids as string[])
      : [];
    return {
      kind: 'CLARIFY',
      clarification_ids: clarificationIds,
      template_text: out.template?.text,
    };
  }

  if (
    pipelineState === 'UNSUPPORTED' ||
    pipelineState === 'DATA_UNAVAILABLE' ||
    pipelineState === 'ERROR' ||
    pipelineState === 'MODEL_UNAVAILABLE' ||
    pipelineState === 'CANCELED'
  ) {
    return { kind: 'ERROR' };
  }

  if (pipelineState !== 'OK') {
    return { kind: 'ERROR' };
  }

  // Pipeline state is OK
  const authoritativeDataVersion = envelope.data_version || out.data_version;
  const audioMeta = buildAudioMetadata(out.audio, out.template, authoritativeDataVersion);

  const actions = Array.isArray(proposal?.actions)
    ? (proposal.actions as Array<{ type?: string; panel?: string }>)
    : [];
  const hasSpeechKey = typeof proposal?.speech_key === 'string' && proposal.speech_key.trim().length > 0;
  const isCameraOrSilent =
    !hasSpeechKey &&
    actions.length > 0 &&
    actions.every(
      (act) =>
        act.type === 'ZOOM' ||
        act.type === 'PAN' ||
        act.type === 'RECENTER' ||
        (act.type === 'OPEN_PANEL' && act.panel === 'ARRIVAL_CONFIRMATION')
    );

  const templateText = out.template?.text && out.template.text.trim().length > 0 ? out.template.text : undefined;
  const guidanceSpeechExpected = !isCameraOrSilent;
  const captionUnavailable = guidanceSpeechExpected && !templateText;

  return {
    kind: 'OK',
    proposal: out.validated_proposal,
    template_text: templateText,
    audio: audioMeta,
    guidanceSpeechExpected,
    captionUnavailable,
  };
}
