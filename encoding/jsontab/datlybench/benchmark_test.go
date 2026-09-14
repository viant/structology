package datlybench

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strconv"
	"testing"

	datlytab "github.com/viant/datly/gateway/router/marshal/tabjson"
	structtab "github.com/viant/structology/encoding/jsontab"
)

type benchFlat struct {
	ID     int    `csvName:"id"`
	Name   string `csvName:"name"`
	Active bool   `csvName:"active"`
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

func BenchmarkMarshalStructologyJSonTab(b *testing.B) {
	data := benchFlatData(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := structtab.Marshal(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarshalDatlyTabJSON(b *testing.B) {
	data := benchFlatData(200)
	m, err := datlytab.NewMarshaller(reflect.TypeOf(benchFlat{}), &datlytab.Config{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = m.Marshal(data); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalStructologyJSonTab(b *testing.B) {
	payload := benchFlatCSV(benchFlatData(200))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out []benchFlat
		if err := structtab.Unmarshal(payload, &out); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalDatlyTabJSON(b *testing.B) {
	data := benchFlatData(200)
	m, err := datlytab.NewMarshaller(reflect.TypeOf(benchFlat{}), &datlytab.Config{})
	if err != nil {
		b.Fatal(err)
	}
	payload := benchFlatCSV(data)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out []benchFlat
		if err = m.Unmarshal(payload, &out); err != nil {
			b.Fatal(err)
		}
	}
}
