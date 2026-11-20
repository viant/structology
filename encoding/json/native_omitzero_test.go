package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/viant/x/shape"
)

type customZeroInt int

func (n *customZeroInt) IsZero() bool { return *n == 7 }

type valueZeroInt int

func (n valueZeroInt) IsZero() bool { return n == 8 }

func TestNativeOmitZeroPredicates(t *testing.T) {
	type scalars struct {
		Ptr      *int          `json:"ptr,omitzero"`
		Custom   customZeroInt `json:"custom,omitzero"`
		Value    valueZeroInt  `json:"value,omitzero"`
		Untagged customZeroInt `json:"untagged"`
	}
	type row struct {
		Scalars scalars                    `json:"scalars"`
		Method  interface{ IsZero() bool } `json:"method,omitzero"`
		Any     any                        `json:"any,omitzero"`
		First   []int                      `json:"first,omitempty,omitzero"`
		Last    []int                      `json:"last,omitzero,omitempty"`
		Slice   []int                      `json:"slice,omitzero"`
		Array   [2]int                     `json:"array,omitzero"`
	}
	zero := 0
	custom := customZeroInt(7)
	var nilCustom *customZeroInt
	for _, value := range []any{
		scalars{}, scalars{&zero, 7, 8, 7},
		row{},
		row{Scalars: scalars{&zero, 7, 8, 7}, Method: nilCustom, Any: nilCustom, First: []int{}, Last: []int{}, Slice: []int{}},
		row{Method: &custom, Any: &custom, Array: [2]int{1}, First: []int{1}, Last: []int{1}},
		[]row{{Method: valueZeroInt(8)}, {Method: valueZeroInt(1)}},
	} {
		want, err := stdjson.Marshal(value)
		require.NoError(t, err)
		for _, opts := range [][]Option{nil, {WithPathFieldExcluder(omissionIdentity{})}, {WithPathNameTransformer(omissionIdentity{})}} {
			got, err := Marshal(value, opts...)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(got))
		}
	}
}

func TestNativeOmitZeroTime(t *testing.T) {
	type row struct {
		First   *time.Time `json:"first,omitempty,omitzero" format:"timeLayout=2006-01-02T15:04:05Z07:00,timezone=UTC,nullable=true"`
		Last    *time.Time `json:"last,omitzero,omitempty" format:"timeLayout=2006-01-02T15:04:05.000Z07:00,nullable=true"`
		Control *time.Time `json:"control,omitempty" format:"timeLayout=2006-01-02T15:04:05Z07:00,timezone=UTC,nullable=true"`
	}
	zero := time.Time{}
	stamp := time.Date(2026, 11, 7, 8, 0, 0, 987654321, time.FixedZone("local", -8*60*60))
	utc := stamp.UTC()
	for _, tc := range []struct {
		name  string
		value *time.Time
		want  string
	}{
		{"nil", nil, `{}`},
		{"zero", &zero, `{"control":null}`},
		{"utc", &utc, `{"first":"2026-11-07T16:00:00Z","last":"2026-11-07T16:00:00.987Z","control":"2026-11-07T16:00:00Z"}`},
		{"offset", &stamp, `{"first":"2026-11-07T16:00:00Z","last":"2026-11-07T08:00:00.987-08:00","control":"2026-11-07T16:00:00Z"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := row{tc.value, tc.value, tc.value}
			for _, opts := range [][]Option{nil, {WithPathNameTransformer(omissionIdentity{})}, {WithPathFieldExcluder(omissionIdentity{})}} {
				raw, err := Marshal(&record, opts...)
				require.NoError(t, err)
				assert.Equal(t, tc.want, string(raw))
				raw, err = Marshal([]row{{}, record}, opts...)
				require.NoError(t, err)
				assert.Equal(t, "[{},"+tc.want+"]", string(raw))
			}
		})
	}
}

func TestNativeOmitZeroRepresentationAndWire(t *testing.T) {
	type row struct {
		Extras  map[string]any `format:"inline=true"`
		Value   customZeroInt  `json:"value"`
		Control customZeroInt  `json:"control"`
	}
	type batch struct {
		Rows []row `json:"rows"`
	}
	typ := reflect.TypeFor[batch]()
	walker, err := shape.CompileRewrite(typ)
	require.NoError(t, err)
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, field *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if field.Name == "Value" {
			return &shape.FieldMetadata{Name: field.Name, Tag: `json:"value,omitzero"`}, nil
		}
		return nil, nil
	})
	require.NoError(t, err)
	value := batch{[]row{{map[string]any{"value": "fallback"}, 7, 7}, {nil, 0, 0}}}
	for _, opts := range [][]Option{{WithRepresentation(metadata)}, {WithRepresentation(metadata), WithPathFieldExcluder(omissionIdentity{})}} {
		encoder, err := NewMarshaller(typ, opts...)
		require.NoError(t, err)
		got, err := encoder.Marshal(&value)
		require.NoError(t, err)
		assert.JSONEq(t, `{"rows":[{"value":"fallback","control":7},{"value":0,"control":0}]}`, string(got))
		wire, err := encoder.Wire()
		require.NoError(t, err)
		fields := wire.Properties()[0].Shape().Element().Properties()
		require.Len(t, fields, 2)
		require.Equal(t, "value", fields[0].Name())
		assert.False(t, fields[0].Required())
		require.Equal(t, reflect.Interface, fields[0].Shape().Kind())
		require.True(t, fields[1].Required())
		require.Equal(t, reflect.Int, fields[1].Shape().Kind())
	}
}

type nonZeroStruct struct{}

func (nonZeroStruct) IsZero() bool { return false }

func TestNativeOmitZeroStructMethod(t *testing.T) {
	type row struct {
		Value nonZeroStruct `json:"value,omitempty,omitzero"`
	}
	for _, opts := range [][]Option{nil, {WithPathFieldExcluder(omissionIdentity{})}} {
		for _, value := range []any{row{}, []row{{}}} {
			want, err := stdjson.Marshal(value)
			require.NoError(t, err)
			got, err := Marshal(value, opts...)
			require.NoError(t, err)
			assert.JSONEq(t, string(want), string(got))
		}
	}
}
