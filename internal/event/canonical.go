// Package event defines the tuhdoo event envelope (002-technology.md, T3):
// canonical JSON encoding, ULID identifiers, date-sharded storage paths,
// and the v1 event-type catalog.
//
// Everything in this package is pure: no clocks, no randomness, no I/O.
// Callers inject time and entropy where they are needed (NewID).
package event

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"unicode/utf8"
)

// Canonicalize returns the canonical encoding of a JSON document.
// It is the single authority on what "canonical" means in tuhdoo.
//
// Canonicalization rules (T3):
//
//  1. No insignificant whitespace anywhere.
//  2. Object keys are sorted by byte value (ascending), at every depth.
//  3. Strings are UTF-8 with minimal escaping: only `"` and `\` are
//     backslash-escaped; control characters U+0000..U+001F use the short
//     forms \b \t \n \f \r where they exist and \u00xx (lowercase hex)
//     otherwise. No \uXXXX escapes for printable characters, no HTML
//     escaping, no \/ escape.
//  4. Number literals are preserved byte-for-byte from the input. Values
//     produced by this package's typed structs are integers, which Go
//     marshals in minimal decimal form ("42", "-1", "0"); that is the
//     canonical form conforming writers must emit. Preserving literals
//     (rather than reformatting) makes canonicalization idempotent, so
//     unknown fields written by newer peers round-trip byte-identically.
//  5. true, false, and null are literal.
//
// Canonicalize(Canonicalize(x)) == Canonicalize(x) for any valid input,
// and for input already in canonical form it is the identity.
func Canonicalize(raw []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber() // keep number literals verbatim (rule 4)
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, fmt.Errorf("canonicalize: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("canonicalize: trailing data after JSON value")
	}
	return appendCanonical(nil, v)
}

// appendCanonical appends the canonical encoding of v (a value produced by
// encoding/json with UseNumber) to dst.
func appendCanonical(dst []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(dst, "null"...), nil
	case bool:
		if x {
			return append(dst, "true"...), nil
		}
		return append(dst, "false"...), nil
	case json.Number:
		return append(dst, x.String()...), nil
	case string:
		return appendCanonicalString(dst, x), nil
	case []any:
		dst = append(dst, '[')
		for i, elem := range x {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = appendCanonical(dst, elem)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		dst = append(dst, '{')
		for i, k := range keys {
			if i > 0 {
				dst = append(dst, ',')
			}
			dst = appendCanonicalString(dst, k)
			dst = append(dst, ':')
			var err error
			dst, err = appendCanonical(dst, x[k])
			if err != nil {
				return nil, err
			}
		}
		return append(dst, '}'), nil
	default:
		return nil, fmt.Errorf("canonicalize: unsupported value type %T", v)
	}
}

const hexDigits = "0123456789abcdef"

// appendCanonicalString appends s as a canonical JSON string (rule 3).
func appendCanonicalString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for _, r := range s {
		switch r {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\t':
			dst = append(dst, '\\', 't')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\r':
			dst = append(dst, '\\', 'r')
		default:
			if r < 0x20 {
				dst = append(dst, '\\', 'u', '0', '0',
					hexDigits[r>>4], hexDigits[r&0xf])
			} else {
				dst = utf8.AppendRune(dst, r)
			}
		}
	}
	return append(dst, '"')
}
