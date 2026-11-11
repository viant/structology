package marshal

import (
	"reflect"
	"strings"
	"testing"
)

type WinnerEmbedded struct{ Id int }
type winnerOuter struct {
	*WinnerEmbedded
	Id int `json:"id"`
}
type WinnerTagged struct {
	Other int `json:"id"`
}
type winnerPeers struct {
	WinnerEmbedded
	WinnerTagged
}
type WinnerOther struct{ ID int }
type winnerAmbiguous struct {
	WinnerEmbedded
	WinnerOther
}

func TestFinalNameFieldWinner(t *testing.T) {
	for _, dynamic := range []bool{false, true} {
		var transform func([]string, string) string
		format := "lc"
		if dynamic {
			transform = func(_ []string, s string) string { return strings.ToLower(s) }
			format = ""
		}
		e := New(transform, nil, false, true, "", format, strings.ToLower)
		for _, value := range []winnerOuter{{&WinnerEmbedded{7}, 42}, {nil, 42}} {
			for _, ptr := range []bool{false, true} {
				var raw []byte
				var err error
				if ptr {
					raw, err = e.MarshalPtrTo(nil, &value)
				} else {
					raw, err = e.Marshal(value)
				}
				if err != nil || string(raw) != `{"id":42}` {
					t.Fatalf("dynamic=%v ptr=%v: %s %v", dynamic, ptr, raw, err)
				}
			}
		}
		shape, err := e.Wire(reflect.TypeFor[winnerOuter]())
		if err != nil || len(shape.Properties()) != 1 || shape.Properties()[0].Name() != "id" {
			t.Fatalf("wire: %+v %v", shape, err)
		}
		raw, err := e.Marshal(winnerPeers{WinnerEmbedded{7}, WinnerTagged{42}})
		if err != nil || string(raw) != `{"id":42}` {
			t.Fatalf("tag precedence: %s %v", raw, err)
		}
		if _, err = e.Marshal(winnerAmbiguous{}); err == nil {
			t.Fatal("ambiguous runtime names accepted")
		}
		if _, err = e.Wire(reflect.TypeFor[winnerAmbiguous]()); err == nil {
			t.Fatal("ambiguous discovery names accepted")
		}
	}
}
func TestExcludedWinnerDoesNotResurrectEmbeddedField(t *testing.T) {
	e := New(func(_ []string, s string) string { return strings.ToLower(s) }, func(_ []string, s string) bool { return s == "Id" }, false, true, "", "", nil)
	raw, err := e.Marshal(winnerOuter{&WinnerEmbedded{7}, 42})
	if err != nil || string(raw) != "{}" {
		t.Fatalf("excluded winner: %s %v", raw, err)
	}
}

func TestWinnerNestedGraphAndNoOmitFallback(t *testing.T) {
	type optionalOuter struct {
		*WinnerEmbedded
		Id int `json:"id,omitempty"`
	}
	value := struct {
		Rows []optionalOuter `json:"rows"`
		Item *optionalOuter  `json:"item"`
	}{[]optionalOuter{{&WinnerEmbedded{7}, 0}}, nil}
	e := New(nil, nil, false, true, "", "lc", strings.ToLower)
	raw, err := e.MarshalPtrTo(nil, &value)
	if err != nil || string(raw) != `{"rows":[{}],"item":null}` {
		t.Fatalf("nested/omitted winner: %s %v", raw, err)
	}
}
