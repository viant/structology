package marshal

import (
	stdjson "encoding/json"
	"testing"
)

type benchSample struct {
	ID   int
	Name string
	Flag bool
}

func BenchmarkEngine_Marshal(b *testing.B) {
	e := New(nil, nil, false, true, "", "", nil)
	in := benchSample{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := e.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_Marshal(b *testing.B) {
	in := benchSample{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_MarshalTo_ReuseBuffer(b *testing.B) {
	e := New(nil, nil, false, true, "", "", nil)
	in := benchSample{ID: 7, Name: "alpha", Flag: true}
	buf := make([]byte, 0, 256)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		var err error
		buf, err = e.MarshalTo(buf, in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEngine_MarshalPtrTo_ReuseBuffer(b *testing.B) {
	e := New(nil, nil, false, true, "", "", nil)
	in := &benchSample{ID: 7, Name: "alpha", Flag: true}
	buf := make([]byte, 0, 256)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		buf = buf[:0]
		var err error
		buf, err = e.MarshalPtrTo(buf, in)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStdlib_MarshalPtr(b *testing.B) {
	in := &benchSample{ID: 7, Name: "alpha", Flag: true}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := stdjson.Marshal(in); err != nil {
			b.Fatal(err)
		}
	}
}
