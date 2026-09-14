package marshal

import (
	"reflect"
	"testing"
)

type docsEmbedded struct {
	ID int `json:"identifier" desc:"authored"`
}
type docsEnvelope struct {
	docsEmbedded
	Name string `json:"displayName"`
}

func TestWirePropertySourceOwnership(t *testing.T) {
	wire, err := NewStandard(reflect.TypeFor[docsEnvelope]()).Wire()
	if err != nil {
		t.Fatal(err)
	}
	field := wire.Properties()[0].Field()
	if field.Name != "ID" || field.Tag.Get("desc") != "authored" || len(field.Index) != 2 {
		t.Fatal(field)
	}
	field.Index[0] = 99
	if wire.Properties()[0].Field().Index[0] == 99 {
		t.Fatal("mutable index escaped")
	}
}
