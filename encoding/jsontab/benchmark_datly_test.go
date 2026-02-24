//go:build datlybench

package jsontab

import (
	"reflect"
	"testing"

	datlytab "github.com/viant/datly/gateway/router/marshal/tabjson"
)

func BenchmarkMarshal_DatlyTabJson_Compare(b *testing.B) {
	data := benchFlatData(200)
	m, err := datlytab.NewMarshaller(reflect.TypeOf(benchFlat{}), &datlytab.Config{})
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err = m.Marshal(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshal_DatlyTabJson_Compare(b *testing.B) {
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
