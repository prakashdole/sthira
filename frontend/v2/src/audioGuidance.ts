export type TTSSettings = {
  sample_rate?: number;
  sample_rate_hz?: number;
  channels: number;
  bit_depth: number;
  codec?: string;
};

export type AudioMetadata = {
  audio_b64: string;
  content_type: string;
  byte_size: number;
  checksum_sha256: string;
  source_version: number;
  data_version: string;
  template_key?: string;
  template_version: number;
  language: string;
  settings: TTSSettings;
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
      settings?: {
        sample_rate?: number;
        sample_rate_hz?: number;
        channels?: number;
        bit_depth?: number;
        codec?: string;
      };
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
 * Validates audio integrity metadata strictly against backend contracts.
 * Mandatory fields: content_type, byte_size, checksum_sha256, source_version,
 * template_version, data_version, language, and required settings (sample_rate, channels, bit_depth).
 * Missing, non-positive, or inconsistent values reject audio metadata (returning undefined).
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

  // Mandatory supported content_type
  if (typeof audio.content_type !== 'string' || !audio.content_type.startsWith('audio/')) {
    return undefined;
  }

  // Mandatory exact byte_size (must be positive integer <= 768 KiB ceiling)
  const byteSize = audio.byte_size;
  if (typeof byteSize !== 'number' || byteSize <= 0 || !Number.isInteger(byteSize) || byteSize > 768 * 1024) {
    return undefined;
  }

  // Mandatory checksum_sha256 (must be 64-char hex)
  if (typeof audio.checksum_sha256 !== 'string' || !/^[a-fA-F0-9]{64}$/.test(audio.checksum_sha256)) {
    return undefined;
  }

  // Mandatory non-empty authoritative data_version from envelope
  if (typeof authoritativeDataVersion !== 'string' || authoritativeDataVersion.trim().length === 0) {
    return undefined;
  }

  // Mandatory positive integer source_version
  const sourceVersion = audio.source_version;
  if (typeof sourceVersion !== 'number' || sourceVersion <= 0 || !Number.isInteger(sourceVersion)) {
    return undefined;
  }

  // Mandatory positive integer template_version
  const templateVersion = audio.template_version;
  if (typeof templateVersion !== 'number' || templateVersion <= 0 || !Number.isInteger(templateVersion)) {
    return undefined;
  }

  // Mandatory non-empty language
  const language = audio.language;
  if (typeof language !== 'string' || language.trim().length === 0) {
    return undefined;
  }

  // Mandatory settings with positive sample_rate, channels, and bit_depth
  if (!audio.settings || typeof audio.settings !== 'object') {
    return undefined;
  }
  const settings = audio.settings as Record<string, unknown>;
  const sampleRate = typeof settings.sample_rate === 'number' ? settings.sample_rate : settings.sample_rate_hz;
  if (typeof sampleRate !== 'number' || sampleRate <= 0) {
    return undefined;
  }
  if (typeof settings.channels !== 'number' || settings.channels <= 0) {
    return undefined;
  }
  if (typeof settings.bit_depth !== 'number' || settings.bit_depth <= 0) {
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
    content_type: audio.content_type,
    byte_size: byteSize,
    checksum_sha256: audio.checksum_sha256.toLowerCase(),
    source_version: sourceVersion,
    data_version: authoritativeDataVersion,
    template_key: templateKey,
    template_version: templateVersion,
    language: language.trim(),
    settings: {
      sample_rate: sampleRate,
      sample_rate_hz: sampleRate,
      channels: settings.channels as number,
      bit_depth: settings.bit_depth as number,
      codec: typeof settings.codec === 'string' ? settings.codec : undefined,
    },
  };
}

/**
 * Performs cryptographic and integrity verification over the audio payload bytes:
 * - Content-type must start with audio/
 * - Base64 must decode cleanly
 * - Byte count must strictly equal audioInfo.byte_size
 * - Web Crypto SubtleCrypto must be available (missing capability reports error)
 * - SHA-256 digest must strictly match audioInfo.checksum_sha256
 */
