package marshal

import (
	"github.com/viant/x/shape"
	"reflect"
	"strings"
	"testing"
)

type enumChild struct {
	Mode int `json:"mode"`
}
type enumParent struct {
	Value   enumChild  `json:"value"`
	Pointer *enumChild `json:"pointer"`
}

func TestCompiledChildrenKeepEnumRepresentation(t *testing.T) {
	walker, err := shape.CompileRewrite(reflect.TypeFor[enumParent]())
	if err != nil {
		t.Fatal(err)
	}
	representation, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == reflect.TypeFor[enumChild]() && f.Name == "Mode" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"mode" choice:"0:draft" choice:"10:active" enumStyle:"literal"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	value := enumParent{enumChild{10}, &enumChild{0}}
	engine := New(nil, nil, false, true, "", "lc", strings.ToLower).WithRepresentation(representation)
	raw, err := engine.MarshalPtrTo(nil, &value)
	if err != nil || string(raw) != `{"value":{"mode":"active"},"pointer":{"mode":"draft"}}` {
		t.Fatalf("compiled child representation: %s %v", raw, err)
	}
}
