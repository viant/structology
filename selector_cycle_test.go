package structology

import (
	"net/http"
	"reflect"
	"testing"
)

type cycleLeft struct {
	Right *cycleRight
	Value string
}
type cycleRight struct {
	Left  *cycleLeft
	Value string
}
type cycleNode struct {
	Next  *cycleNode
	Value string
}

func TestSelectorsBoundRecursiveTypes(t *testing.T) {
	for _, tt := range []struct {
		name   string
		typeOf reflect.Type
		paths  []string
	}{
		{"direct", reflect.TypeOf(cycleNode{}), []string{"Next", "Value"}},
		{"indirect", reflect.TypeOf(cycleLeft{}), []string{"Right", "Right.Left", "Right.Value", "Value"}},
		{"request", reflect.TypeOf(struct{ Request *http.Request }{}), []string{"Request", "Request.Method", "Request.Response", "Request.Response.Request"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			state := NewStateType(tt.typeOf)
			for _, path := range tt.paths {
				if state.Lookup(path) == nil {
					t.Errorf("missing bounded field %s", path)
				}
			}
		})
	}
}

func TestCycleGuardDoesNotSuppressSiblings(t *testing.T) {
	type child struct{ Name string }
	type root struct{ A, B *child }
	state := NewStateType(reflect.TypeOf(root{}))
	for _, path := range []string{"A.Name", "B.Name"} {
		if state.Lookup(path) == nil {
			t.Fatalf("missing sibling path %s", path)
		}
	}
}
