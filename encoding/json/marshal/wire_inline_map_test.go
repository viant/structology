package marshal

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestWireInlineMapEmissionOrder(t *testing.T) {
	type row struct {
		Before   string         `json:"before"`
		Extras   map[string]any `format:"inline=true"`
		After    int            `json:"after"`
		Optional string         `json:"optional,omitempty"`
	}
	e := New(nil, nil, false, true, "", "", nil)
	raw, err := e.Marshal(row{Before: "original", Extras: map[string]any{"before": 42, "after": false, "optional": 7, "extra": nil}, After: 3})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["before"] != float64(42) || decoded["after"] != float64(3) || decoded["optional"] != float64(7) {
		t.Fatalf("runtime order changed: %s", raw)
	}
	shape, err := e.Wire(reflect.TypeFor[row]())
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]reflect.Kind{"before": reflect.Interface, "after": reflect.Int, "optional": reflect.Interface}
	for _, p := range shape.Properties() {
		if p.Shape().Kind() != expected[p.Name()] {
			t.Errorf("%s: got %v want %v", p.Name(), p.Shape().Kind(), expected[p.Name()])
		}
		if p.Required() != (p.Name() != "optional") {
			t.Errorf("wrong presence: %s", p.Name())
		}
	}
	if len(shape.Properties()) != 3 {
		t.Fatal("inline map must not create wrapper")
	}
}

func TestWireInlineMapNestedAndOptionalOrder(t *testing.T) {
	type nested struct {
		Extras map[string]any `format:"inline=true"`
		Value  int            `json:"value"`
	}
	type row struct {
		First  string         `json:"first"`
		Nested *nested        `format:"inline=true"`
		Last   int            `json:"last"`
		More   map[string]any `format:"inline=true"`
	}
	for _, omit := range []bool{false, true} {
		e := New(nil, nil, omit, true, "", "", nil)
		shape, err := e.Wire(reflect.TypeFor[row]())
		if err != nil {
			t.Fatal(err)
		}
		if shape.Additional() == nil || shape.Additional().Kind() != reflect.Interface || shape.Nullable() {
			t.Fatal("missing open-object metadata")
		}
		for _, p := range shape.Properties() {
			if p.Shape().Kind() != reflect.Interface {
				t.Errorf("later map must widen %s", p.Name())
			}
			if p.Name() == "value" && p.Required() {
				t.Fatal("nullable ancestor cannot guarantee presence")
			}
		}
		for _, value := range []row{{}, {Nested: &nested{}}, {Nested: &nested{Extras: map[string]any{"first": false}, Value: 2}, More: map[string]any{"last": "replacement"}}} {
			raw, err := e.Marshal(value)
			if err != nil || !json.Valid(raw) {
				t.Fatalf("invalid runtime output: %s %v", raw, err)
			}
		}
	}
}

func TestWireInlineMapDoesNotClaimTypedMapSupport(t *testing.T) {
	type row struct {
		Name   string         `json:"name"`
		Extras map[string]int `format:"inline=true"`
	}
	e := New(nil, nil, false, true, "", "", nil)
	if _, err := e.Wire(reflect.TypeFor[row]()); err == nil {
		t.Fatal("typed inline maps require union metadata, not unconstrained any values")
	}
}

func TestWireInlineMapIgnoredFieldDoesNotWiden(t *testing.T) {
	type row struct {
		Name   string         `json:"name"`
		Extras map[string]any `json:"-" format:"inline=true"`
	}
	e := New(nil, nil, false, true, "", "", nil)
	shape, err := e.Wire(reflect.TypeFor[row]())
	if err != nil {
		t.Fatal(err)
	}
	if shape.Additional() != nil || len(shape.Properties()) != 1 || shape.Properties()[0].Shape().Kind() != reflect.String {
		t.Fatal("ignored map changed schema")
	}
}
