package json

import (
	"bytes"
	"testing"
)

func TestUnmarshal_StringDoesNotAliasInputBuffer(t *testing.T) {
	type payload struct {
		S string
	}
	data := []byte(`{"S":"hello"}`)
	var out payload
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.S != "hello" {
		t.Fatalf("unexpected decoded value: %q", out.S)
	}

	idx := bytes.Index(data, []byte("hello"))
	if idx < 0 {
		t.Fatalf("fixture mismatch: missing hello in input")
	}
	data[idx] = 'X'
	if out.S != "hello" {
		t.Fatalf("decoded string was mutated via input buffer aliasing, got %q", out.S)
	}
}

func TestUnmarshal_TypedStringContainersDoNotAliasInputBuffer(t *testing.T) {
	type payload struct {
		Tags    []string
		Payload map[string]string
	}
	data := []byte(`{"Tags":["x","y"],"Payload":{"k1":"v1"}}`)
	var out payload
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(out.Tags) != 2 || out.Tags[0] != "x" || out.Tags[1] != "y" {
		t.Fatalf("unexpected tags: %#v", out.Tags)
	}
	if got := out.Payload["k1"]; got != "v1" {
		t.Fatalf("unexpected payload value: %q", got)
	}

	for _, token := range [][]byte{[]byte("x"), []byte("y"), []byte("k1"), []byte("v1")} {
		idx := bytes.Index(data, token)
		if idx < 0 {
			t.Fatalf("fixture mismatch: missing %q in input", token)
		}
		data[idx] = 'Z'
	}
	if out.Tags[0] != "x" || out.Tags[1] != "y" {
		t.Fatalf("decoded tags were mutated via input buffer aliasing: %#v", out.Tags)
	}
	if got := out.Payload["k1"]; got != "v1" {
		t.Fatalf("decoded payload was mutated via input buffer aliasing: %q", got)
	}
}

func TestUnmarshal_GenericMapStringValuesDoNotAliasInputBuffer(t *testing.T) {
	data := []byte(`{"k":"hello"}`)
	var out map[string]interface{}
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	got, ok := out["k"].(string)
	if !ok {
		t.Fatalf("expected string, got %T", out["k"])
	}
	if got != "hello" {
		t.Fatalf("unexpected decoded value: %q", got)
	}

	idx := bytes.Index(data, []byte("hello"))
	if idx < 0 {
		t.Fatalf("fixture mismatch: missing hello in input")
	}
	data[idx] = 'X'
	got, _ = out["k"].(string)
	if got != "hello" {
		t.Fatalf("decoded map value was mutated via input buffer aliasing, got %q", got)
	}
}
