package marshal

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"unsafe"
)

func TestAllEngineEntriesGuardCycles(t *testing.T) {
	type node struct{ Next *node }
	n := &node{}
	n.Next = n
	e := New(nil, nil, false, true, "", "", nil)
	entries := []func() ([]byte, error){func() ([]byte, error) { return e.Marshal(n) }, func() ([]byte, error) { return e.MarshalTo([]byte("prefix"), n) }, func() ([]byte, error) { return e.MarshalPtr(n) }, func() ([]byte, error) { return e.MarshalPtrTo([]byte("prefix"), n) }, func() ([]byte, error) {
		return e.MarshalTypedPtr([]byte("prefix"), reflect.TypeOf(*n), unsafe.Pointer(n))
	}}
	for i, run := range entries {
		out, err := run()
		var cycle *json.UnsupportedValueError
		if len(out) != 0 || !errors.As(err, &cycle) {
			t.Fatalf("entry %d: %s %v", i, out, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, run := range []func() ([]byte, error){func() ([]byte, error) { return e.MarshalContext(ctx, n) }, func() ([]byte, error) { return e.MarshalToContext(ctx, []byte("prefix"), n) }, func() ([]byte, error) {
		return e.MarshalTypedPtrContext(ctx, nil, reflect.TypeOf(*n), unsafe.Pointer(n))
	}} {
		out, err := run()
		if len(out) != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel: %s %v", out, err)
		}
	}
}

func TestSessionReleaseDropsTraversalAndContext(t *testing.T) {
	s := acquireSession()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s.ctx = ctx
	value := reflect.ValueOf(new(int))
	if err := s.enter(value); err != nil {
		t.Fatal(err)
	}
	releaseSession(s)
	if s.ctx != nil || s.traversal.depth != 0 || len(s.traversal.active) != 0 {
		t.Fatal("pooled session retained operation state")
	}
}

type standardContextCallback struct{ run func() ([]byte, error) }

func (v standardContextCallback) MarshalJSON() ([]byte, error) { return v.run() }
func TestStandardPreparedContextPreservesAuthority(t *testing.T) {
	sentinel := errors.New("standard callback failure")
	for _, exclude := range []func([]string, string) bool{nil, func([]string, string) bool { return false }} {
		e := NewStandardEncoder(exclude, nil)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if out, err := e.MarshalContext(ctx, struct{ Value int }{1}); len(out) != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("pre-cancel: %s %v", out, err)
		}
		for _, fail := range []bool{false, true} {
			ctx, cancel := context.WithCancel(context.Background())
			callback := standardContextCallback{func() ([]byte, error) {
				cancel()
				if fail {
					return nil, sentinel
				}
				return []byte(`1`), nil
			}}
			out, err := e.MarshalContext(ctx, struct{ Callback standardContextCallback }{callback})
			if len(out) != 0 {
				t.Fatalf("partial output: %s", out)
			}
			expected := error(context.Canceled)
			if fail {
				expected = sentinel
			}
			if !errors.Is(err, expected) {
				t.Fatalf("lost primary cause: %v", err)
			}
		}
		value := struct {
			Bytes  []byte
			Values []*int
		}{[]byte{1, 2, 3}, []*int{nil}}
		old, err := e.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		out, err := e.MarshalContext(context.Background(), value)
		if err != nil || string(out) != string(old) {
			t.Fatalf("standard wire parity: %s / %s %v", old, out, err)
		}
	}
}
