package templates

import "errors"

// Typed errors the renderer returns. The worker maps these to TTSState
// values consumed by the orchestrator.
var (
	// ErrUnknownKey is returned for an unknown speech_key. Mapped to
	// TEMPLATE_UNKNOWN at the public boundary.
	ErrUnknownKey = errors.New("templates: unknown key")

	// ErrWithdrawn is returned for a template the catalog marks as
	// withdrawn. The renderer never produces audio for a withdrawn
	// template even if a cache hit existed seconds before; the worker's
	// cache is source-version-wired so withdrawal propagates.
	ErrWithdrawn = errors.New("templates: withdrawn")

	// ErrPendingReview is returned for a template that the catalog marks
	// as PENDING_REVIEW. Pending-review templates are accepted in the
	// registry but never reach production synthesis until a human
	// approves them.
	ErrPendingReview = errors.New("templates: pending review")

	// ErrSyntheticOnly is returned when the catalog marks a template as
	// SYNTHETIC_ONLY and the renderer was not toggled to allow
	// synthetic content. Synthetic templates are test/demo only.
	ErrSyntheticOnly = errors.New("templates: synthetic-only")

	// ErrLanguageUnsupported is returned for a language the template
	// does not support. The renderer NEVER invents translations.
	ErrLanguageUnsupported = errors.New("templates: language unsupported")

	// ErrNoTranslation is returned when a template is registered for a
	// language but the translation table has no entry. Equivalent to
	// ErrLanguageUnsupported in practice but distinguished for the
	// orchestrator's diagnostics.
	ErrNoTranslation = errors.New("templates: no translation")

	// ErrUnknownArg is returned for an argument name not declared in
	// the template's arg schema.
	ErrUnknownArg = errors.New("templates: unknown arg")

	// ErrArgMismatch is returned for malformed / wrong-typed / extra
	// arguments. The renderer enforces the schema exactly.
	ErrArgMismatch = errors.New("templates: argument mismatch")

	// ErrMalformedTemplate is returned when the catalog holds a body
	// with unbalanced braces; the catalog refuses such templates at
	// registration so this fires only when a corrupt catalog is in
	// memory.
	ErrMalformedTemplate = errors.New("templates: malformed template")
)
