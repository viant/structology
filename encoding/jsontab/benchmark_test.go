package jsontab

import (
	"bytes"
	"encoding/csv"
	"strconv"
	"testing"
)

type benchChild struct {
	ID   int    `csvName:"id"`
	Name string `csvName:"name"`
}

type benchRec struct {
	ID       int          `csvName:"id"`
	Name     string       `csvName:"name"`
	Active   bool         `csvName:"active"`
	Children []benchChild `csvName:"children"`
}

type benchFlat struct {
	ID     int    `csvName:"id"`
	Name   string `csvName:"name"`
	Active bool   `csvName:"active"`
}

func benchData(n int) []benchRec {
	out := make([]benchRec, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, benchRec{
			ID:     i + 1,
			Name:   "name",
			Active: i%2 == 0,
			Children: []benchChild{
				{ID: i + 100, Name: "c1"},
				{ID: i + 200, Name: "c2"},
			},
		})
	}
	return out
}

func benchFlatData(n int) []benchFlat {
	out := make([]benchFlat, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, benchFlat{
			ID:     i + 1,
			Name:   "name",
			Active: i%2 == 0,
		})
	}
	return out
}

func benchFlatCSV(data []benchFlat) []byte {
	buf := bytes.NewBuffer(make([]byte, 0, len(data)*32))
	w := csv.NewWriter(buf)
	_ = w.Write([]string{"id", "name", "active"})
	for _, item := range data {
		_ = w.Write([]string{
			strconv.Itoa(item.ID),
			item.Name,
			strconv.FormatBool(item.Active),
		})
	}
	w.Flush()
	return buf.Bytes()
}

func BenchmarkMarshal_JSonTab(b *testing.B) {
	data := benchData(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_JSonTab(b *testing.B) {
	data := benchData(200)
	payload, err := Marshal(data)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out []benchRec
		if err = Unmarshal(payload, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshal_JSonTab_Compare(b *testing.B) {
	data := benchFlatData(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_JSonTab_Compare(b *testing.B) {
	data := benchFlatData(200)
	payload := benchFlatCSV(data)
	var err error
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out []benchFlat
		if err = Unmarshal(payload, &out); err != nil {
			b.Fatal(err)
		}
	}
}
