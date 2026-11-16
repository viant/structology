package json

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/viant/tagly/choice"
	"github.com/viant/tagly/format/text"
	"github.com/viant/x/shape"
)

type nilEnumChannels []string

type nilEnumRecord struct {
	Channels nilEnumChannels  `json:"channels" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" choice:"VIDEO"`
	Pointer  *nilEnumChannels `json:"pointer" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" choice:"VIDEO"`
	Omitted  nilEnumChannels  `json:"omitted,omitempty" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY"`
	Numbers  []int            `json:"numbers"`
}

func nilEnumRepresentation(t testing.TB) *shape.Representation {
	t.Helper()
	walker, err := shape.CompileRewrite(reflect.TypeFor[nilEnumRecord]())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Channels" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"renamed" mcpEnum:"true" enumAccept:"literal" choice:"DISPLAY" choice:"VIDEO"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestCollectionEnumNilSlicePolicy(t *testing.T) {
	var null nilEnumChannels
	empty, valid := nilEnumChannels{}, nilEnumChannels{"DISPLAY", "VIDEO"}
	for _, metadata := range []*shape.Representation{nil, nilEnumRepresentation(t)} {
		name := "channels"
		if metadata != nil {
			name = "renamed"
		}
		// Alternate policy engines sharing exactly the same type/representation
		// plan, then re-use each engine after the other policy has encoded.
		for _, policies := range [][]NilSlicePolicy{{NilSliceAsNull, NilSliceAsEmptyArray}, {NilSliceAsEmptyArray, NilSliceAsNull}} {
			encoders := make([]*Marshaller, 0, len(policies))
			for _, policy := range policies {
				encoder, err := NewMarshaller(reflect.TypeFor[nilEnumRecord](), WithNilSlicePolicy(policy), WithRepresentation(metadata))
				if err != nil {
					t.Fatal(err)
				}
				encoders = append(encoders, encoder)
			}
			for round := 0; round < 2; round++ {
				for i, policy := range policies {
					nilValue := "null"
					if policy == NilSliceAsEmptyArray {
						nilValue = "[]"
					}
					for _, tc := range []struct {
						value                      nilEnumRecord
						channels, pointer, numbers string
					}{
						{nilEnumRecord{}, nilValue, "null", nilValue},
						{nilEnumRecord{Pointer: &null}, nilValue, nilValue, nilValue},
						{nilEnumRecord{Channels: empty, Pointer: &empty, Omitted: empty, Numbers: []int{}}, "[]", "[]", "[]"},
						{nilEnumRecord{Channels: valid, Pointer: &valid, Numbers: []int{1, 2}}, `["DISPLAY","VIDEO"]`, `["DISPLAY","VIDEO"]`, `[1,2]`},
					} {
						before := tc.value
						want := fmt.Sprintf(`{"%s":%s,"pointer":%s,"numbers":%s}`, name, tc.channels, tc.pointer, tc.numbers)
						for _, encode := range []func(any) ([]byte, error){
							encoders[i].Marshal,
							func(value any) ([]byte, error) {
								return Marshal(value, WithRepresentation(metadata), WithNilSlicePolicy(policy))
							},
						} {
							encoded, err := encode(&tc.value)
							if err != nil || string(encoded) != want {
								t.Fatalf("representation=%v policy=%v round=%d: want=%s got=%s err=%v", metadata != nil, policy, round, want, encoded, err)
							}
						}
						if !reflect.DeepEqual(tc.value, before) {
							t.Fatal("source record changed")
						}
						if null != nil || !reflect.DeepEqual(valid, nilEnumChannels{"DISPLAY", "VIDEO"}) || empty == nil {
							t.Fatal("source collection changed")
						}
					}
				}
			}
		}
	}
}

type nilEnumIdentityTransform struct{}

func (nilEnumIdentityTransform) Transform(_, name string) string { return name }

func TestCollectionEnumNilSliceDynamicAndInline(t *testing.T) {
	type nested struct {
		Record nilEnumRecord `json:"record"`
	}
	type EmbeddedRecord nilEnumRecord
	type embedded struct {
		EmbeddedRecord
		Other string `json:"other"`
	}
	for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
		nilValue := "null"
		if policy == NilSliceAsEmptyArray {
			nilValue = "[]"
		}
		object := fmt.Sprintf(`"channels":%s,"pointer":null,"numbers":%s`, nilValue, nilValue)
		for _, tc := range []struct {
			value any
			want  string
		}{
			{&nested{}, `{"record":{` + object + `}}`},
			{&embedded{}, `{` + object + `,"other":""}`},
		} {
			encoded, err := Marshal(tc.value, WithNilSlicePolicy(policy), WithNameTransformer(nilEnumIdentityTransform{}))
			if err != nil || string(encoded) != tc.want {
				t.Fatalf("dynamic %T policy=%v want=%s got=%s err=%v", tc.value, policy, tc.want, encoded, err)
			}
		}
	}
}

type nilEnumCustomSlice []string

