package json

import (
	"bytes"
	stdjson "encoding/json"
	"github.com/viant/x/shape"
	"reflect"
	"testing"
)

func TestMarshalDirectInterfaceStructStorage(t *testing.T) {
	type row struct {
		Value int `json:"value"`
	}
	type mapRoot struct {
		Rows map[string][][]*row `json:"rows"`
	}
	type ptrRoot struct {
		Row *row `json:"row"`
	}
	type arrayRoot struct {
		Rows [1]*row `json:"rows"`
	}
	for _, value := range []any{mapRoot{map[string][][]*row{"a": {{{Value: 3}}}}}, ptrRoot{&row{3}}, arrayRoot{[1]*row{{Value: 3}}}} {
		walker, err := shape.CompileRewrite(reflect.TypeOf(value))
		if err != nil {
			t.Fatal(err)
		}
		metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			if parent == reflect.TypeFor[row]() {
				return &shape.FieldMetadata{Name: f.Name, Tag: `json:"renamed"`}, nil
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, opts := range [][]Option{nil, {WithRepresentation(metadata)}} {
			raw, err := Marshal(value, opts...)
			if err != nil {
				t.Fatal(err)
			}
			var actual any
			if err = stdjson.Unmarshal(raw, &actual); err != nil {
				t.Fatal(err)
			}
			expected, err := stdjson.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if len(opts) > 0 {
				expected = bytes.ReplaceAll(expected, []byte(`"value"`), []byte(`"renamed"`))
			}
			var wanted any
			if err = stdjson.Unmarshal(expected, &wanted); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(wanted, actual) {
				t.Fatalf("got %s want %s", raw, expected)
			}
		}
	}
}
