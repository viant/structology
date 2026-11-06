package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/viant/tagly/format/text"
	"github.com/viant/x/shape"
)

type representationExcluder struct{}

func (representationExcluder) ExcludePath([]string, string) bool { return false }
func TestRepresentationOptionPublicCodecs(t *testing.T) {
	typ := reflect.TypeFor[representationEnvelope]()
	walker, _ := shape.CompileRewrite(typ)
	effective, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		switch f.Name {
		case "Secret", "Private":
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-" internal:"true"`}, nil
		case "Value":
			return &shape.FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	value := &representationEnvelope{Rows: []*representationRecord{{RepresentationEmbedded: &RepresentationEmbedded{Name: "linked", Secret: "blocked"}, Value: 7, Private: "blocked"}, nil}, ByName: map[string]*representationRecord{"a": {Value: 2}}}
	for _, mode := range []string{"standard", "native", "formatted", "dynamic", "compiled"} {
		t.Run(mode, func(t *testing.T) {
			options := []Option{WithRepresentation(effective)}
			if mode == "formatted" {
				options = append(options, WithCaseFormat(text.CaseFormatLowerCamel))
			}
			if mode == "dynamic" {
				options = append(options, WithPathFieldExcluder(representationExcluder{}))
			}
			var encoded []byte
			var err error
			switch mode {
			case "standard":
				encoded, err = MarshalStandard(value, options...)
			case "compiled":
				encoder, e := NewMarshaller(typ, options...)
				if e != nil {
					t.Fatal(e)
				}
				encoded, err = encoder.Marshal(value)
				wire, e := encoder.Wire()
				if e != nil {
					t.Fatal(e)
				}
				if wire.Representation() != effective {
					t.Fatal("wire lost opted metadata")
				}
			default:
				encoded, err = Marshal(value, options...)
			}
			if err != nil {
				t.Fatal(err)
			}
			var public map[string]any
			if err = stdjson.Unmarshal(encoded, &public); err != nil {
				t.Fatal(err)
			}
			row := public["rows"].([]any)[0].(map[string]any)
			if row["visible"] != float64(7) || row["secret"] != nil || row["private"] != nil || row["name"] != "linked" {
				t.Fatalf("%s", encoded)
			}
			var decoded representationEnvelope
			if err = Unmarshal(encoded, &decoded, options...); err != nil {
				t.Fatal(err)
			}
			if decoded.Rows[0].Value != 7 || decoded.Rows[0].Identity() != "linked" || decoded.Rows[0].Has == nil || !decoded.Rows[0].Has.Value {
				t.Fatal("original receiver/presence lost")
			}
			if decoded.ByName["a"].Value != 2 {
				t.Fatal("map metadata lost")
			}
		})
	}
	if value.Rows[0].Secret != "blocked" || value.Rows[0].Private != "blocked" {
		t.Fatal("source mutated")
	}
	// Explicit nil and unchanged metadata opt into no new behavior.
	linked, _ := walker.RewriteRepresentation(nil)
	base, _ := Marshal(value)
	for _, metadata := range []*shape.Representation{nil, linked} {
		actual, err := Marshal(value, WithRepresentation(metadata))
		if err != nil {
			t.Fatal(err)
		}
		var left, right any
		_ = stdjson.Unmarshal(base, &left)
		_ = stdjson.Unmarshal(actual, &right)
		if !reflect.DeepEqual(left, right) {
			t.Fatal("no-op metadata changed baseline output")
		}
	}
	if _, err := Marshal("wrong", WithRepresentation(effective)); err == nil {
		t.Fatal("storage mismatch accepted")
	}
}

type metadataZeroOwner struct {
	Value int
	Label string
}

func (v metadataZeroOwner) IsZero() bool { return v.Value == 0 }
func TestMetadataStandardEncodingRetainsIsZero(t *testing.T) {
	type root struct {
		Item metadataZeroOwner `json:"item,omitzero"`
	}
	walker, _ := shape.CompileRewrite(reflect.TypeFor[root]())
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == reflect.TypeFor[metadataZeroOwner]() && f.Name == "Value" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		value int
		want  string
	}{{0, `{}`}, {7, `{"item":{"Label":"visible"}}`}} {
		encoded, err := MarshalStandard(root{Item: metadataZeroOwner{Value: test.value, Label: "visible"}}, WithRepresentation(metadata))
		if err != nil || string(encoded) != test.want {
			t.Fatalf("%s %v", encoded, err)
		}
	}
}
