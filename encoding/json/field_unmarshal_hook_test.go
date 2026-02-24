package json

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"unsafe"
)

func TestFieldUnmarshalHook_SameHolderDecodedSoFar(t *testing.T) {
	type item struct {
		A int
		B int
	}
	hook := func(_ context.Context, holder unsafe.Pointer, field string, value any) (any, error) {
		if field != "B" {
			return value, nil
		}
		cur := (*item)(holder)
		v, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("unexpected type: %T", value)
		}
		return int64(cur.A) * v, nil
	}

	var out item
	if err := Unmarshal([]byte(`{"A":2,"B":3}`), &out, WithFieldUnmarshalHook(hook)); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.A != 2 || out.B != 6 {
		t.Fatalf("unexpected values: %+v", out)
	}
}

func TestFieldUnmarshalHook_NestedHolderLevel(t *testing.T) {
	type child struct {
		X int
		Y int
	}
	type parent struct {
		C child
	}
	hook := func(_ context.Context, holder unsafe.Pointer, field string, value any) (any, error) {
		if field != "Y" {
			return value, nil
		}
		c := (*child)(holder)
		v, ok := value.(int64)
		if !ok {
			return nil, fmt.Errorf("unexpected type: %T", value)
		}
		return int64(c.X) + v, nil
	}

	var out parent
	if err := Unmarshal([]byte(`{"C":{"X":5,"Y":7}}`), &out, WithFieldUnmarshalHook(hook)); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.C.X != 5 || out.C.Y != 12 {
		t.Fatalf("unexpected nested values: %+v", out)
	}
}

func TestFieldUnmarshalHook_Error(t *testing.T) {
	type item struct {
		A int
	}
	want := "hook-fail"
	hook := func(_ context.Context, _ unsafe.Pointer, _ string, _ any) (any, error) {
		return nil, errors.New(want)
	}

	var out item
	err := Unmarshal([]byte(`{"A":1}`), &out, WithFieldUnmarshalHook(hook))
	if err == nil || err.Error() != want {
		t.Fatalf("expected %q, got %v", want, err)
	}
}
