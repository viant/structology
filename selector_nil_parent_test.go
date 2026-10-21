package structology

import (
	"reflect"
	"testing"
)

func TestSelectorNilParentRead(t *testing.T) {
	type child struct{ ID int }
	type parent struct{ Child *child }
	type input struct{ Parent *parent }
	for _, value := range []*input{{}, {Parent: &parent{}}, {Parent: &parent{Child: &child{}}}} {
		state := NewStateType(reflect.TypeFor[input]()).WithValue(value)
		selector := state.Type().Lookup("Parent.Child.ID")
		if selector == nil {
			t.Fatal("missing nested selector")
		}
		present := value.Parent != nil && value.Parent.Child != nil
		if selector.Has(state.Pointer()) != present {
			t.Fatalf("nil parent presence differs for %+v", value)
		}
		actual := selector.Value(state.Pointer())
		if !present && actual != nil {
			t.Fatal("nil parent produced a field value")
		}
		if present && actual != 0 {
			t.Fatalf("present zero field lost: %v", actual)
		}
		if selector.Has(nil) || selector.Value(nil) != nil {
			t.Fatal("nil root produced a field value")
		}
	}
}