export async function verifyAudioIntegrity(
  audioInfo: AudioMetadata,
  cryptoProvider?: SubtleCrypto
): Promise<{ success: boolean; error?: string }> {
  if (!audioInfo.content_type || !audioInfo.content_type.startsWith('audio/')) {
    return { success: false, error: 'Invalid audio content-type: ' + audioInfo.content_type };
  }

  if (audioInfo.byte_size <= 0 || audioInfo.byte_size > 768 * 1024) {
    return { success: false, error: 'Audio byte size out of valid bounds' };
  }

  if (!audioInfo.checksum_sha256 || !/^[a-fA-F0-9]{64}$/.test(audioInfo.checksum_sha256)) {
    return { success: false, error: 'Invalid or missing checksum_sha256' };
  }

  if (!audioInfo.settings || audioInfo.settings.channels <= 0 || audioInfo.settings.bit_depth <= 0) {
    return { success: false, error: 'Invalid audio settings' };
  }

  let binaryStr: string;
  try {
    binaryStr = atob(audioInfo.audio_b64);
  } catch {
    return { success: false, error: 'Corrupted audio base64' };
  }

  const byteLen = binaryStr.length;
  if (byteLen !== audioInfo.byte_size) {
    return { success: false, error: `Audio byte size mismatch: expected ${audioInfo.byte_size}, got ${byteLen}` };
  }

  const subtle =
    cryptoProvider !== undefined
      ? cryptoProvider
      : ((typeof window !== 'undefined' ? window.crypto?.subtle : undefined) ??
         (typeof globalThis !== 'undefined' ? (globalThis as any).crypto?.subtle : undefined));

  if (!subtle) {
    return { success: false, error: 'Web Crypto API unavailable: cannot verify audio integrity' };
  }

  const uint8 = new Uint8Array(byteLen);
  for (let i = 0; i < byteLen; i++) uint8[i] = binaryStr.charCodeAt(i);

  const hashBuf = await subtle.digest('SHA-256', uint8);
  const hashHex = Array.from(new Uint8Array(hashBuf))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');

  if (hashHex.toLowerCase() !== audioInfo.checksum_sha256.toLowerCase()) {
    return { success: false, error: 'Audio checksum mismatch (integrity failure)' };
  }

  return { success: true };
}

/**
 * Checks whether audio metadata is valid for replay in the active UI context:
 * - Freshness must be CURRENT
 * - Active UI language must match audio language
 * - Active UI data_version must match audio data_version
 * - Positive source_version, template_version, and byte_size required
 * - Checksum must be 64-character hex
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
  if (typeof audio.byte_size !== 'number' || audio.byte_size <= 0) return false;
  if (!audio.checksum_sha256 || !/^[a-fA-F0-9]{64}$/.test(audio.checksum_sha256)) return false;
  if (!audio.content_type || !audio.content_type.startsWith('audio/')) return false;
  if (!audio.settings || audio.settings.channels <= 0 || audio.settings.bit_depth <= 0) return false;
  return true;
}

/**
 * Lightweight context guard tracking state generation across asynchronous operations.
 * Prevents an old asynchronous verification or delayed autoplay rejection from restoring
 * obsolete audio after language/version/freshness changes or manual invalidation.
 */
export class AudioPlaybackGuard {
  private generation = 0;
  private pending: {
    audio: HTMLAudioElement;
    metadata: AudioMetadata;
    expectedLanguage: string;
    expectedDataVersion: string;
    generation: number;
  } | null = null;

  get currentGeneration(): number {
    return this.generation;
  }

  get pendingAutoplay() {
    return this.pending;
  }

  invalidate(): void {
    this.generation++;
    this.pending = null;
  }

  setPending(params: {
    audio: HTMLAudioElement;
    metadata: AudioMetadata;
    expectedLanguage: string;
    expectedDataVersion: string;
  }): boolean {
    this.pending = {
      ...params,
      generation: this.generation,
    };
    return true;
  }

  canPlayPending(currentFreshness: string, currentDataVersion: string, currentLanguageTag: string): boolean {
    if (!this.pending) return false;
    if (this.pending.generation !== this.generation) return false;
    if (currentFreshness !== 'CURRENT') return false;
    if (this.pending.expectedDataVersion !== currentDataVersion) return false;
    if (this.pending.expectedLanguage !== currentLanguageTag) return false;
    if (!isAudioValidForReplay(this.pending.metadata, currentFreshness, currentDataVersion, currentLanguageTag)) {
      return false;
    }
    return true;
  }

  consumePending(): HTMLAudioElement | null {
    const p = this.pending;
    this.pending = null;
    return p ? p.audio : null;
  }
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
