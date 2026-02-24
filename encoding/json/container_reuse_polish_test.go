package json

import (
	"testing"
	"unsafe"
)

func TestContainerReuse_TypedSlices(t *testing.T) {
	type payload struct {
		Ints  []int
		Int64 []int64
		F64   []float64
		Flags []bool
	}
	out := payload{
		Ints:  make([]int, 0, 8),
		Int64: make([]int64, 0, 8),
		F64:   make([]float64, 0, 8),
		Flags: make([]bool, 0, 8),
	}
	intsPtr := unsafe.SliceData(out.Ints)
	int64Ptr := unsafe.SliceData(out.Int64)
	f64Ptr := unsafe.SliceData(out.F64)
	flagsPtr := unsafe.SliceData(out.Flags)

	data := []byte(`{"Ints":[1,2,3],"Int64":[4,5],"F64":[1.5,2.5],"Flags":[true,false,true]}`)
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(out.Ints) != 3 || out.Ints[0] != 1 || out.Ints[2] != 3 {
		t.Fatalf("unexpected ints: %#v", out.Ints)
	}
	if len(out.Int64) != 2 || out.Int64[0] != 4 || out.Int64[1] != 5 {
		t.Fatalf("unexpected int64: %#v", out.Int64)
	}
	if len(out.F64) != 2 || out.F64[0] != 1.5 || out.F64[1] != 2.5 {
		t.Fatalf("unexpected f64: %#v", out.F64)
	}
	if len(out.Flags) != 3 || !out.Flags[0] || out.Flags[1] || !out.Flags[2] {
		t.Fatalf("unexpected flags: %#v", out.Flags)
	}
	if intsPtr != unsafe.SliceData(out.Ints) || int64Ptr != unsafe.SliceData(out.Int64) || f64Ptr != unsafe.SliceData(out.F64) || flagsPtr != unsafe.SliceData(out.Flags) {
		t.Fatalf("expected slice backing arrays to be reused")
	}
}

func TestContainerReuse_TypedSlicePointers(t *testing.T) {
	type payload struct {
		Ints  *[]int
		Int64 *[]int64
		F64   *[]float64
		Flags *[]bool
	}
	ints := make([]int, 0, 8)
	int64s := make([]int64, 0, 8)
	f64 := make([]float64, 0, 8)
	flags := make([]bool, 0, 8)
	out := payload{
		Ints:  &ints,
		Int64: &int64s,
		F64:   &f64,
		Flags: &flags,
	}
	intsPtr := unsafe.SliceData(*out.Ints)
	int64Ptr := unsafe.SliceData(*out.Int64)
	f64Ptr := unsafe.SliceData(*out.F64)
	flagsPtr := unsafe.SliceData(*out.Flags)

	data := []byte(`{"Ints":[1,2],"Int64":[3,4],"F64":[5.5],"Flags":[true]}`)
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if len(*out.Ints) != 2 || (*out.Ints)[1] != 2 {
		t.Fatalf("unexpected ints: %#v", *out.Ints)
	}
	if len(*out.Int64) != 2 || (*out.Int64)[0] != 3 {
		t.Fatalf("unexpected int64: %#v", *out.Int64)
	}
	if len(*out.F64) != 1 || (*out.F64)[0] != 5.5 {
		t.Fatalf("unexpected f64: %#v", *out.F64)
	}
	if len(*out.Flags) != 1 || !(*out.Flags)[0] {
		t.Fatalf("unexpected flags: %#v", *out.Flags)
	}
	if intsPtr != unsafe.SliceData(*out.Ints) || int64Ptr != unsafe.SliceData(*out.Int64) || f64Ptr != unsafe.SliceData(*out.F64) || flagsPtr != unsafe.SliceData(*out.Flags) {
		t.Fatalf("expected pointed slice backing arrays to be reused")
	}
}
