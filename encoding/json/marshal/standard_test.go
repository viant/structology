package marshal

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type StdBase struct {
	ID   int `json:",string"`
	Name string
}
type stdZero int

func (v stdZero) IsZero() bool { return v == 7 }

type stdNamedPointer *int
type stdBytes []byte

type standardExample struct {
	*StdBase
	Quoted     string          `json:"quoted,string"`
	Flag       bool            `json:"flag,string"`
	Number     float64         `json:"number,string"`
	Pointer    *int            `json:"pointer,string"`
	Deep       **int           `json:"deep,string"`
	Named      stdNamedPointer `json:"named,string"`
	Bytes      stdBytes        `json:"bytes"`
	Alias      string          `json:"alias" format:"name=ignored"`
	CustomZero stdZero         `json:"zero,omitzero"`
	Internal   string          `internal:"true"`
}

func TestStandardWireUsesStandardPolicy(t *testing.T) {
	compiler := NewStandard(reflect.TypeFor[standardExample]())
	shape, err := compiler.Wire()
	require.NoError(t, err)
	properties := map[string]WireProperty{}
	for _, field := range shape.Properties() {
		properties[field.Name()] = field
	}
	require.False(t, properties["ID"].Required())
	require.Equal(t, reflect.String, properties["ID"].Shape().Kind())
	for _, name := range []string{"quoted", "flag", "number", "pointer"} {
		require.Equal(t, reflect.String, properties[name].Shape().Kind(), name)
	}
	require.True(t, properties["pointer"].Shape().Nullable())
	require.Equal(t, reflect.Pointer, properties["deep"].Shape().Kind())
	require.Equal(t, reflect.Pointer, properties["named"].Shape().Kind())
	require.Equal(t, "byte", properties["bytes"].Shape().Format())
	require.Contains(t, properties, "Internal")
	require.NotContains(t, properties, "ignored")
	require.False(t, properties["zero"].Required())
	n := 0
	p := &n
	value := standardExample{StdBase: &StdBase{0, "B"}, Quoted: "a\"b", Pointer: p, Deep: &p, Named: stdNamedPointer(p), Bytes: stdBytes{1, 2}, CustomZero: 7, Internal: "kept"}
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	require.JSONEq(t, `{"ID":"0","Name":"B","quoted":"\"a\\\"b\"","flag":"false","number":"0","pointer":"0","deep":0,"named":0,"bytes":"AQI=","alias":"","Internal":"kept"}`, string(raw))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			again, err := compiler.Wire()
			require.NoError(t, err)
			require.Same(t, shape, again)
		}()
	}
	wg.Wait()
}

type stdCustom int

func (stdCustom) MarshalJSON() ([]byte, error) { return []byte(`"opaque"`), nil }
func TestStandardWireKeepsCustomJSONUnconstrainedEvenWithString(t *testing.T) {
	type sample struct {
		Value stdCustom `json:",string"`
	}
	shape, err := NewStandard(reflect.TypeFor[sample]()).Wire()
	require.NoError(t, err)
	require.Equal(t, reflect.Interface, shape.Properties()[0].Shape().Kind())
	raw, err := json.Marshal(sample{})
	require.NoError(t, err)
	require.JSONEq(t, `{"Value":"opaque"}`, string(raw))
}
