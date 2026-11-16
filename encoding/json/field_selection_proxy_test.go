package json_test

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	structjson "github.com/viant/structology/encoding/json"
	"github.com/viant/x/shape"
)

type proxySelectionRow struct {
	ID    int    `json:"id"`
	Fee   int    `json:"fee"`
	Other string `json:"other"`
}
type childSelectionRoot struct {
	Rows []proxySelectionRow `json:"data"`
}
type parentSelectionRoot childSelectionRoot

func TestFieldFilterUsesDefinedProxyRoot(t *testing.T) {
	child := childSelectionRoot{Rows: []proxySelectionRow{{ID: 7, Fee: 19, Other: "unselected"}}}
	parent := parentSelectionRoot(child)
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(child), []structjson.FieldSelection{
		{Path: []string{"Rows"}, Indexes: [][]int{{0}, {1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	walk, err := shape.CompileRewrite(reflect.TypeOf(&parent))
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, hide := range []bool{false, true} {
		hide := hide
		rep, err := walk.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			field := *f
			switch f.Name {
			case "Rows":
				field.Tag = `json:"records"`
			case "ID":
				field.Tag = `json:"identifier"`
			case "Fee":
				if !hide {
					return nil, nil
				}
				field.Tag = `json:"-"`
			default:
				return nil, nil
			}
			return &field, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			projected, err := filter.WithOptions(structjson.WithRepresentation(rep))
			if err != nil {
				t.Error(err)
				return
			}
			if !projected.ExcludeIndexes([]int{0, 2}) {
				t.Error("unselected index became visible")
				return
			}
			buf, err := structjson.Marshal(&parent, structjson.WithRepresentation(rep), structjson.WithPathFieldExcluder(projected))
			if err != nil {
				t.Error(err)
				return
			}
			expected := map[string]int{"identifier": 7}
			if !hide {
				expected["fee"] = 19
			}
			var body map[string][]map[string]int
			if err := json.Unmarshal(buf, &body); err != nil || !reflect.DeepEqual(body, map[string][]map[string]int{"records": {expected}}) {
				t.Errorf("proxy output %s (error %v)", buf, err)
			}
		}()
	}
	wg.Wait()
	buf, err := structjson.MarshalStandard(child, structjson.WithPathFieldExcluder(filter))
	if err != nil || string(buf) != `{"data":[{"id":7,"fee":19}]}` {
		t.Errorf("child filter changed: %s (error %v)", buf, err)
	}
}

type proxyCustomJSON childSelectionRoot

func (proxyCustomJSON) MarshalJSON() ([]byte, error) { return []byte(`"custom"`), nil }

type proxyCustomText childSelectionRoot

func (*proxyCustomText) MarshalText() ([]byte, error) { return []byte("custom"), nil }

type proxyReorderedA struct {
	A int
	B int
}
type proxyReorderedB struct {
	B int
	A int
}

type proxyDifferentTag struct {
	Rows []proxySelectionRow `json:"rows"`
}
type proxyDifferentType struct {
	Rows []*proxySelectionRow `json:"data"`
}
type proxyDifferentName struct {
	Values []proxySelectionRow `json:"data"`
}
type proxyExtraField struct {
	Rows  []proxySelectionRow `json:"data"`
	Other int
}
type ProxyEmbedMember struct{ Count int }
type proxyEmbeddedRow struct{ ProxyEmbedMember }
type proxyNamedRow struct{ ProxyEmbedMember ProxyEmbedMember }

func TestFieldFilterRejectsDifferentProxyLayouts(t *testing.T) {
	for _, tc := range []struct{ from, to reflect.Type }{
		{reflect.TypeFor[childSelectionRoot](), reflect.TypeFor[proxyDifferentTag]()},
		{reflect.TypeFor[childSelectionRoot](), reflect.TypeFor[proxyDifferentType]()},
		{reflect.TypeFor[childSelectionRoot](), reflect.TypeFor[proxyDifferentName]()},
		{reflect.TypeFor[childSelectionRoot](), reflect.TypeFor[proxyExtraField]()},
		{reflect.TypeFor[proxyEmbeddedRow](), reflect.TypeFor[proxyNamedRow]()},
		{reflect.TypeFor[proxyReorderedA](), reflect.TypeFor[proxyReorderedB]()},
		{reflect.TypeFor[proxyCustomJSON](), reflect.TypeFor[parentSelectionRoot]()},
		{reflect.TypeFor[proxyCustomText](), reflect.TypeFor[parentSelectionRoot]()},
	} {
		filter, err := structjson.NewFieldFilter(tc.from, nil)
		if err != nil {
			t.Fatal(err)
		}
		walk, err := shape.CompileRewrite(tc.to)
		if err != nil {
			t.Fatal(err)
		}
		rep, err := walk.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			copy := *f
			copy.Tag = reflect.StructTag(string(f.Tag) + ` description:"parent contract"`)
			return &copy, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if rep == nil {
			t.Fatalf("missing test representation for %s", tc.to)
		}
		if _, err := filter.WithOptions(structjson.WithRepresentation(rep)); err == nil {
			t.Errorf("allowed %s -> %s", tc.from, tc.to)
		}
	}
}
