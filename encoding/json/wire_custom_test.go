package json

import (
	"context"
	stdjson "encoding/json"
	"github.com/stretchr/testify/require"
	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"github.com/viant/tagly/format/text"
	"reflect"
	"testing"
)

type wireTextID [16]byte

func (wireTextID) MarshalText() ([]byte, error) { return []byte("text-id"), nil }

type wireDynamicJSON struct{ Number bool }

func (v wireDynamicJSON) MarshalJSON() ([]byte, error) {
	if v.Number {
		return []byte("7"), nil
	}
	return []byte(`{"value":true}`), nil
}

type wireBothCodecs int

func (wireBothCodecs) MarshalJSON() ([]byte, error) { return []byte("7"), nil }
func (wireBothCodecs) MarshalText() ([]byte, error) { panic("JSON must take precedence") }

type wireNoExecution int

func (wireNoExecution) MarshalJSON() ([]byte, error) { panic("schema must never invoke user code") }

func TestCustomWireInterfaceGuarantees(t *testing.T) {
	type row struct {
		Text    wireTextID
		Pointer *wireTextID
		JSON    wireDynamicJSON
		Raw     stdjson.RawMessage
		Both    wireBothCodecs `json:",string"`
		Values  []wireDynamicJSON
	}
	for _, native := range []bool{false, true} {
		name := "standard"
		if native {
			name = "native"
		}
		t.Run(name, func(t *testing.T) {
			var shape *jsonmarshal.WireShape
			var err error
			if native {
				m, e := NewMarshaller(reflect.TypeFor[row](), WithCaseFormat(text.CaseFormatLowerCamel))
				require.NoError(t, e)
				shape, err = m.Wire()
			} else {
				shape, err = jsonmarshal.NewStandard(reflect.TypeFor[row]()).Wire()
			}
			require.NoError(t, err)
			fields := shape.Properties()
			require.Len(t, fields, 6)
			require.Equal(t, reflect.String, fields[0].Shape().Kind())
			require.Equal(t, reflect.TypeFor[wireTextID](), fields[0].Shape().Source())
			require.Equal(t, reflect.Pointer, fields[1].Shape().Kind())
			require.True(t, fields[1].Shape().Nullable())
			require.Equal(t, reflect.String, fields[1].Shape().Element().Kind())
			for _, i := range []int{2, 3, 4} {
				require.Equal(t, reflect.Interface, fields[i].Shape().Kind())
				require.True(t, fields[i].Shape().Nullable())
			}
			require.Equal(t, reflect.Interface, fields[5].Shape().Element().Kind())
			for _, number := range []bool{false, true} {
				value := row{JSON: wireDynamicJSON{number}, Raw: stdjson.RawMessage(`[1,true]`), Values: []wireDynamicJSON{{number}}}
				var data []byte
				if native {
					data, err = MarshalContext(context.Background(), value, WithCaseFormat(text.CaseFormatLowerCamel))
				} else {
					data, err = stdjson.Marshal(value)
				}
				require.NoError(t, err)
				if native {
					compiled, e := NewMarshaller(reflect.TypeFor[row](), WithCaseFormat(text.CaseFormatLowerCamel))
					require.NoError(t, e)
					actual, e := compiled.Marshal(&value)
					require.NoError(t, e)
					require.JSONEq(t, string(data), string(actual))
				}
				require.True(t, stdjson.Valid(data))
				var object map[string]any
				require.NoError(t, stdjson.Unmarshal(data, &object))
				key := "Text"
				both := "Both"
				if native {
					key = "text"
					both = "both"
				}
				require.Equal(t, "text-id", object[key])
				require.Equal(t, float64(7), object[both])
				jsonKey, rawKey := "JSON", "Raw"
				if native {
					jsonKey, rawKey = "json", "raw"
				}
				if number {
					require.Equal(t, float64(7), object[jsonKey])
				} else {
					require.Equal(t, map[string]any{"value": true}, object[jsonKey])
				}
				require.Equal(t, []any{float64(1), true}, object[rawKey])
			}
		})
	}
}

func TestWireNeverInvokesCustomMarshalers(t *testing.T) {
	type row struct{ Value wireNoExecution }
	_, err := jsonmarshal.NewStandard(reflect.TypeFor[row]()).Wire()
	require.NoError(t, err)
	m, err := NewMarshaller(reflect.TypeFor[row](), WithCaseFormat(text.CaseFormatLowerCamel))
	require.NoError(t, err)
	_, err = m.Wire()
	require.NoError(t, err)
}
