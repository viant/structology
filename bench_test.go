package structology

import (
    "reflect"
    "testing"
)

// Benchmark simple Set/Get on a primitive field with marker enabled.
func BenchmarkState_SetGet_Primitive(b *testing.B) {
    type FooHas struct {
        Id   bool
        Name bool
    }
    type Foo struct {
        Id   int
        Name string
        Has  *FooHas `setMarker:"true"`
    }
    foo := &Foo{Id: 1, Has: &FooHas{}}
    st := NewStateType(reflect.TypeOf(foo))
    state := st.WithValue(foo)
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = state.SetString("Name", "John")
        _, _ = state.String("Name")
    }
}

// Benchmark Set/Get on a slice element field using WithPathIndex.
func BenchmarkState_SetGet_SliceIndex(b *testing.B) {
    type Item struct{ Name string }
    type HolderHas struct{ Items bool }
    type Holder struct {
        Items []Item
        Has   *HolderHas `setMarker:"true"`
    }
    h := &Holder{Items: make([]Item, 100), Has: &HolderHas{}}
    st := NewStateType(reflect.TypeOf(h))
    state := st.WithValue(h)
    idx := 50
    opt := WithPathIndex(idx)
    b.ReportAllocs()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = state.SetString("Items.Name", "X", opt)
        _, _ = state.String("Items.Name", opt)
    }
}

