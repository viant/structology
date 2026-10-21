package json

import (
	"context"
	stdjson "encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type omissionInt int
type omissionBool bool
type omissionString string
type omissionAlias = int

type omissionIdentity struct{}

func (omissionIdentity) TransformPath(_ []string, name string) string { return name }
func (omissionIdentity) ExcludePath(_ []string, _ string) bool        { return false }

func TestNativePointerOmissionMatchesStandardJSON(t *testing.T) {
	for _, typ := range []reflect.Type{
		reflect.TypeFor[string](), reflect.TypeFor[bool](),
		reflect.TypeFor[int](), reflect.TypeFor[int8](), reflect.TypeFor[int16](), reflect.TypeFor[int32](), reflect.TypeFor[int64](),
		reflect.TypeFor[uint](), reflect.TypeFor[uint8](), reflect.TypeFor[uint16](), reflect.TypeFor[uint32](), reflect.TypeFor[uint64](), reflect.TypeFor[uintptr](),
		reflect.TypeFor[float32](), reflect.TypeFor[float64](), reflect.TypeFor[byte](), reflect.TypeFor[rune](),
		reflect.TypeFor[omissionInt](), reflect.TypeFor[omissionBool](), reflect.TypeFor[omissionString](), reflect.TypeFor[omissionAlias](),
	} {
		for _, path := range []string{"fast-only", "mixed-static", "dynamic", "slow-field"} {
			for _, policy := range []string{"tag", "global", "disabled"} {
				t.Run(typ.String()+"/"+path+"/"+policy, func(t *testing.T) {
					tag := `json:"value,omitempty"`
					if policy != "tag" {
						tag = `json:"value"`
					}
					if path == "slow-field" {
						tag += ` format:"nullable=true"`
					}
					fields := []reflect.StructField{{Name: "Value", Type: reflect.PointerTo(typ), Tag: reflect.StructTag(tag)}, {Name: "Scalar", Type: typ, Tag: `json:"scalar,omitempty"`}}
					if path == "mixed-static" {
						fields = append(fields, reflect.StructField{Name: "Slow", Type: reflect.TypeFor[map[string]int](), Tag: `json:"slow,omitempty"`})
					}
					rowType := reflect.StructOf(fields)
					var opts []Option
					if policy == "global" {
						opts = append(opts, WithOmitEmpty(true))
					}
					if path == "dynamic" {
						opts = append(opts, WithPathNameTransformer(omissionIdentity{}), WithPathFieldExcluder(omissionIdentity{}))
					}
					encoder, err := NewMarshaller(rowType, opts...)
					require.NoError(t, err)
					for _, value := range []string{"nil", "zero", "nonzero"} {
						row := reflect.New(rowType).Elem()
						if value != "nil" {
							pointer := reflect.New(typ)
							if value == "nonzero" {
								setOmissionNonzero(pointer.Elem())
							}
							row.Field(0).Set(pointer)
						}
						standard := row.Interface()
						if policy == "global" {
							// Standard JSON has no global omission option; compare its
							// equivalent field-tag contract without changing values.
							standardFields := append([]reflect.StructField(nil), fields...)
							standardFields[0].Tag = `json:"value,omitempty"`
							standardRow := reflect.New(reflect.StructOf(standardFields)).Elem()
							for i := 0; i < row.NumField(); i++ {
								standardRow.Field(i).Set(row.Field(i))
							}
							standard = standardRow.Interface()
						}
						want, err := stdjson.Marshal(standard)
						require.NoError(t, err)
						for _, fn := range []func() ([]byte, error){
							func() ([]byte, error) { return Marshal(row.Interface(), opts...) },
							func() ([]byte, error) { return MarshalContext(context.Background(), row.Addr().Interface(), opts...) },
							func() ([]byte, error) { return encoder.Marshal(row.Interface()) },
						} {
							got, err := fn()
							require.NoError(t, err)
							require.JSONEq(t, string(want), string(got), value)
						}
					}
				})
			}
		}
	}
}

func setOmissionNonzero(value reflect.Value) {
	switch value.Kind() {
	case reflect.String:
		value.SetString("present")
	case reflect.Bool:
		value.SetBool(true)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(7)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value.SetUint(7)
	case reflect.Float32, reflect.Float64:
		value.SetFloat(7)
	}
}

func TestPointerOmissionConcurrentColdAndWarmPlans(t *testing.T) {
	type row struct {
		Value *int    `json:"value,omitempty"`
		Flag  *bool   `json:"flag,omitempty"`
		Text  *string `json:"text,omitempty"`
	}
	zero, flag, text := 0, false, ""
	value := row{&zero, &flag, &text}
	for _, warm := range []bool{false, true} {
		encoder, err := NewMarshaller(reflect.TypeFor[row]())
		require.NoError(t, err)
		if warm {
			_, err = encoder.Marshal(value)
			require.NoError(t, err)
		}
		var wg sync.WaitGroup
		for range 16 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 20 {
					got, err := encoder.Marshal(value)
					if err != nil {
						t.Error(err)
						return
					}
					if string(got) != `{"value":0,"flag":false,"text":""}` {
						t.Errorf("%s", got)
						return
					}
				}
			}()
		}
		wg.Wait()
	}
}
