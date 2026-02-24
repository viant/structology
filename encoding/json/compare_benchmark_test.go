package json

import (
	stdjson "encoding/json"
	"testing"
)

type compareBasic struct {
	ID   int
	Name string
	Flag bool
}

type compareAdvanced struct {
	ID      int
	Name    string
	Score   float64
	Tags    []string
	Payload map[string]string
	Child   *compareBasic
}

func BenchmarkCompare_Marshal_Basic_Structology(b *testing.B) {
	in := compareBasic{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Marshal_Basic_Stdlib(b *testing.B) {
	in := compareBasic{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := stdjson.Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Unmarshal_Basic_Structology(b *testing.B) {
	data := []byte(`{"ID":7,"Name":"alpha","Flag":true}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out compareBasic
		if err := Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Unmarshal_Basic_Stdlib(b *testing.B) {
	data := []byte(`{"ID":7,"Name":"alpha","Flag":true}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out compareBasic
		if err := stdjson.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Marshal_Advanced_Structology(b *testing.B) {
	in := compareAdvanced{
		ID:    11,
		Name:  "beta",
		Score: 99.1,
		Tags:  []string{"x", "y", "z"},
		Payload: map[string]string{
			"k1": "1",
			"k2": "v2",
		},
		Child: &compareBasic{ID: 1, Name: "child", Flag: true},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Marshal_Advanced_Stdlib(b *testing.B) {
	in := compareAdvanced{
		ID:    11,
		Name:  "beta",
		Score: 99.1,
		Tags:  []string{"x", "y", "z"},
		Payload: map[string]string{
			"k1": "1",
			"k2": "v2",
		},
		Child: &compareBasic{ID: 1, Name: "child", Flag: true},
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := stdjson.Marshal(in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Unmarshal_Advanced_Structology(b *testing.B) {
	data := []byte(`{"ID":11,"Name":"beta","Score":99.1,"Tags":["x","y","z"],"Payload":{"k1":"1","k2":"v2"},"Child":{"ID":1,"Name":"child","Flag":true}}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out compareAdvanced
		if err := Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompare_Unmarshal_Advanced_Stdlib(b *testing.B) {
	data := []byte(`{"ID":11,"Name":"beta","Score":99.1,"Tags":["x","y","z"],"Payload":{"k1":"1","k2":"v2"},"Child":{"ID":1,"Name":"child","Flag":true}}`)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var out compareAdvanced
		if err := stdjson.Unmarshal(data, &out); err != nil {
			b.Fatal(err)
		}
	}
}
