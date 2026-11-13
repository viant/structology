package json

import (
	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"reflect"
	"testing"
)

func TestNestedCollectionEnumAdmission(t *testing.T) {
	type deal struct {
		Channels []string `json:"channels" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" choice:"VIDEO"`
	}
	type input struct {
		Deals []deal `json:"deals"`
	}
	for _, standard := range []bool{false, true} {
		decode := func(raw string, target any) error {
			if standard {
				return UnmarshalStandard([]byte(raw), target)
			}
			return Unmarshal([]byte(raw), target)
		}
		for _, raw := range []string{`{"deals":[{"channels":["DISPLAY","VIDEO"]}]}`, `{"deals":[{"channels":[]}]}`, `{"deals":[{"channels":null}]}`, `{"deals":[{}]}`, `{}`} {
			var target input
			if err := decode(raw, &target); err != nil {
				t.Fatalf("standard=%v %s: %v", standard, raw, err)
			}
		}
		for _, raw := range []string{`{"deals":[{"channels":["UNKNOWN"]}]}`, `{"deals":[{"channels":[null]}]}`, `{"deals":[{"channels":[1]}]}`, `{"deals":[{"channels":"DISPLAY"}]}`, `{"deals":[{"channels":{}}]}`} {
			var target input
			if err := decode(raw, &target); err == nil {
				t.Fatalf("standard=%v accepted %s", standard, raw)
			}
		}
	}
}

func TestCollectionEnumOutputUsesCollectionWireShape(t *testing.T) {
	type record struct {
		Channels []string `json:"channels" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" choice:"VIDEO"`
	}
	value := &record{Channels: []string{"DISPLAY", "VIDEO"}}
	for _, standard := range []bool{false, true} {
		var wire *jsonmarshal.WireShape
		var encoded []byte
		var err error
		if standard {
			wire, err = jsonmarshal.NewStandard(reflect.TypeFor[record]()).Wire()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err = MarshalStandard(value)
		} else {
			encoder, e := NewMarshaller(reflect.TypeFor[record]())
			if e != nil {
				t.Fatal(e)
			}
			wire, err = encoder.Wire()
			if err != nil {
				t.Fatal(err)
			}
			encoded, err = encoder.Marshal(value)
		}
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != `{"channels":["DISPLAY","VIDEO"]}` {
			t.Fatalf("standard=%v: %s", standard, encoded)
		}
		channels := wire.Properties()[0].Shape()
		if channels.Kind() != reflect.Slice || channels.Element() == nil || channels.Element().Kind() != reflect.String {
			t.Fatalf("standard=%v collection wire shape=%v", standard, channels.Kind())
		}
	}
}
