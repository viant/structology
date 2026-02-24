package json

import (
	"testing"
)

type benchBasic struct {
	ID   int
	Name string
	Flag bool
}

type benchAdvanced struct {
	ID      int
	Name    string
	Score   float64
	Tags    []string
	Payload map[string]interface{}
	Child   *benchBasic
}

func BenchmarkMarshal_Basic(b *testing.B) {
	in := benchBasic{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_Basic(b *testing.B) {
	data := []byte(`{"ID":7,"Name":"alpha","Flag":true}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out benchBasic
		if err := Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshal_Advanced(b *testing.B) {
	in := benchAdvanced{
		ID:    11,
		Name:  "beta",
		Score: 99.1,
		Tags:  []string{"x", "y", "z"},
		Payload: map[string]interface{}{
			"k1": 1,
			"k2": "v2",
		},
		Child: &benchBasic{ID: 1, Name: "child", Flag: true},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_Advanced(b *testing.B) {
	data := []byte(`{"ID":11,"Name":"beta","Score":99.1,"Tags":["x","y","z"],"Payload":{"k1":1,"k2":"v2"},"Child":{"ID":1,"Name":"child","Flag":true}}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out benchAdvanced
		if err := Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkScanner_SkipWhitespace(b *testing.B) {
	hooks := scalarScannerHooks{}
	data := []byte("    \n\t  {\"x\":1}")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hooks.SkipWhitespace(data, 0)
	}
}

func BenchmarkScanner_FindStructural(b *testing.B) {
	hooks := scalarScannerHooks{}
	data := []byte("\"name\":\"value\",\"n\":123}")
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = hooks.FindStructural(data, 0)
	}
}
