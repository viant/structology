package marshal

import (
	"reflect"
	"strings"
	"testing"
)

func TestProjectionDisambiguatesOnlyEqualPriorityFields(t *testing.T) {
	type row struct {
		First  int
		Second int
	}
	value := row{11, 77}
	for _, keep := range []string{"First", "Second"} {
		e := New(func(_ []string, _ string) string { return "value" }, func(_ []string, name string) bool { return name != keep }, false, true, "", "", nil)
		raw, err := e.Marshal(value)
		want := `{"value":11}`
		if keep == "Second" {
			want = `{"value":77}`
		}
		if err != nil || string(raw) != want {
			t.Fatalf("%s: %s %v", keep, raw, err)
		}
		wire, err := e.Wire(reflect.TypeFor[row]())
		if err != nil || len(wire.Properties()) != 1 {
			t.Fatalf("wire %s: %v", keep, err)
		}
	}
	e := New(func(_ []string, _ string) string { return "value" }, nil, false, true, "", "", nil)
	if _, err := e.Marshal(value); err == nil {
		t.Fatal("unprojected collision accepted")
	}
	excluded := New(func(_ []string, _ string) string { return "value" }, func(_ []string, _ string) bool { return true }, false, true, "", "", nil)
	raw, err := excluded.Marshal(value)
	if err != nil || string(raw) != "{}" {
		t.Fatalf("empty projection: %s %v", raw, err)
	}
	// Choosing only the embedded declaration cannot resurrect it after dominance.
	outer := New(func(_ []string, s string) string { return strings.ToLower(s) }, nil, false, true, "", "", nil)
	if err := outer.ExcludeFields(reflect.TypeFor[winnerOuter](), []string{"Id"}); err != nil {
		t.Fatal(err)
	}
	raw, err = outer.Marshal(winnerOuter{&WinnerEmbedded{7}, 42})
	if err != nil || string(raw) != "{}" {
		t.Fatalf("dominated resurrection: %s %v", raw, err)
	}
}
