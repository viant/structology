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

func TestCollectionEnumOmitEmptyUsesNativeFieldPlan(t *testing.T) {
	type record struct {
		Channels []string `json:"channels,omitempty" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" format:"nullable=false"`
	}
	for _, standard := range []bool{false, true} {
		for _, channels := range [][]string{nil, {}, {"DISPLAY"}} {
			value := &record{Channels: channels}
			var encoded []byte
			var err error
			if standard {
				encoded, err = MarshalStandard(value)
			} else {
				encoder, e := NewMarshaller(reflect.TypeFor[record]())
				if e != nil {
					t.Fatal(e)
				}
				encoded, err = encoder.Marshal(value)
			}
			if err != nil {
				t.Fatalf("standard=%v: %v", standard, err)
			}
			want := `{}`
			if len(channels) > 0 {
				want = `{"channels":["DISPLAY"]}`
			}
			if string(encoded) != want {
				t.Fatalf("standard=%v: %s", standard, encoded)
			}
		}
	}
}

func TestCollectionEnumOutputRejectsUnknownMembers(t *testing.T) {
	type channels []string
	type record struct {
		Channels channels `json:"channels,omitempty" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY"`
	}
	for _, value := range []*record{{Channels: channels{"DISPLAY"}}, {Channels: channels{"UNKNOWN"}}} {
		for _, standard := range []bool{false, true} {
			var err error
			if standard {
				_, err = MarshalStandard(value)
			} else {
				encoder, e := NewMarshaller(reflect.TypeFor[record]())
				if e != nil {
					t.Fatal(e)
				}
				_, err = encoder.Marshal(value)
			}
			if value.Channels[0] == "UNKNOWN" && err == nil {
				t.Fatalf("standard=%v accepted unknown output", standard)
			}
			if value.Channels[0] == "DISPLAY" && err != nil {
				t.Fatalf("standard=%v named collection: %v", standard, err)
			}
		}
	}
}

func TestCollectionEnumPointerOutputPreservesOmission(t *testing.T) {
	type record struct {
		Channels *[]string `json:"channels,omitempty" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY"`
	}
	empty, valid, null := []string{}, []string{"DISPLAY"}, []string(nil)
	for _, tc := range []struct {
		value *[]string
		want  string
	}{{nil, `{}`}, {&empty, `{"channels":[]}`}, {&valid, `{"channels":["DISPLAY"]}`}, {&null, `{"channels":null}`}} {
		for _, standard := range []bool{false, true} {
			var encoded []byte
			var err error
			if standard {
				encoded, err = MarshalStandard(&record{Channels: tc.value})
			} else {
				encoder, e := NewMarshaller(reflect.TypeFor[record]())
				if e != nil {
					t.Fatal(e)
				}
				encoded, err = encoder.Marshal(&record{Channels: tc.value})
			}
			if err != nil || string(encoded) != tc.want {
				t.Fatalf("standard=%v want=%s got=%s err=%v", standard, tc.want, encoded, err)
			}
		}
	}
}

func BenchmarkCollectionEnumWholeEncodeWarm(b *testing.B) {
	type plain struct {
		Channels []string `json:"channels,omitempty"`
	}
	type enumeration struct {
		Channels []string `json:"channels,omitempty" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY"`
	}
	for _, tc := range []struct {
		name  string
		typ   reflect.Type
		value any
	}{{"plain omitted", reflect.TypeFor[plain](), &plain{}}, {"enum omitted", reflect.TypeFor[enumeration](), &enumeration{}}, {"plain valid", reflect.TypeFor[plain](), &plain{Channels: []string{"DISPLAY"}}}, {"enum valid", reflect.TypeFor[enumeration](), &enumeration{Channels: []string{"DISPLAY"}}}} {
		b.Run(tc.name, func(b *testing.B) {
			encoder, err := NewMarshaller(tc.typ)
			if err != nil {
				b.Fatal(err)
			}
			if _, err = encoder.Marshal(tc.value); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err = encoder.Marshal(tc.value); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
