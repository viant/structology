package json_test

import (
	"context"
	stdjson "encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	native "github.com/viant/structology/encoding/json"
	"github.com/viant/tagly/choice"
	"github.com/viant/tagly/format/text"
)

type traversalNames struct{}

func (traversalNames) TransformPath(_ []string, field string) string { return field }

type traversalNode struct {
	Value    int
	Next     *traversalNode
	Children []*traversalNode
	Hidden   any `json:"-"`
}
type traversalCallback struct{ run func() ([]byte, error) }

func (v traversalCallback) MarshalJSON() ([]byte, error) { return v.run() }

type traversalFailure struct{}

func (*traversalFailure) Error() string { return "callback failure" }

func TestTransformedTraversalCyclesAndReuse(t *testing.T) {
	n := &traversalNode{Value: 1}
	n.Next = n
	sl := make([]any, 1)
	sl[0] = sl
	mp := map[string]any{}
	mp["self"] = mp
	var iface any
	iface = &iface
	shared := &traversalNode{Value: 2}
	for _, opts := range [][]native.Option{nil, {native.WithCaseFormat(text.CaseFormatLowerCamel)}, {native.WithPathNameTransformer(traversalNames{})}} {
		for _, v := range []any{n, sl, mp, &iface, struct{ M map[string]any }{mp}} {
			m, err := native.NewMarshaller(reflect.TypeOf(v), opts...)
			if err != nil {
				t.Fatal(err)
			}
			for _, encode := range []func(any) ([]byte, error){func(v any) ([]byte, error) { return native.Marshal(v, opts...) }, m.Marshal} {
				out, err := encode(v)
				var unsupported *stdjson.UnsupportedValueError
				if len(out) != 0 || !errors.As(err, &unsupported) {
					t.Fatalf("%T: output=%s error=%v", v, out, err)
				}
			}
		}
		good := &traversalNode{Children: []*traversalNode{shared, shared}, Hidden: n}
		m, _ := native.NewMarshaller(reflect.TypeOf(good), opts...)
		for i := 0; i < 3; i++ {
			if out, err := m.Marshal(n); len(out) != 0 || err == nil {
				t.Fatalf("cycle: %s %v", out, err)
			}
			out, err := m.Marshal(good)
			if err != nil || !stdjson.Valid(out) {
				t.Fatalf("valid: %s %v", out, err)
			}
		}
		for _, v := range []any{(*traversalNode)(nil), []*traversalNode(nil), []*traversalNode{}} {
			out, err := native.Marshal(v, opts...)
			if err != nil || !stdjson.Valid(out) {
				t.Fatalf("nil/empty: %s %v", out, err)
			}
		}
	}
}

func TestTransformedMarshalContext(t *testing.T) {
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	opts := []native.Option{native.WithCaseFormat(text.CaseFormatLowerCamel)}
	m, _ := native.NewMarshaller(reflect.TypeOf(&traversalNode{}), opts...)
	for _, ctx := range []context.Context{canceled, expired} {
		for _, v := range []any{nil, 7, &traversalNode{}, struct{ X int }{1}} {
			for _, encode := range []func(context.Context, any) ([]byte, error){m.MarshalContext, func(c context.Context, v any) ([]byte, error) { return native.MarshalContext(c, v, opts...) }, func(c context.Context, v any) ([]byte, error) { return native.Marshal(v, native.WithContext(c)) }} {
				out, err := encode(ctx, v)
				if len(out) != 0 || !errors.Is(err, ctx.Err()) {
					t.Fatalf("%T: %s %v", v, out, err)
				}
			}
		}
	}
	for _, fail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		sentinel := &traversalFailure{}
		callback := traversalCallback{func() ([]byte, error) {
			cancel()
			if fail {
				return nil, sentinel
			}
			return []byte(`1`), nil
		}}
		v := struct {
			First    int
			Callback traversalCallback
			Last     []int
		}{1, callback, []int{1, 2}}
		prepared, _ := native.NewMarshaller(reflect.TypeOf(v), opts...)
		out, err := prepared.MarshalContext(ctx, v)
		if len(out) != 0 {
			t.Fatalf("partial output: %s", out)
		}
		if fail {
			var typed *traversalFailure
			if !errors.Is(err, sentinel) || !errors.As(err, &typed) {
				t.Fatalf("lost callback error %v", err)
			}
		} else if !errors.Is(err, context.Canceled) {
			t.Fatalf("lost cancel %v", err)
		}
		out, err = prepared.MarshalContext(context.Background(), struct {
			First    int
			Callback traversalCallback
			Last     []int
		}{1, traversalCallback{func() ([]byte, error) { return []byte(`2`), nil }}, []int{3}})
		if err != nil || !stdjson.Valid(out) {
			t.Fatalf("reused: %s %v", out, err)
		}
	}
}

func TestPreparedConcurrentContexts(t *testing.T) {
	m, _ := native.NewMarshaller(reflect.TypeOf(&traversalNode{}), native.WithCaseFormat(text.CaseFormatLowerCamel))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				current := context.Background()
				if i%2 == 0 {
					current = ctx
				}
				out, err := m.MarshalContext(current, &traversalNode{Value: i})
				if i%2 == 0 {
					if len(out) != 0 || !errors.Is(err, context.Canceled) {
						t.Errorf("cancel: %s %v", out, err)
					}
				} else if err != nil || !stdjson.Valid(out) {
					t.Errorf("success: %s %v", out, err)
				}
			}
		}(i)
	}
	wg.Wait()
}

