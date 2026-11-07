package marshal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTransformedWirePreservesDynamicLeaf(t *testing.T) {
	type envelope struct {
		Errors any    `json:"errors,omitempty"`
		Name   string `json:"name"`
	}
	e := New(func(_ []string, s string) string { return strings.ToLower(s) }, func(_ []string, s string) bool { return s == "Secret" }, false, true, "", "", nil)
	// Dynamic data is already supported by the encoder. Discovery must not
	// infer a concrete type from one value or reject the enclosing typed object.
	for _, value := range []any{nil, "message", []any{"message"}, struct {
		Code   int
		Secret string
	}{7, "private"}} {
		raw, err := e.Marshal(envelope{Errors: value, Name: "visible"})
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(raw) || strings.Contains(string(raw), "private") {
			t.Fatalf("invalid/private output: %s", raw)
		}
	}
	shape, err := e.Wire(reflect.TypeFor[envelope]())
	if err != nil {
		t.Fatal(err)
	}
	var dynamic *WireShape
	for _, p := range shape.Properties() {
		if p.Name() == "errors" {
			dynamic = p.Shape()
		}
	}
	if dynamic == nil || dynamic.Kind() != reflect.Interface || !dynamic.Nullable() {
		t.Fatalf("dynamic shape not preserved: %+v", dynamic)
	}
	standard, err := NewStandard(reflect.TypeFor[envelope]()).Wire()
	if err != nil {
		t.Fatal(err)
	}
	if standard.Properties()[0].Shape().Kind() != dynamic.Kind() {
		t.Fatal("standard/native representation differs")
	}
}
