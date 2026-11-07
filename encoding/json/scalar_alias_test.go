package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
)

type scalarAliasName string

type scalarAliasRecord struct {
	Mode *scalarAliasName `json:"mode" choice:"0:ASAP" choice:"1:SPEND_EVENLY" enumAccept:"scalar" enumStyle:"literal" enumAlias:"0:0" enumAlias:"1:1" enumAlias:"1:Even" enumCase:"fold"`
}

func TestScalarAliasNativeAndStandardJSON(t *testing.T) {
	type envelope struct {
		Rows   []*scalarAliasRecord          `json:"rows"`
		ByName map[string]*scalarAliasRecord `json:"byName"`
	}
	for _, standard := range []bool{false, true} {
		for _, raw := range []string{`1`, `"1"`, `"Even"`, `"spend_evenly"`, `"ſpend_evenly"`} {
			data := []byte(`{"rows":[{"mode":` + raw + `},null,{"mode":null},{}],"byName":{"zero":{"mode":0}}}`)
			var value envelope
			var err error
			if standard {
				err = UnmarshalStandard(data, &value)
			} else {
				err = Unmarshal(data, &value)
			}
			if err != nil {
				t.Fatalf("standard=%v input=%s: %v", standard, raw, err)
			}
			if len(value.Rows) != 4 || value.Rows[0] == nil || value.Rows[0].Mode == nil || *value.Rows[0].Mode != "SPEND_EVENLY" || value.Rows[1] != nil || value.Rows[2].Mode != nil || value.Rows[3].Mode != nil {
				t.Fatalf("nested scalar result differs: %+v", value)
			}
			if zero := value.ByName["zero"]; zero == nil || zero.Mode == nil || *zero.Mode != "ASAP" {
				t.Fatalf("map scalar result differs: %+v", zero)
			}
			for _, encode := range []func(any) ([]byte, error){
				func(v any) ([]byte, error) { return Marshal(v) },
				func(v any) ([]byte, error) { return MarshalStandard(v) },
			} {
				encoded, err := encode(&value)
				if err != nil {
					t.Fatal(err)
				}
				// Standard-library decoding of the canonical output does not need
				// any enum conversion or custom receiver methods.
				var got envelope
				if err := stdjson.Unmarshal(encoded, &got); err != nil || !reflect.DeepEqual(value, got) {
					t.Fatalf("canonical output %s: %v", encoded, err)
				}
			}
		}
		for _, raw := range []string{`2`, `"other"`, `true`, `[]`, `{}`, `1.5`, `"Even\n"`} {
			var value scalarAliasRecord
			data := []byte(`{"mode":` + raw + `}`)
			var err error
			if standard {
				err = UnmarshalStandard(data, &value)
			} else {
				err = Unmarshal(data, &value)
			}
			if err == nil {
				t.Fatalf("standard=%v accepted %s", standard, raw)
			}
		}
	}
}
