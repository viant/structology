package conv

import (
    "reflect"
    "testing"
    "time"
)

type benchStruct struct {
    Name       string
    Age        int
    Active     bool
    Score      float64
    DateJoined time.Time
}

func BenchmarkConverter_MapToStruct(b *testing.B) {
    c := NewConverter(DefaultOptions())
    src := map[string]interface{}{
        "Name":       "Jane",
        "Age":        42,
        "Active":     true,
        "Score":      99.5,
        "DateJoined": "2023-01-15T12:30:45Z",
    }
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var dst benchStruct
        if err := c.Convert(src, &dst); err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkConverter_SliceMapToSliceStruct(b *testing.B) {
    c := NewConverter(DefaultOptions())
    one := map[string]interface{}{
        "Name":       "Jane",
        "Age":        42,
        "Active":     true,
        "Score":      99.5,
        "DateJoined": "2023-01-15T12:30:45Z",
    }
    src := make([]interface{}, 256)
    for i := range src {
        src[i] = one
    }
    // Destination slice type
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var dst []benchStruct
        if err := c.Convert(src, &dst); err != nil {
            b.Fatal(err)
        }
        // Prevent compiler from optimizing away
        _ = reflect.ValueOf(dst).Len()
    }
}

