package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"

	"github.com/viant/x/shape"
)

type standardMetadataRow struct {
	*RepresentationEmbedded
	Amount int64  `json:"amount"`
	Quoted int    `json:"quoted,string"`
	Hidden string `json:"hidden"`
}
type standardMetadataInput struct {
	Rows []*standardMetadataRow       `json:"rows"`
	ByID map[int]*standardMetadataRow `json:"byId"`
}

func TestStandardDecoderOptInMetadataWithLinkedEmbeddings(t *testing.T) {
	walker, _ := shape.CompileRewrite(reflect.TypeFor[standardMetadataInput]())
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Hidden" || f.Name == "Secret" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		if f.Name == "Amount" {
			return &shape.FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"rows":[{"name":"first","visible":1,"visible":9007199254740993,"quoted":"7","hidden":"blocked","secret":"blocked"},null],"byId":{"2":{"visible":8},"3":null}}`)
	var actual standardMetadataInput
	if err := UnmarshalStandard(raw, &actual, WithRepresentation(metadata)); err != nil {
		t.Fatal(err)
	}
	first := actual.Rows[0]
	if first.Amount != 9007199254740993 || first.Quoted != 7 || first.Name != "first" || first.Identity() != "linked" {
		t.Fatalf("original typed decoding lost: %+v", first)
	}
	if first.Hidden != "" || first.Secret != "" || actual.Rows[1] != nil || actual.ByID[3] != nil || actual.ByID[2].Amount != 8 {
		t.Fatal("metadata/container behavior lost")
	}
	// Repeated decoding retains native slice-element reuse and pointer nulling.
	if err := UnmarshalStandard([]byte(`{"rows":[{"visible":2}]}`), &actual, WithRepresentation(metadata)); err != nil {
		t.Fatal(err)
	}
	if len(actual.Rows) != 1 || actual.Rows[0] != first || first.Name != "first" || first.Quoted != 7 || first.Amount != 2 {
		t.Fatal("standard slice reuse changed")
	}
	if err := UnmarshalStandard([]byte(`{"rows":null}`), &actual, WithRepresentation(metadata)); err != nil || actual.Rows != nil {
		t.Fatal("null slice did not clear", err)
	}
}
func TestStandardDecoderNoopAndSyntaxOwnership(t *testing.T) {
	walker, _ := shape.CompileRewrite(reflect.TypeFor[standardMetadataInput]())
	unchanged, _ := walker.RewriteRepresentation(nil)
	raw := []byte(`{"rows":[{"amount":9007199254740993,"quoted":"3","hidden":"allowed"}]}`)
	var want, actual standardMetadataInput
	if err := stdjson.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if err := UnmarshalStandard(raw, &actual, WithRepresentation(unchanged)); err != nil || !reflect.DeepEqual(actual, want) {
		t.Fatal("no-op changed decoder", err)
	}
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Hidden" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := actual.Rows[0]
	if err := UnmarshalStandard([]byte(`{"rows":[{"amount":1}],"broken":`), &actual, WithRepresentation(metadata)); err == nil {
		t.Fatal("malformed suffix accepted")
	}
	if actual.Rows[0] != before || before.Amount != 9007199254740993 {
		t.Fatal("syntax error mutated destination")
	}
}
