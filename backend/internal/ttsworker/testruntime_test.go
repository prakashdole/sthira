package ttsworker

// taggedRuntime is a small testing adapter. It advertises a single
// (language, voice, revision) tuple and a model revision. Every
// Synthesize call returns ErrRuntimeUnavailable (production
// behavior) but the identity fields are populated, so cache hits
// match the worker's identity output.
type taggedRuntime struct {
	language  string
	voice     string
	revision  string
	supported []string
	voices    []VoiceInfo
}

// newTaggedRuntime builds a tagged runtime with one (lang, voice)
// pair. Additional voices can be supplied.
func newTaggedRuntime(language, voice, modelRevision string) *taggedRuntime {
	r := &taggedRuntime{
		language:  language,
		voice:     voice,
		revision:  modelRevision,
		supported: []string{language},
		voices:    []VoiceInfo{{Language: language, Name: voice, Revision: modelRevision}},
	}
	return r
}

func (r *taggedRuntime) Synthesize(_ RequestContext, _ string, _ string, _ string) (*SynthResult, error) {
	return nil, ErrRuntimeUnavailable
}

func (r *taggedRuntime) Close() error { return nil }

func (r *taggedRuntime) Languages() []string { return append([]string(nil), r.supported...) }

func (r *taggedRuntime) Revision() string { return r.revision }

func (r *taggedRuntime) Voice(language string) string {
	if language == r.language {
		return r.voice
	}
	return ""
}

func (r *taggedRuntime) Voices() []VoiceInfo {
	out := make([]VoiceInfo, len(r.voices))
	copy(out, r.voices)
	return out
}
