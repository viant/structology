package json_test

import (
	"fmt"
	"reflect"
	"testing"

	native "github.com/viant/structology/encoding/json"
	"github.com/viant/tagly/format/text"
)

type traversalBenchmarkNode struct {
	ID       int
	Name     string
	Children []*traversalBenchmarkNode
}

func traversalBenchmarkRows(n, depth int) []*traversalBenchmarkNode {
	shared := &traversalBenchmarkNode{ID: 9, Name: "shared"}
	rows := make([]*traversalBenchmarkNode, n)
	for i := range rows {
		node := &traversalBenchmarkNode{ID: i, Name: "row", Children: []*traversalBenchmarkNode{shared, shared}}
		for j := 0; j < depth; j++ {
			node = &traversalBenchmarkNode{ID: i, Name: "parent", Children: []*traversalBenchmarkNode{node}}
		}
		rows[i] = node
	}
	return rows
}
func BenchmarkNativeTraversalFullMarshal(b *testing.B) {
	values := []struct {
		name  string
		value any
	}{{"scalar", int64(42)}, {"flat", &struct {
		ID     int
		Name   string
		Active bool
	}{42, "row", true}}}
	for _, n := range []int{1, 100, 1000} {
		for _, depth := range []int{1, 5} {
			values = append(values, struct {
				name  string
				value any
			}{fmt.Sprintf("recursive/rows%d/depth%d", n, depth), traversalBenchmarkRows(n, depth)})
		}
	}
	for _, item := range values {
		for _, mode := range []string{"static", "case"} {
			b.Run(item.name+"/"+mode, func(b *testing.B) {
				var opts []native.Option
				if mode == "case" {
					opts = []native.Option{native.WithCaseFormat(text.CaseFormatLowerCamel)}
				}
				b.Run("warm", func(b *testing.B) {
					m, err := native.NewMarshaller(reflect.TypeOf(item.value), opts...)
					if err != nil {
						b.Fatal(err)
					}
					if _, err = m.Marshal(item.value); err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						if _, err = m.Marshal(item.value); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("new-prepared", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						m, err := native.NewMarshaller(reflect.TypeOf(item.value), opts...)
						if err != nil {
							b.Fatal(err)
						}
						if _, err = m.Marshal(item.value); err != nil {
							b.Fatal(err)
						}
					}
				})
			})
		}
	}
}
