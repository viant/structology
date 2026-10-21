package json_test

import (
	sjson "github.com/viant/structology/encoding/json"
	"github.com/viant/structology/encoding/json/marshal"
	"reflect"
	"testing"
)

type IndexLeaf struct {
	ID int `json:"identifier" desc:"authored identifier"`
}
type IndexMiddle struct {
	Padding string
	*IndexLeaf
}
type IndexRoot struct {
	Lead string
	*IndexMiddle
	Tail bool
}
type IndexInline struct {
	Lead   string    `json:"lead"`
	Fields IndexLeaf `jsonx:"inline"`
}
type IndexNamed struct {
	Lead  string
	Child IndexLeaf `json:"child"`
}

func TestTransformedWireFieldIndexesAreRooted(t *testing.T) {
	for _, test := range []struct {
		name     string
		typ      reflect.Type
		property string
		index    []int
	}{
		{"anonymous", reflect.TypeFor[struct {
			Lead string
			IndexLeaf
		}](), "identifier", []int{1, 0}},
		{"pointer chain", reflect.TypeFor[IndexRoot](), "identifier", []int{1, 1, 0}},
		{"explicit inline", reflect.TypeFor[IndexInline](), "identifier", []int{1, 0}},
		{"named holder", reflect.TypeFor[IndexNamed](), "child", []int{1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoder, err := sjson.NewMarshaller(test.typ)
			if err != nil {
				t.Fatal(err)
			}
			wire, err := encoder.Wire()
			if err != nil {
				t.Fatal(err)
			}
			var found *marshal.WireProperty
			for _, property := range wire.Properties() {
				if property.Name() == test.property {
					copy := property
					found = &copy
					break
				}
			}
			if found == nil {
				t.Fatal("property absent")
			}
			field := found.Field()
			if !reflect.DeepEqual(field.Index, test.index) {
				t.Fatalf("index=%v want=%v", field.Index, test.index)
			}
			selected := wire.Source().FieldByIndex(field.Index)
			if selected.Name != field.Name || selected.Type != field.Type || selected.Tag != field.Tag {
				t.Fatalf("index selected wrong field: %v vs %v", selected, field)
			}
			field.Index[0] = 99
			if !reflect.DeepEqual(found.Field().Index, test.index) {
				t.Fatal("mutable index escaped")
			}
			if test.property == "child" {
				nested := found.Shape().Properties()[0].Field()
				if !reflect.DeepEqual(nested.Index, []int{0}) {
					t.Fatal("named child inherited parent root index", nested)
				}
			}
		})
	}
}
