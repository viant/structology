package csv

import (
	"reflect"
	"strconv"
	"testing"
)

func BenchmarkMarshal1000(b *testing.B) {
	type row struct {
		ID     int
		Name   string
		Active bool
		Score  float64
	}
	rows := make([]row, 1000)
	for i := range rows {
		rows[i] = row{i, "record " + strconv.Itoa(i), true, float64(i) / 10}
	}
	m, err := NewMarshaller(reflect.TypeFor[row](), nil)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = m.Marshal(rows); err != nil {
			b.Fatal(err)
		}
	}
}
func FuzzCSVStrings(f *testing.F) {
	for _, seed := range []string{"", `a,"b"\c`, "one\ntwo", "one\r\ntwo", "null", "日本語"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		type row struct{ Value string }
		data, err := Marshal([]row{{value}})
		if err != nil {
			t.Fatal(err)
		}
		var output []row
		if err = Unmarshal(data, &output); err != nil {
			t.Fatalf("roundtrip failed: %v", err)
		}
		// encoding/csv normalizes CRLF inside quoted records.
		expected := normalizeCRLF(value)
		if len(output) != 1 || output[0].Value != expected {
			t.Fatalf("got %#v; want %q", output, expected)
		}
	})
}
func normalizeCRLF(value string) string {
	result := make([]byte, 0, len(value))
	for i := 0; i < len(value); i++ {
		if value[i] == '\r' && i+1 < len(value) && value[i+1] == '\n' {
			continue
		}
		result = append(result, value[i])
	}
	return string(result)
}
