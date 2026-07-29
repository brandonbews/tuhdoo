package event

import (
	"bytes"
	"testing"
)

func TestCanonicalize(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"sorts keys", `{"b":1,"a":2}`, `{"a":2,"b":1}`},
		{"sorts nested keys", `{"z":{"y":1,"x":2},"a":[{"c":1,"b":2}]}`,
			`{"a":[{"b":2,"c":1}],"z":{"x":2,"y":1}}`},
		{"strips whitespace", "{ \"a\" : [ 1 , 2 ] ,\n\t\"b\" : null }", `{"a":[1,2],"b":null}`},
		{"preserves number literals", `[0,-1,42,3.14,1e2,-0.5]`, `[0,-1,42,3.14,1e2,-0.5]`},
		{"literals", `[true,false,null]`, `[true,false,null]`},
		{"utf8 stays literal", `{"s":"héllo — ✓ 日本語"}`, `{"s":"héllo — ✓ 日本語"}`},
		{"no html escaping", `{"s":"<a> & </a>"}`, `{"s":"<a> & </a>"}`},
		{"short control escapes", "{\"s\":\"a\\tb\\nc\"}", `{"s":"a\tb\nc"}`},
		{"u00xx for other controls", `{"s":"a\u0001b"}`, `{"s":"a\u0001b"}`},
		{"quote and backslash", `{"s":"say \"hi\" c:\\tmp"}`, `{"s":"say \"hi\" c:\\tmp"}`},
		{"solidus unescaped", `{"s":"a\/b"}`, `{"s":"a/b"}`},
		{"empty containers", `{"a":{},"b":[]}`, `{"a":{},"b":[]}`},
	}
	for _, c := range cases {
		got, err := Canonicalize([]byte(c.in))
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("%s:\n got: %s\nwant: %s", c.name, got, c.want)
		}

		// Idempotence: canonical output canonicalizes to itself.
		again, err := Canonicalize(got)
		if err != nil {
			t.Errorf("%s: second pass: %v", c.name, err)
			continue
		}
		if !bytes.Equal(got, again) {
			t.Errorf("%s: not idempotent:\n1st: %s\n2nd: %s", c.name, got, again)
		}
	}
}

func TestCanonicalizeRejectsGarbage(t *testing.T) {
	for _, in := range []string{``, `{`, `{"a":1}tail`, `[1,]`} {
		if _, err := Canonicalize([]byte(in)); err == nil {
			t.Errorf("Canonicalize(%q) accepted invalid input", in)
		}
	}
}
