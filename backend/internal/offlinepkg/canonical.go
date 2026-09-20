package offlinepkg

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

// CanonicalBytes returns the deterministic canonical JSON byte representation
// of v for checksumming and signing. For Manifest and PublicIncidentCard,
// the mutable integrity metadata (ChecksumSHA256 and Signature) are cleared
// before serialization. Keys are recursively sorted in lexicographical order
// with minimal separators and no insignificant whitespace.
func CanonicalBytes(v any) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: cannot canonicalize nil", ErrMalformedData)
	}

	// Normalize structs by clearing mutable checksum and signature fields.
	switch obj := v.(type) {
	case *Manifest:
		cpy := *obj
		cpy.ChecksumSHA256 = ""
		cpy.Signature = nil
		v = cpy
	case Manifest:
		obj.ChecksumSHA256 = ""
		obj.Signature = nil
		v = obj
	case *PublicIncidentCard:
		cpy := *obj
		cpy.ChecksumSHA256 = ""
		cpy.Signature = nil
		v = cpy
	case PublicIncidentCard:
		obj.ChecksumSHA256 = ""
		obj.Signature = nil
		v = obj
	}

	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("%w: json marshal: %v", ErrMalformedData, err)
	}

	var generic any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&generic); err != nil {
		return nil, fmt.Errorf("%w: decode generic: %v", ErrMalformedData, err)
	}

	var buf bytes.Buffer
	writeCanonical(&buf, generic)
	return buf.Bytes(), nil
}

// ChecksumSHA256 computes the 64-character lowercase hex SHA-256 digest
// over canonical bytes.
func ChecksumSHA256(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// SignCanonical computes the canonical bytes of v, signs them with the
// provided Ed25519 private key, and returns the detached Signature metadata.
func SignCanonical(privateKey ed25519.PrivateKey, keyID string, v any) (*Signature, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: invalid private key length %d", ErrInvalidSignature, len(privateKey))
	}
	if keyID == "" {
		return nil, fmt.Errorf("%w: key_id is required", ErrSignerUnauthorized)
	}

	canonical, err := CanonicalBytes(v)
	if err != nil {
		return nil, err
	}

	sigBytes := ed25519.Sign(privateKey, canonical)
	b64Sig := base64.StdEncoding.EncodeToString(sigBytes)

	return &Signature{
		Algorithm: "Ed25519",
		KeyID:     keyID,
		Value:     b64Sig,
	}, nil
}

func writeCanonical(buf *bytes.Buffer, v any) {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			kb, _ := json.Marshal(k)
			buf.Write(kb)
			buf.WriteByte(':')
			writeCanonical(buf, t[k])
		}
		buf.WriteByte('}')
	case []any:
		buf.WriteByte('[')
		for i, e := range t {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonical(buf, e)
		}
		buf.WriteByte(']')
	default:
		b, _ := json.Marshal(t)
		buf.Write(b)
	}
}
