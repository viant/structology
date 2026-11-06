package unmarshal

import (
	"github.com/viant/x/shape"
	"reflect"
	"testing"
)

type cacheVariantRecord struct {
	Secret string `json:"secret"`
	Mode   int    `json:"mode" choice:"0:draft" choice:"10:active"`
}

func TestVariantDecoderPlansAreReusedSeparately(t *testing.T) {
	typ := reflect.TypeFor[cacheVariantRecord]()
	walker, _ := shape.CompileRewrite(typ)
	build := func(reader bool) *shape.Representation {
		metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			if f.Name == "Secret" {
				tag := reflect.StructTag(`json:"public"`)
				if reader {
					tag = `json:"-"`
				}
				return &shape.FieldMetadata{Name: f.Name, Tag: tag}, nil
			}
			if f.Name == "Mode" && reader {
				return &shape.FieldMetadata{Name: f.Name, Tag: `json:"mode" choice:"0:draft" choice:"10:active" enumAccept:"literal"`}, nil
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		return metadata
	}
	a, b := build(true), build(false)
	pa, pb, plain := planFor(typ, "", nil, a), planFor(typ, "", nil, b), planFor(typ, "", nil, nil)
	sa, sb, standardPlain := standardDecodePlanFor(typ, a), standardDecodePlanFor(typ, b), standardDecodePlanFor(typ, nil)
	if pa == pb || pa == plain || sa == sb || sa == standardPlain {
		t.Fatal("decoder variants collapsed")
	}
	for _, meta := range []*shape.Representation{a, b, a, nil, b, a} {
		expected, expectedStandard := plain, standardPlain
		wire := `{"secret":"linked","mode":10}`
		wantSecret := "linked"
		if meta == a {
			expected, expectedStandard = pa, sa
			wire = `{"secret":"blocked","mode":"active"}`
			wantSecret = ""
		}
		if meta == b {
			expected, expectedStandard = pb, sb
			wire = `{"public":"visible","mode":10}`
			wantSecret = "visible"
		}
		if planFor(typ, "", nil, meta) != expected || standardDecodePlanFor(typ, meta) != expectedStandard {
			t.Fatal("cached variant not reused")
		}
		var result cacheVariantRecord
		err := NewStandardDecoder(meta).Unmarshal([]byte(wire), &result)
		if err != nil || result.Secret != wantSecret || result.Mode != 10 {
			t.Fatalf("%+v %v", result, err)
		}
		// Public codec integration tests exercise the native scanner. Here inspect
		// its cached plans directly and verify selecting the linked variant resets it.
		engine := New(t.Context(), nil, IgnoreUnknown, CoerceNumbers, CompatNulls, LastWins, Tolerant, "", "", nil, nil).WithRepresentation(meta)
		if engine.WithRepresentation(nil).representation != nil {
			t.Fatal("nil retained variant")
		}

	}
}

func TestStandardCachedEnumErrorStillRequiresPresentField(t *testing.T) {
	type invalid struct {
		Mode int `json:"mode" choice:"0:fixed" enumAccept:"invalid"`
	}
	var value invalid
	decoder := NewStandardDecoder(nil)
	if err := decoder.Unmarshal([]byte(`{}`), &value); err != nil {
		t.Fatalf("absent field changed behavior: %v", err)
	}
	if err := decoder.Unmarshal([]byte(`{"mode":"fixed"}`), &value); err == nil {
		t.Fatal("present invalid enum accepted")
	}
}
