package json

import (
	"context"
	"strings"
	"testing"
)

func TestStrictMode_DefaultPolicies(t *testing.T) {
	opts := resolveOptions(context.Background(), []Option{WithMode(ModeStrict)})
	if opts.UnknownFieldPolicy != ErrorOnUnknown {
		t.Fatalf("expected ErrorOnUnknown, got %v", opts.UnknownFieldPolicy)
	}
	if opts.NumberPolicy != ExactNumbers {
		t.Fatalf("expected ExactNumbers, got %v", opts.NumberPolicy)
	}
	if opts.NullPolicy != StrictNulls {
		t.Fatalf("expected StrictNulls, got %v", opts.NullPolicy)
	}
	if opts.DuplicateKeyPolicy != ErrorOnDuplicate {
		t.Fatalf("expected ErrorOnDuplicate, got %v", opts.DuplicateKeyPolicy)
	}
	if opts.MalformedPolicy != FailFast {
		t.Fatalf("expected FailFast, got %v", opts.MalformedPolicy)
	}
}

func TestStrictMode_UnmarshalUnknownField(t *testing.T) {
	type sample struct {
		ID int
	}
	var out sample
	err := Unmarshal([]byte(`{"ID":1,"Unknown":2}`), &out, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("expected unknown field error in strict mode")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStrictMode_UnmarshalDuplicateField(t *testing.T) {
	type sample struct {
		ID int
	}
	var out sample
	err := Unmarshal([]byte(`{"ID":1,"ID":2}`), &out, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("expected duplicate field error in strict mode")
	}
	if !strings.Contains(err.Error(), "duplicate field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStrictMode_UnmarshalNumberCoercion(t *testing.T) {
	type sample struct {
		ID int
	}
	var out sample
	err := Unmarshal([]byte(`{"ID":1.5}`), &out, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("expected number coercion error in strict mode")
	}
	if !strings.Contains(err.Error(), "expected integer") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStrictMode_UnmarshalNullToNonNullable(t *testing.T) {
	type sample struct {
		ID int
	}
	var out sample
	err := Unmarshal([]byte(`{"ID":null}`), &out, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("expected strict null error")
	}
	if !strings.Contains(err.Error(), "null is not allowed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMalformedPolicy_TrailingComma(t *testing.T) {
	type sample struct {
		ID int
	}
	var compat sample
	if err := Unmarshal([]byte(`{"ID":1,}`), &compat); err != nil {
		t.Fatalf("compat mode should accept trailing comma: %v", err)
	}
	if compat.ID != 1 {
		t.Fatalf("expected ID=1, got %d", compat.ID)
	}
	var strict sample
	err := Unmarshal([]byte(`{"ID":1,}`), &strict, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("strict mode should reject trailing comma")
	}
}

func TestStrictMode_UnmarshalDuplicateFieldMap(t *testing.T) {
	var out map[string]int
	err := Unmarshal([]byte(`{"k":1,"k":2}`), &out, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("expected duplicate field error for map in strict mode")
	}
	if !strings.Contains(err.Error(), "duplicate field") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMalformedPolicy_TrailingCommaArray(t *testing.T) {
	var compat []int
	if err := Unmarshal([]byte(`[1,2,]`), &compat); err != nil {
		t.Fatalf("compat mode should accept trailing comma array: %v", err)
	}
	if len(compat) != 2 || compat[0] != 1 || compat[1] != 2 {
		t.Fatalf("unexpected compat array: %#v", compat)
	}
	var strict []int
	err := Unmarshal([]byte(`[1,2,]`), &strict, WithMode(ModeStrict))
	if err == nil {
		t.Fatalf("strict mode should reject trailing comma array")
	}
}
