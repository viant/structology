package marshal

import (
	"testing"
)

func TestEngine_MarshalTo_AppendsToPrefix(t *testing.T) {
	type sample struct {
		ID int
	}
	e := New(nil, nil, false, true, "", "", nil)
	dst := []byte("prefix:")
	out, err := e.MarshalTo(dst, sample{ID: 7})
	if err != nil {
		t.Fatalf("marshal to failed: %v", err)
	}
	if string(out) != `prefix:{"ID":7}` {
		t.Fatalf("unexpected output: %s", string(out))
	}
}

func TestEngine_MarshalTo_GrowsWhenCapacityTooSmall(t *testing.T) {
	type sample struct {
		ID int
	}
	e := New(nil, nil, false, true, "", "", nil)
	base := make([]byte, 0, 1)
	ptr0 := &base[:cap(base)][0]
	out, err := e.MarshalTo(base, sample{ID: 7})
	if err != nil {
		t.Fatalf("marshal to failed: %v", err)
	}
	if string(out) != `{"ID":7}` {
		t.Fatalf("unexpected output: %s", string(out))
	}
	ptr1 := &out[:cap(out)][0]
	if ptr0 == ptr1 {
		t.Fatalf("expected reallocation when capacity is too small")
	}
}

func TestEngine_MarshalPtrTo_ValidatesPointer(t *testing.T) {
	e := New(nil, nil, false, true, "", "", nil)
	if _, err := e.MarshalPtrTo(nil, 1); err == nil {
		t.Fatalf("expected pointer validation error")
	}
}
