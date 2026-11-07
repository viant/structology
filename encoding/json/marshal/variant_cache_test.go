package marshal

import (
	"github.com/viant/x/shape"
	"reflect"
	"sync"
	"testing"
)

type cacheVariantRecord struct {
	Secret string `json:"secret"`
	Mode   int    `json:"mode" choice:"0:draft" choice:"10:active"`
}

func cacheVariants(t *testing.T) (*shape.Representation, *shape.Representation) {
	t.Helper()
	walker, err := shape.CompileRewrite(reflect.TypeFor[cacheVariantRecord]())
	if err != nil {
		t.Fatal(err)
	}
	makeVariant := func(reader bool) *shape.Representation {
		metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			switch f.Name {
			case "Secret":
				tag := reflect.StructTag(`json:"public"`)
				if reader {
					tag = `json:"-"`
				}
				return &shape.FieldMetadata{Name: f.Name, Tag: tag}, nil
			case "Mode":
				if reader {
					return &shape.FieldMetadata{Name: f.Name, Tag: `json:"mode" choice:"0:draft" choice:"10:active" enumStyle:"literal"`}, nil
				}
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return metadata
	}
	return makeVariant(true), makeVariant(false)
}
func TestVariantEncoderCachesReuseWithoutCollisions(t *testing.T) {
	a, b := cacheVariants(t)
	typ := reflect.TypeFor[cacheVariantRecord]()
	newEngine := func() *Engine { return New(nil, nil, false, true, "", "", nil) }
	base := newEngine()
	pa := base.getStaticPlan(typ, a)
	pb := base.getStaticPlan(typ, b)
	plain := base.getStaticPlan(typ)
	if pa == pb || pa == plain || pb == plain {
		t.Fatal("variants collapsed to one plan")
	}
	for _, meta := range []*shape.Representation{a, b, a, nil, b, nil, a} {
		want := plain
		if meta == a {
			want = pa
		}
		if meta == b {
			want = pb
		}
		engine := newEngine().WithRepresentation(meta)
		if engine.getStaticPlan(typ, meta) != want || engine.getDynamicPlan(typ, meta) != want {
			t.Fatal("fresh encoder did not reuse variant plan")
		}
		value := cacheVariantRecord{Secret: "token", Mode: 10}
		expected := `{"secret":"token","mode":10}`
		if meta == a {
			expected = `{"mode":"active"}`
		}
		if meta == b {
			expected = `{"public":"token","mode":10}`
		}
		raw, err := engine.Marshal(value)
		if err != nil || string(raw) != expected {
			t.Fatalf("%s %v", raw, err)
		}
		standard := NewStandardEncoder(nil, nil).WithRepresentation(meta)
		raw, err = standard.Marshal(value)
		if err != nil || string(raw) != expected {
			t.Fatalf("standard %s %v", raw, err)
		}
	}
	sa, sb := standardFieldsFor(typ, a), standardFieldsFor(typ, b)
	if sa == sb || sa != standardFieldsFor(typ, a) || sb != standardFieldsFor(typ, b) {
		t.Fatal("standard variant cache identity")
	}
	if base.WithRepresentation(a).WithRepresentation(nil).representation != nil {
		t.Fatal("linked selection retained prior variant")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			meta, want := a, pa
			if i%2 != 0 {
				meta, want = b, pb
			}
			if newEngine().getStaticPlan(typ, meta) != want {
				t.Error("concurrent cache collision")
			}
		}(i)
	}
	wg.Wait()
}
func TestCompiledNamingConfigurationDoesNotSharePlans(t *testing.T) {
	a, _ := cacheVariants(t)
	typ := reflect.TypeFor[cacheVariantRecord]()
	first := New(nil, nil, false, true, "", "custom", func(name string) string { return "first_" + name })
	second := New(nil, nil, false, true, "", "custom", func(name string) string { return "second_" + name })
	plan := first.getStaticPlan(typ, a)
	if plan == second.getStaticPlan(typ, a) {
		t.Fatal("custom naming owners shared a plan")
	}
	if plan != first.WithRepresentation(a).getStaticPlan(typ, a) {
		t.Fatal("configured encoder clone lost cached variant")
	}
}
