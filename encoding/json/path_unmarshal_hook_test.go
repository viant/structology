package json

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unsafe"
)

func TestPathUnmarshalHook_NestedPath(t *testing.T) {
	type child struct {
		X int
		Y int
	}
	type root struct {
		C child
	}
	hook := func(_ context.Context, holder unsafe.Pointer, path []string, field string, value any) (any, error) {
		if strings.Join(path, ".") == "C" && field == "Y" {
			c := (*child)(holder)
			v, ok := value.(int64)
			if !ok {
				return nil, fmt.Errorf("unexpected type: %T", value)
			}
			return int64(c.X) + v, nil
		}
		return value, nil
	}

	var out root
	if err := Unmarshal([]byte(`{"C":{"X":5,"Y":7}}`), &out, WithPathUnmarshalHook(hook)); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.C.X != 5 || out.C.Y != 12 {
		t.Fatalf("unexpected values: %+v", out)
	}
}

func TestPathAndFieldUnmarshalHook_Compose(t *testing.T) {
	type item struct {
		A int
		B int
	}
	pathHook := func(_ context.Context, _ unsafe.Pointer, path []string, field string, value any) (any, error) {
		if len(path) == 0 && field == "B" {
			v := value.(int64)
			return v + 1, nil
		}
		return value, nil
	}
	fieldHook := func(_ context.Context, holder unsafe.Pointer, field string, value any) (any, error) {
		if field == "B" {
			i := (*item)(holder)
			return int64(i.A) + value.(int64), nil
		}
		return value, nil
	}

	var out item
	if err := Unmarshal([]byte(`{"A":2,"B":3}`), &out, WithPathUnmarshalHook(pathHook), WithFieldUnmarshalHook(fieldHook)); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	// path hook: B=3->4, field hook: A(2)+4=6
	if out.B != 6 {
		t.Fatalf("unexpected B: %v", out.B)
	}
}
