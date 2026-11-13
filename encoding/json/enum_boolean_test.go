package json

import (
	"errors"
	"reflect"
	"testing"

	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"github.com/viant/tagly/choice"
	"github.com/viant/x/shape"
)

type booleanEnumRecord struct {
	Flag     int                            `json:"flag" choice:"0:false" choice:"1:true" enumAccept:"boolean" enumStyle:"boolean" enumNull:"reject"`
	Optional *int                           `json:"optional,omitempty" choice:"0:false" choice:"1:true" enumAccept:"boolean" enumStyle:"boolean"`
	Hidden   int                            `json:"-" choice:"invalid" enumStyle:"boolean"`
	Has      *struct{ Flag, Optional bool } `json:"-" setMarker:"true"`
}

func TestBooleanEnumNativeCodecs(t *testing.T) {
	typ := reflect.TypeFor[booleanEnumRecord]()
	for _, standard := range []bool{false, true} {
		decode := func(raw string, target any) error {
			if standard {
				return UnmarshalStandard([]byte(raw), target, WithPresenceTracking(true))
			}
			return Unmarshal([]byte(raw), target)
		}
		encode := func(value any) ([]byte, error) {
			if standard {
				return MarshalStandard(value)
			}
			return Marshal(value)
		}
		var wire *jsonmarshal.WireShape
		var err error
		if standard {
			wire, err = jsonmarshal.NewStandard(typ).Wire()
		} else {
			encoder, e := NewMarshaller(typ)
			if e != nil {
				t.Fatal(e)
			}
			wire, err = encoder.Wire()
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(wire.Properties()) != 2 {
			t.Fatal("hidden fields leaked")
		}
		for i, p := range wire.Properties() {
			if p.Shape().Kind() != reflect.Bool || p.Shape().Nullable() != (i == 1) || !reflect.DeepEqual(p.Field().Index, []int{i}) {
				t.Fatalf("standard=%v property=%+v", standard, p)
			}
		}
		for _, tc := range []struct {
			raw, want string
			flag      int
			present   bool
		}{
			{`{}`, `{"flag":false}`, 0, false},
			{`{"flag":false}`, `{"flag":false}`, 0, true},
			{`{"flag":true,"optional":false}`, `{"flag":true,"optional":false}`, 1, true},
			{`{"flag":false,"optional":null}`, `{"flag":false}`, 0, true},
		} {
			var value booleanEnumRecord
			if err = decode(tc.raw, &value); err != nil {
				t.Fatalf("standard=%v %s: %v", standard, tc.raw, err)
			}
			if value.Flag != tc.flag || (value.Has != nil && value.Has.Flag) != tc.present {
				t.Fatalf("presence/storage %s: %+v", tc.raw, value)
			}
			if tc.raw == `{"flag":true,"optional":false}` && (value.Optional == nil || *value.Optional != 0 || !value.Has.Optional) {
				t.Fatal("pointer false lost")
			}
			got, e := encode(&value)
			if e != nil || string(got) != tc.want {
				t.Fatalf("standard=%v want=%s got=%s err=%v", standard, tc.want, got, e)
			}
		}
		for _, raw := range []string{`{"flag":null}`, `{"flag":0}`, `{"flag":1}`, `{"flag":"true"}`, `{"flag":[]}`, `{"flag":{}}`} {
			var value booleanEnumRecord
			if decode(raw, &value) == nil {
				t.Fatalf("standard=%v accepted %s", standard, raw)
			}
		}
		_, err = encode(&booleanEnumRecord{Flag: 2})
		if !errors.Is(err, choice.ErrUnknownOutput) {
			t.Fatalf("standard=%v lost output cause: %v", standard, err)
		}
	}
}

func TestBooleanEnumEffectiveMetadataUsesLinkedStorage(t *testing.T) {
	type record struct {
		Flag   int `json:"flag"`
		Secret int `json:"secret"`
	}
	typ := reflect.TypeFor[record]()
	walker, err := shape.CompileRewrite(typ)
	if err != nil {
		t.Fatal(err)
	}
	effective, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Secret" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return &shape.FieldMetadata{Name: f.Name, Tag: `json:"enabled" choice:"0:false" choice:"1:true" enumAccept:"boolean" enumStyle:"boolean"`}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, standard := range []bool{false, true} {
		value := record{Flag: 1, Secret: 7}
		var encoded []byte
		if standard {
			encoded, err = MarshalStandard(&value, WithRepresentation(effective))
		} else {
			encoded, err = Marshal(&value, WithRepresentation(effective))
		}
		if err != nil || string(encoded) != `{"enabled":true}` {
			t.Fatalf("standard=%v %s %v", standard, encoded, err)
		}
		var decoded record
		if standard {
			err = UnmarshalStandard(encoded, &decoded, WithRepresentation(effective))
		} else {
			err = Unmarshal(encoded, &decoded, WithRepresentation(effective))
		}
		if err != nil || decoded.Flag != 1 || decoded.Secret != 0 || value.Secret != 7 {
			t.Fatalf("linked metadata: %+v %v", decoded, err)
		}
	}
}

func BenchmarkBooleanEnumWholeEncodeWarm(b *testing.B) {
	encoder, err := NewMarshaller(reflect.TypeFor[booleanEnumRecord]())
	if err != nil {
		b.Fatal(err)
	}
	value := &booleanEnumRecord{Flag: 1}
	if _, err = encoder.Marshal(value); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = encoder.Marshal(value); err != nil {
			b.Fatal(err)
		}
	}
}
