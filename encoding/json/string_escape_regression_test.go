package json

import (
	stdjson "encoding/json"
	"testing"
)

func TestUnmarshal_StringEscapes_Parity(t *testing.T) {
	type payload struct {
		S string
	}
	cases := []string{
		`{"S":"line1\nline2"}`,
		`{"S":"tab\tsep"}`,
		`{"S":"quote:\"ok\""}`,
		`{"S":"slash:\/"}`,
		`{"S":"backslash:\\\\"}`,
		`{"S":"music:\u266B"}`,
		`{"S":"emoji:\uD83D\uDE00"}`,
		`{"S":"combo:\\\\\"end"}`,
	}
	for _, input := range cases {
		var got payload
		if err := Unmarshal([]byte(input), &got); err != nil {
			t.Fatalf("structology unmarshal failed for %s: %v", input, err)
		}
		var want payload
		if err := stdjson.Unmarshal([]byte(input), &want); err != nil {
			t.Fatalf("stdlib unmarshal failed for %s: %v", input, err)
		}
		if got != want {
			t.Fatalf("mismatch for %s: got=%q want=%q", input, got.S, want.S)
		}
	}
}

func TestUnmarshal_StringEscapes_Invalid(t *testing.T) {
	type payload struct {
		S string
	}
	cases := []string{
		`{"S":"\x"}`,
		`{"S":"\u12"}`,
		`{"S":"\uD83Dx"}`,
		"{\"S\":\"unterminated}",
	}
	for _, input := range cases {
		var got payload
		if err := Unmarshal([]byte(input), &got); err == nil {
			t.Fatalf("expected error for invalid input %s", input)
		}
	}
}
