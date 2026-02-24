package json

import (
	stdjson "encoding/json"
	"testing"
)

type benchNested struct {
	ID    int
	Name  string
	Child *benchNested
}

type benchMapHeavy struct {
	ID      int
	Labels  map[string]string
	Payload map[string]interface{}
}

type benchSliceHeavy struct {
	ID     int
	Values []int
	Names  []string
	Flags  []bool
}

type benchIfaceHeavy struct {
	ID      int
	Any     interface{}
	AnyList []interface{}
}

func BenchmarkShape_Marshal_Nested_Structology(b *testing.B) {
	in := &benchNested{ID: 1, Name: "root", Child: &benchNested{ID: 2, Name: "child"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_Nested_Stdlib(b *testing.B) {
	in := &benchNested{ID: 1, Name: "root", Child: &benchNested{ID: 2, Name: "child"}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_MapHeavy_Structology(b *testing.B) {
	in := &benchMapHeavy{
		ID:     1,
		Labels: map[string]string{"a": "x", "b": "y", "c": "z"},
		Payload: map[string]interface{}{
			"k1": 1, "k2": "v2", "k3": true, "k4": []int{1, 2, 3},
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_MapHeavy_Stdlib(b *testing.B) {
	in := &benchMapHeavy{
		ID:     1,
		Labels: map[string]string{"a": "x", "b": "y", "c": "z"},
		Payload: map[string]interface{}{
			"k1": 1, "k2": "v2", "k3": true, "k4": []int{1, 2, 3},
		},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_SliceHeavy_Structology(b *testing.B) {
	in := &benchSliceHeavy{ID: 1, Values: []int{1, 2, 3, 4, 5, 6, 7, 8}, Names: []string{"a", "b", "c"}, Flags: []bool{true, false, true}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_SliceHeavy_Stdlib(b *testing.B) {
	in := &benchSliceHeavy{ID: 1, Values: []int{1, 2, 3, 4, 5, 6, 7, 8}, Names: []string{"a", "b", "c"}, Flags: []bool{true, false, true}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_InterfaceHeavy_Structology(b *testing.B) {
	in := &benchIfaceHeavy{ID: 1, Any: map[string]interface{}{"a": 1, "b": "x"}, AnyList: []interface{}{1, "x", true, []int{1, 2}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkShape_Marshal_InterfaceHeavy_Stdlib(b *testing.B) {
	in := &benchIfaceHeavy{ID: 1, Any: map[string]interface{}{"a": 1, "b": "x"}, AnyList: []interface{}{1, "x", true, []int{1, 2}}}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}
