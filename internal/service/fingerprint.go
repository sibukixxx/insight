package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Fingerprint is the canonical content hash for run snapshots:
// "sha256:<hex>" over compact JSON with sorted object keys and no HTML
// escaping. Null values and empty arrays/objects are dropped, so a nil
// slice, an empty slice and an absent field hash the same. Array order is
// kept because it can be meaningful; callers sort sets before hashing.
// The "sha256:" prefix matches the Analytical Artifact ReproducibilityKey
// and the approved research artifact reference.
func Fingerprint(v any) (string, error) {
	canonical, err := CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// CanonicalJSON returns the byte form Fingerprint hashes.
func CanonicalJSON(v any) ([]byte, error) {
	raw, err := marshalNoEscape(v)
	if err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var generic any
	if err := decoder.Decode(&generic); err != nil {
		return nil, fmt.Errorf("canonical json: %w", err)
	}
	pruned, _ := pruneEmpty(generic)
	// encoding/json writes map keys in sorted order.
	return marshalNoEscape(pruned)
}

func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// pruneEmpty removes nulls and empty collections. keep is false when the
// value itself should be dropped from its parent.
func pruneEmpty(v any) (value any, keep bool) {
	switch typed := v.(type) {
	case nil:
		return nil, false
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			if pruned, ok := pruneEmpty(child); ok {
				out[key] = pruned
			}
		}
		return out, len(out) > 0
	case []any:
		if len(typed) == 0 {
			return nil, false
		}
		out := make([]any, 0, len(typed))
		for _, child := range typed {
			pruned, ok := pruneEmpty(child)
			if !ok {
				// Keep position so element order stays meaningful.
				pruned = nil
			}
			out = append(out, pruned)
		}
		return out, true
	}
	return v, true
}
