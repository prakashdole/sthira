// Package templates implements the frozen approved-template registry
// for the TTS worker. Go renders reviewed template keys from a
// structurally-validated argument object; arbitrary user / model
// prose never reaches the synthesis endpoint.
//
// Templates are intentionally narrow. Each template:
//   - has a fixed TemplateKey and a fixed small language allow list.
//   - declares its argument schema (key -> type) so the renderer can
//     reject arbitrary text fields.
//   - lists every translation it ships; any missing language is rejected
//     rather than invented at run time.
//   - carries a status flag that the renderer returns alongside the
//     text. Synthetic-only templates are gated behind an explicit
//     allow flag (ApproveSynthetic) and never reached in production
//     synthesis paths.
//   - never carries filler acknowledgments for map movement.
//
// The renderer returns a Rendered struct, never raw strings into the
// worker pipeline. Callers compose the template key and the validated
// Argument object; the renderer enforces the schema, validates the
// language, builds the cache-stable text and returns the audible
// version. Anything not in the catalog is refused at the typed-error
// level.
package templates

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Key is a frozen speech template key. Strict allow-list; unknown
// values are rejected before any synthesis happens.
type Key string

// Status describes the approval state of a template. SyntheticOnly
// templates are TEST/DEMO only and are rejected by the renderer for
// non-test callers regardless of language support.
type Status string

const (
	StatusApproved      Status = "APPROVED"
	StatusPendingReview Status = "PENDING_REVIEW"
	StatusSyntheticOnly Status = "SYNTHETIC_ONLY"
	StatusWithdrawn     Status = "WITHDRAWN"
)

// Template is one frozen, reviewed entry. Translations is keyed by
// language; an empty Translations map for a supported language means
// the template is not translated and Render returns ErrNoTranslation.
type Template struct {
	Key         Key    `json:"key"`
	Status      Status `json:"status"`
	Version     int    `json:"version"`
	Description string `json:"description,omitempty"`
	// Languages is the explicit list of supported languages for this
	// template. Order is not semantic.
	Languages []string `json:"languages"`
	// ArgSchema declares the keys and types the renderer expects.
	// Allowed types are "string" (validated as a typed ID), "int".
	// Anything not declared here is rejected.
	ArgSchema map[string]ArgType `json:"arg_schema"`
	// Translations is the rendered text per language. The renderer
	// substitutes {arg_name} with the validated argument; substitution
	// is bounded and not a generic template engine.
	Translations map[string]string `json:"translations"`
}

// ArgType is the schema-declared argument type for a template slot.
type ArgType string

const (
	ArgString ArgType = "string"
	ArgInt    ArgType = "int"
)

// Argument carries one validated argument value. The renderer never
// accepts raw maps from outside the validator; callers run ValidateArgs
// first and pass the typed slice through.
type Argument struct {
	Name  string  `json:"name"`
	Type  ArgType `json:"type"`
	Value string  `json:"value"`
}

