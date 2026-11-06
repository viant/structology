package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/viant/tagly/format/text"
)

type enumRecord struct {
	Code     int    `json:"code" choice:"0:draft" choice:"10:active" enumAccept:"literal" enumStyle:"literal"`
	Label    string `json:"label" choice:"0:draft" choice:"10:active" enumAccept:"ordinal" enumStyle:"ordinal"`
	Auto     int    `json:"auto" choice:"draft" choice:"active" enumAccept:"literal" enumStyle:"ordinal"`
	Nullable *int   `json:"nullable" choice:"0:draft" choice:"10:active" enumAccept:"literal" enumStyle:"literal"`
}

func TestEnumJSONOppositeStorageAndNestedShapes(t *testing.T) {
	type envelope struct {
		Rows   []*enumRecord          `json:"rows"`
		ByName map[string]*enumRecord `json:"byName"`
	}
	raw := []byte(`{"rows":[{"code":"active","label":10,"auto":"draft","nullable":null}],"byName":{"a":{"code":"draft","label":0,"auto":"active","nullable":"active"}}}`)
	for _, standard := range []bool{false, true} {
		var value envelope
		var err error
		if standard {
			err = UnmarshalStandard(raw, &value)
		} else {
			err = Unmarshal(raw, &value)
		}
		if err != nil {
			t.Fatalf("standard=%v: %v", standard, err)
		}
		if value.Rows[0].Code != 10 || value.Rows[0].Label != "active" || value.Rows[0].Auto != 0 || value.Rows[0].Nullable != nil || *value.ByName["a"].Nullable != 10 {
			t.Fatalf("%+v", value)
		}
		for _, mode := range []string{"standard", "native", "formatted", "compiled"} {
			var encoded []byte
			switch mode {
			case "standard":
				encoded, err = MarshalStandard(&value)
			case "native":
				encoded, err = Marshal(&value)
			case "formatted":
				encoded, err = Marshal(&value, WithCaseFormat(text.CaseFormatLowerCamel))
			case "compiled":
				m, e := NewMarshaller(reflect.TypeFor[envelope]())
				if e != nil {
					t.Fatal(e)
				}
				encoded, err = m.Marshal(&value)
			}
			if err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			var object map[string]any
			if err = stdjson.Unmarshal(encoded, &object); err != nil {
				t.Fatal(err)
			}
			row := object["rows"].([]any)[0].(map[string]any)
			if row["code"] != "active" || row["label"] != float64(10) || row["auto"] != float64(0) || row["nullable"] != nil {
				t.Fatalf("%s: %s", mode, encoded)
			}
		}
	}
}
func TestEnumRejectsWrongInputRepresentation(t *testing.T) {
	for _, raw := range []string{`{"code":10}`, `{"label":"active"}`, `{"code":"missing"}`, `{"label":99}`} {
		for _, decode := range []func([]byte, any) error{
			func(raw []byte, target any) error { return Unmarshal(raw, target) },
			func(raw []byte, target any) error { return UnmarshalStandard(raw, target) },
		} {
			var value enumRecord
			if err := decode([]byte(raw), &value); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
	}
}