func (nilEnumCustomSlice) MarshalJSON() ([]byte, error) { return []byte(`["OWNER"]`), nil }

func TestCollectionEnumNilSliceCompatibility(t *testing.T) {
	type record struct {
		Channels []string           `json:"channels" mcpEnum:"true" enumAccept:"literal" choice:"<VIDEO>"`
		Code     int                `json:"code" choice:"10:<ACTIVE>" enumStyle:"literal"`
		Owner    nilEnumCustomSlice `json:"owner"`
	}
	for _, dynamic := range []bool{false, true} {
		for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
			opts := []Option{WithNilSlicePolicy(policy)}
			if dynamic {
				opts = append(opts, WithNameTransformer(nilEnumIdentityTransform{}))
			}
			value := &record{Channels: []string{"<VIDEO>"}, Code: 10}
			encoded, err := Marshal(value, opts...)
			want := `{"channels":["\u003cVIDEO\u003e"],"code":"\u003cACTIVE\u003e","owner":["OWNER"]}`
			if err != nil || string(encoded) != want {
				t.Fatalf("policy=%v want=%s got=%s err=%v", policy, want, encoded, err)
			}
			value.Channels = []string{"UNKNOWN"}
			if _, err = Marshal(value, opts...); err == nil || !strings.Contains(err.Error(), "unknown enum collection item") {
				t.Fatalf("unknown enum collection: %v", err)
			}
		}
	}
	type booleanEnum struct {
		Flag int `json:"flag" choice:"0:false" choice:"1:true" enumAccept:"boolean" enumStyle:"boolean"`
	}
	for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
		if _, err := Marshal(&booleanEnum{Flag: 2}, WithNilSlicePolicy(policy)); !errors.Is(err, choice.ErrUnknownOutput) {
			t.Fatalf("enum validation cause lost: %v", err)
		}
	}
}

func TestCollectionEnumNilSliceConcurrentPolicyEngines(t *testing.T) {
	metadata := nilEnumRepresentation(t)
	var group sync.WaitGroup
	for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
		encoder, err := NewMarshaller(reflect.TypeFor[nilEnumRecord](), WithRepresentation(metadata), WithNilSlicePolicy(policy))
		if err != nil {
			t.Fatal(err)
		}
		want := `{"renamed":null,"pointer":null,"numbers":null}`
		if policy == NilSliceAsEmptyArray {
			want = `{"renamed":[],"pointer":null,"numbers":[]}`
		}
		for worker := 0; worker < 4; worker++ {
			group.Add(1)
			go func() {
				defer group.Done()
				for i := 0; i < 50; i++ {
					encoded, err := encoder.Marshal(&nilEnumRecord{})
					if err != nil || string(encoded) != want {
						t.Errorf("concurrent policy=%v want=%s got=%s err=%v", policy, want, encoded, err)
						return
					}
				}
			}()
		}
	}
	group.Wait()
}

func TestCollectionEnumStandardNilSlicePolicyUnchanged(t *testing.T) {
	for _, metadata := range []*shape.Representation{nil, nilEnumRepresentation(t)} {
		name := "channels"
		if metadata != nil {
			name = "renamed"
		}
		for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
			encoded, err := MarshalStandard(&nilEnumRecord{}, WithRepresentation(metadata), WithNilSlicePolicy(policy))
			want := fmt.Sprintf(`{"%s":null,"pointer":null,"numbers":null}`, name)
			if err != nil || string(encoded) != want {
				t.Fatalf("standard representation=%v policy=%v want=%s got=%s err=%v", metadata != nil, policy, want, encoded, err)
			}
		}
	}
}

func BenchmarkCollectionEnumNilSliceWholeEncode(b *testing.B) {
	metadata := nilEnumRepresentation(b)
	for _, policy := range []NilSlicePolicy{NilSliceAsNull, NilSliceAsEmptyArray} {
		for _, prepared := range []bool{false, true} {
			b.Run(fmt.Sprintf("policy=%v/prepared=%v", policy, prepared), func(b *testing.B) {
				value := &nilEnumRecord{}
				options := []Option{WithRepresentation(metadata), WithNilSlicePolicy(policy)}
				encoder, err := NewMarshaller(reflect.TypeFor[nilEnumRecord](), options...)
				if err != nil {
					b.Fatal(err)
				}
				encode := func() ([]byte, error) { return Marshal(value, options...) }
				if prepared {
					encode = func() ([]byte, error) { return encoder.Marshal(value) }
				}
				if _, err := encode(); err != nil {
					b.Fatal(err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := encode(); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

func BenchmarkCollectionEnumNilSliceColdEncode(b *testing.B) {
	value := &nilEnumRecord{}
	// Compiled casing uses an engine-local plan cache, so each Marshal includes
	// engine construction, metadata compilation, validation, and encoding.
	opts := []Option{WithCaseFormat(text.CaseFormatLowerCamel), WithNilSlicePolicy(NilSliceAsEmptyArray)}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := Marshal(value, opts...); err != nil {
			b.Fatal(err)
		}
	}
}