// Rendered is the result of rendering a template. The Text is the
// canonical pronunciation string used as one of the cache identity
// inputs. Status is returned to the caller so a withdrawn template can
// surface as AUDIO_UNAVAILABLE even if a cache hit existed seconds
// before.
type Rendered struct {
	Key         Key       `json:"key"`
	Language    string    `json:"language"`
	Text        string    `json:"text"`
	Status      Status    `json:"status"`
	Version     int       `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
}

// Catalog is a small read-only registry of approved templates.
// Production code loads the catalog once and freezes it. Synthetic-only
// templates are loaded only when the loader explicitly opts in.
type Catalog struct {
	mu       sync.RWMutex
	tmpls    map[Key]*Template
	versions map[Key]int
}

// NewCatalog constructs a fresh empty catalog. Registry grows by
// Register; production freezes the registry before handing it to the
// renderer.
func NewCatalog() *Catalog {
	return &Catalog{
		tmpls:    map[Key]*Template{},
		versions: map[Key]int{},
	}
}

// Register adds a single template to the catalog. Returns an error if
// the template fails validation (missing language, declared arg schema
// that does not match translations, etc.) — the catalog refuses
// half-formed entries.
func (c *Catalog) Register(t *Template) error {
	if err := validateTemplate(t); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	cp := *t
	cp.Languages = append([]string(nil), t.Languages...)
	cp.ArgSchema = map[string]ArgType{}
	for k, v := range t.ArgSchema {
		cp.ArgSchema[k] = v
	}
	cp.Translations = map[string]string{}
	for k, v := range t.Translations {
		cp.Translations[k] = v
	}
	c.tmpls[t.Key] = &cp
	c.versions[t.Key] = t.Version
	return nil
}

// Lookup returns the template for a key, if present.
func (c *Catalog) Lookup(k Key) (*Template, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.tmpls[k]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// Keys returns the registered template keys sorted. Useful for /health
// and the allowed-keys listing.
func (c *Catalog) Keys() []Key {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Key, 0, len(c.tmpls))
	for k := range c.tmpls {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// Snapshot returns a deep copy of the catalog for frozen use.
func (c *Catalog) Snapshot() *Catalog {
	cp := NewCatalog()
	c.mu.RLock()
	defer c.mu.RUnlock()
	for k, t := range c.tmpls {
		t2 := *t
		t2.Languages = append([]string(nil), t.Languages...)
		t2.ArgSchema = map[string]ArgType{}
		for ak, av := range t.ArgSchema {
			t2.ArgSchema[ak] = av
		}
		t2.Translations = map[string]string{}
		for tk, tv := range t.Translations {
			t2.Translations[tk] = tv
		}
		cp.tmpls[k] = &t2
		cp.versions[k] = t.Version
	}
	return cp
}

// Withdraw marks a template key as withdrawn. Subsequent Render
// calls reject with ErrWithdrawn.
func (c *Catalog) Withdraw(k Key, version int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.tmpls[k]
	if !ok {
		return ErrUnknownKey
	}
	t.Status = StatusWithdrawn
	c.versions[k] = version
	return nil
}

// ValidateArgs is the only path for untyped arguments to reach the
// renderer. It enforces the schema and rejects extra fields. Returns
// a typed Argument slice the renderer can consume without any further
// string-key parsing.
//
// The check order is deliberate: an UNKNOWN argument name is more
// specific than a count mismatch, so we surface ErrUnknownArg before
// ErrArgMismatch for extra fields.
func (c *Catalog) ValidateArgs(k Key, raw map[string]any) ([]Argument, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.tmpls[k]
	if !ok {
		return nil, ErrUnknownKey
	}
	out := make([]Argument, 0, len(t.ArgSchema))
	for name, v := range raw {
		want, ok := t.ArgSchema[name]
		if !ok {
			return nil, fmt.Errorf("%w: %q", ErrUnknownArg, name)
		}
		switch want {
		case ArgString:
			s, ok := v.(string)
			if !ok || s == "" {
				return nil, fmt.Errorf("%w: %q must be non-empty string", ErrArgMismatch, name)
			}
			if !validIDChar(s) {
				return nil, fmt.Errorf("%w: %q contains forbidden characters", ErrArgMismatch, name)
			}
			out = append(out, Argument{Name: name, Type: want, Value: s})
		case ArgInt:
			f, ok := toInt(v)
			if !ok {
				return nil, fmt.Errorf("%w: %q must be integer", ErrArgMismatch, name)
			}
			out = append(out, Argument{Name: name, Type: want, Value: fmt.Sprintf("%d", f)})
		default:
			return nil, fmt.Errorf("%w: type %q unknown", ErrArgMismatch, want)
		}
	}
	if len(out) != len(t.ArgSchema) {
		return nil, fmt.Errorf("%w: missing arg(s)", ErrArgMismatch)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Renderer renders templates with strict validation. Use one per
// worker; it is safe for concurrent use.
type Renderer struct {
	cat              *Catalog
	allowSynthetic   bool
	allowWithdrawing bool
}

// NewRenderer returns a renderer over catalog c.
func NewRenderer(c *Catalog) *Renderer {
	return &Renderer{cat: c}
}

// AllowSynthetic toggles rendering for SYNTHETIC_ONLY templates. The
// toggle defaults to false; production must never set it true.
func (r *Renderer) AllowSynthetic(v bool) { r.allowSynthetic = v }

// Cat exposes the underlying catalog for tests and integration paths
// that need cross-catalog validation. Production callers should
// normally reach the catalog directly.
func (r *Renderer) Cat() *Catalog { return r.cat }

// AllowWithdrawing toggles rendering of WITHDRAWN templates. The
// default of false keeps the cache invalidated: anything recognized
// as withdrawn refuses to render and the worker surfaces
// AUDIO_UNAVAILABLE.
func (r *Renderer) AllowWithdrawing(v bool) { r.allowWithdrawing = v }

// Render substitutes the validated arguments into the language's
// translation. Errors are typed and the worker maps them to
// transcript states.
func (r *Renderer) Render(k Key, language string, args []Argument) (*Rendered, error) {
	t, ok := r.cat.Lookup(k)
	if !ok {
		return nil, ErrUnknownKey
	}
	if t.Status == StatusWithdrawn && !r.allowWithdrawing {
		return nil, ErrWithdrawn
	}
	if t.Status == StatusSyntheticOnly && !r.allowSynthetic {
		return nil, ErrSyntheticOnly
	}
	if t.Status == StatusPendingReview {
		return nil, ErrPendingReview
	}
	if !containsString(t.Languages, language) {
		return nil, ErrLanguageUnsupported
	}
	body, ok := t.Translations[language]
	if !ok {
		return nil, ErrNoTranslation
	}
	substituted, err := substitute(body, t.ArgSchema, args)
	if err != nil {
		return nil, err
	}
	return &Rendered{
		Key:         t.Key,
		Language:    language,
		Text:        substituted,
		Status:      t.Status,
		Version:     t.Version,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// substitute replaces {name} placeholders with the validated
// argument value. The placeholder engine is intentionally tiny: it
// walks the body looking for balanced {name} occurrences and refuses
// anything that does not match the schema. No conditional, no loop,
// no recursion — substitution is pure data flow.
func substitute(body string, schema map[string]ArgType, args []Argument) (string, error) {
	if !strings.Contains(body, "{") {
		return body, nil
	}
	byName := map[string]Argument{}
	for _, a := range args {
		byName[a.Name] = a
	}
	var b strings.Builder
	i := 0
	for i < len(body) {
		if body[i] == '{' {
			end := strings.IndexByte(body[i:], '}')
			if end < 0 {
				return "", ErrMalformedTemplate
			}
			// Bounds-check end relative to i.
			j := i + end
			if j+1 > len(body) {
				return "", ErrMalformedTemplate
			}
			name := body[i+1 : j]
			if name == "" {
				return "", ErrMalformedTemplate
			}
			if _, ok := schema[name]; !ok {
				return "", fmt.Errorf("%w: %q", ErrUnknownArg, name)
			}
			arg, ok := byName[name]
			if !ok {
				return "", fmt.Errorf("%w: %q missing", ErrArgMismatch, name)
			}
			if !validIDChar(arg.Value) {
				return "", fmt.Errorf("%w: %q contains forbidden characters", ErrArgMismatch, name)
			}
			b.WriteString(arg.Value)
			i = j + 1
			continue
		}
		b.WriteByte(body[i])
		i++
	}
	return b.String(), nil
}

func validateTemplate(t *Template) error {
	if t == nil {
		return errors.New("nil template")
	}
	if t.Key == "" {
		return errors.New("template key is empty")
	}
	switch t.Status {
	case StatusApproved, StatusPendingReview, StatusSyntheticOnly, StatusWithdrawn:
	default:
		return fmt.Errorf("template status %q invalid", t.Status)
	}
	if len(t.Languages) == 0 {
		return errors.New("template has no languages")
	}
	if len(t.ArgSchema) == 0 {
		// Templates without args are allowed but the renderer must
		// still reject extra-arg payloads.
	}
	for lang, body := range t.Translations {
		if !containsString(t.Languages, lang) {
			return fmt.Errorf("translation present for undeclared language %q", lang)
		}
		// Every placeholder must reference a declared arg schema entry.
		for i := 0; i < len(body); i++ {
			if body[i] != '{' {
				continue
			}
			end := strings.IndexByte(body[i:], '}')
			if end < 0 {
				return fmt.Errorf("unterminated placeholder in %q", t.Key)
			}
			name := body[i+1 : i+end]
			if _, ok := t.ArgSchema[name]; !ok {
				return fmt.Errorf("translation for %s references undeclared arg %q", lang, name)
			}
		}
	}
	return nil
}

func containsString(s []string, needle string) bool {
	for _, v := range s {
		if v == needle {
			return true
		}
	}
	return false
}

// validIDChar returns true when the value looks like a typed ID, not
// arbitrary user text. We forbid control characters, angle brackets,
// curly braces, quotes, and backslashes so substituted text cannot
// smuggle SSTI / template injection payloads into the synthesis text.
func validIDChar(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c == '-' || c == '_' || c == '.' || c == ':':
		default:
			return false
		}
	}
	return true
}

func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int32:
		return int(x), true
	case int64:
		return int(x), true
	case float64:
		if x != float64(int(x)) {
			return 0, false
		}
		return int(x), true
	default:
		return 0, false
	}
}
