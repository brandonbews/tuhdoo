package event

import (
	"bytes"
	"encoding/json"
	"testing"
)

// A canonical event as a newer peer might write it: unknown fields at
// both envelope level ("trace") and payload level ("estimate", "kind"),
// with values our binary cannot interpret. Keys are in sorted order and
// the encoding is canonical, exactly as a conforming writer stores it.
const futureEvent = `{"actor":"brandon/impl-2","data":{"depends_on":null,"description":"","estimate":{"unit":"pomodoro","value":3.5},"kind":["deep","risky"],"labels":null,"parents":null,"priority":2,"title":"From the future"},"id":"01BX5ZZKBKACTAV9WEVGEMMVRY","machine":"m-3f9a","sig":null,"task":"t-01BX5ZZKBKACTAV9WEVGEMMVRX","trace":"otel-4711","type":"task.created","v":1}`

func TestDecodeEncodePreservesUnknownFields(t *testing.T) {
	e, err := Decode([]byte(futureEvent))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// The unknown envelope field is held, not dropped.
	if string(e.Unknown["trace"]) != `"otel-4711"` {
		t.Errorf("unknown envelope field not preserved: %q", e.Unknown["trace"])
	}

	// Byte-identical round trip, envelope and payload included.
	out, err := Encode(e)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(out) != futureEvent {
		t.Errorf("round trip changed bytes\n got: %s\nwant: %s", out, futureEvent)
	}
}

func TestPayloadStructPreservesUnknownFields(t *testing.T) {
	e, err := Decode([]byte(futureEvent))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	// Typed decode of the payload keeps the fields it has no place for...
	var p TaskCreated
	if err := json.Unmarshal(e.Data, &p); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if p.Title != "From the future" || p.Priority != 2 {
		t.Errorf("known fields lost: %+v", p)
	}
	if _, ok := p.Unknown["estimate"]; !ok {
		t.Error("unknown payload field 'estimate' dropped")
	}
	if _, ok := p.Unknown["kind"]; !ok {
		t.Error("unknown payload field 'kind' dropped")
	}

	// ...and re-marshaling the struct carries them back out.
	remarshaled, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	canon, err := Canonicalize(remarshaled)
	if err != nil {
		t.Fatalf("Canonicalize: %v", err)
	}
	wantCanon, err := Canonicalize(e.Data)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(canon, wantCanon) {
		t.Errorf("payload round trip changed bytes\n got: %s\nwant: %s", canon, wantCanon)
	}
}

func TestEncodeDeterminism(t *testing.T) {
	payload := NoteAdded{Text: "same event, same bytes"}
	e, err := New("01BX5ZZKBKACTAV9WEVGEMMVRY", TypeNoteAdded, 1,
		"brandon", "m-3f9a", "t-01BX5ZZKBKACTAV9WEVGEMMVRX", payload)
	if err != nil {
		t.Fatal(err)
	}

	first, err := Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("two encodes of the same event differ:\n%s\n%s", first, second)
	}

	decoded, err := Decode(first)
	if err != nil {
		t.Fatal(err)
	}
	third, err := Encode(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, third) {
		t.Errorf("encoding a decoded copy differs:\n%s\n%s", first, third)
	}
}

func TestEncodeOmitsEmptyTaskAndAbsentSig(t *testing.T) {
	e, err := New("01BX5ZZKBKACTAV9WEVGEMMVRY", TypeNoteAdded, 1,
		"brandon", "m-3f9a", "", NoteAdded{Text: "no subject task"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := Encode(e)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte(`"task"`)) {
		t.Errorf("empty task should be omitted: %s", out)
	}
	if !bytes.Contains(out, []byte(`"sig":null`)) {
		t.Errorf("new events must carry sig:null: %s", out)
	}

	// An event stored without sig (hypothetical foreign writer) keeps
	// sig absent through a round trip — bytes are never invented.
	noSig := `{"actor":"a","data":{"text":"x"},"id":"01BX5ZZKBKACTAV9WEVGEMMVRY","machine":"m","type":"note.added","v":1}`
	d, err := Decode([]byte(noSig))
	if err != nil {
		t.Fatal(err)
	}
	again, err := Encode(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != noSig {
		t.Errorf("sig-less round trip changed bytes\n got: %s\nwant: %s", again, noSig)
	}
}

func TestDecodeRejectsMissingRequiredFields(t *testing.T) {
	cases := map[string]string{
		"no id":    `{"data":{},"type":"note.added","v":1}`,
		"no type":  `{"data":{},"id":"01BX5ZZKBKACTAV9WEVGEMMVRY","v":1}`,
		"no v":     `{"data":{},"id":"01BX5ZZKBKACTAV9WEVGEMMVRY","type":"note.added"}`,
		"no data":  `{"id":"01BX5ZZKBKACTAV9WEVGEMMVRY","type":"note.added","v":1}`,
		"not json": `{"id":`,
	}
	for name, in := range cases {
		if _, err := Decode([]byte(in)); err == nil {
			t.Errorf("%s: Decode accepted invalid event", name)
		}
	}
}