func TestTransformedDepthBoundary(t *testing.T) {
	type link struct{ Next *link }
	for _, opts := range [][]native.Option{nil, {native.WithCaseFormat(text.CaseFormatLowerCamel)}, {native.WithPathNameTransformer(traversalNames{})}} {
		var root *link
		for i := 0; i < 5000; i++ {
			root = &link{Next: root}
		}
		m, _ := native.NewMarshaller(reflect.TypeOf(root), opts...)
		for _, encode := range []func(any) ([]byte, error){m.Marshal, func(v any) ([]byte, error) { return native.Marshal(v, opts...) }} {
			if out, err := encode(root); err != nil || len(out) == 0 {
				t.Fatalf("depth 10000 should encode: %v", err)
			}
			if out, err := encode(&link{Next: root}); err == nil || len(out) != 0 || err.Error() != "JSON value exceeds maximum nesting depth" {
				t.Fatalf("depth overflow: %d %v", len(out), err)
			}
		}
	}
}

// Context cancellation triggered by traversal checkpoints makes this deterministic
// and verifies cancellation during primitive loops without timing-dependent sleeps.
type checkpointContext struct {
	context.Context
	remaining int
}

func (c *checkpointContext) Err() error {
	c.remaining--
	if c.remaining <= 0 {
		return context.Canceled
	}
	return nil
}
func TestCancellationInsidePrimitiveContainers(t *testing.T) {
	for _, value := range []any{struct{ Values []int }{make([]int, 100000)}, struct{ Values map[string]string }{map[string]string{"a": "b", "c": "d"}}, make([]int, 100000)} {
		m, _ := native.NewMarshaller(reflect.TypeOf(value))
		ctx := &checkpointContext{Context: context.Background(), remaining: 12}
		// Map has fewer checkpoints, so choose a smaller budget for that workload.
		if reflect.TypeOf(value).Kind() == reflect.Struct && reflect.TypeOf(value).Field(0).Type.Kind() == reflect.Map {
			ctx.remaining = 6
		}
		out, err := m.MarshalContext(ctx, value)
		if len(out) != 0 || !errors.Is(err, context.Canceled) {
			t.Fatalf("%T: %s %v", value, out, err)
		}
	}
}

func TestMarshalDeadlineAfterSynchronousCodec(t *testing.T) {
	ctx, stop := context.WithTimeout(context.Background(), time.Millisecond)
	defer stop()
	v := traversalCallback{func() ([]byte, error) { <-ctx.Done(); return []byte(`1`), nil }}
	m, _ := native.NewMarshaller(reflect.TypeOf(v))
	if out, err := m.MarshalContext(ctx, v); len(out) != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline: %s %v", out, err)
	}
}

func TestMarshalEnumErrorIdentity(t *testing.T) {
	type record struct {
		Prefix int
		Mode   string `choice:"0:Pending" choice:"10:Active" enumStyle:"literal" enumOutputUnknown:"reject"`
	}
	for _, opts := range [][]native.Option{nil, {native.WithCaseFormat(text.CaseFormatLowerCamel)}, {native.WithPathNameTransformer(traversalNames{})}} {
		m, _ := native.NewMarshaller(reflect.TypeOf(&record{}), opts...)
		for _, encode := range []func(any) ([]byte, error){m.Marshal, func(v any) ([]byte, error) { return native.Marshal(v, opts...) }} {
			out, err := encode(&record{Prefix: 1, Mode: "invalid"})
			if len(out) != 0 || !errors.Is(err, choice.ErrUnknownOutput) {
				t.Fatalf("enum error identity: %s %v", out, err)
			}
			if out, err = encode(&record{Mode: "Active"}); err != nil || !stdjson.Valid(out) {
				t.Fatalf("enum recovery: %s %v", out, err)
			}
		}
	}
}

type traversalOpaque struct{ N int }

func (*traversalOpaque) MarshalJSON() ([]byte, error) { return []byte(`"custom"`), nil }
func TestTraversalPreservesCustomAdmissionBoundary(t *testing.T) {
	p := &traversalOpaque{N: 7}
	for _, opts := range [][]native.Option{nil, {native.WithPathNameTransformer(traversalNames{})}} {
		cases := []struct {
			value any
			want  string
		}{{struct{ Value any }{p}, `{"Value":{"N":7}}`}, {&p, `{"N":7}`}, {p, `"custom"`}}
		for _, tc := range cases {
			m, _ := native.NewMarshaller(reflect.TypeOf(tc.value), opts...)
			for _, encode := range []func(any) ([]byte, error){m.Marshal, func(v any) ([]byte, error) { return native.Marshal(v, opts...) }} {
				out, err := encode(tc.value)
				if err != nil || string(out) != tc.want {
					t.Fatalf("%T admission changed: %s %v, want %s", tc.value, out, err, tc.want)
				}
			}
		}
	}
}
