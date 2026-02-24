package jsontab

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshal_UnknownHeaderPolicy(t *testing.T) {
	type rec struct {
		A int `csvName:"a"`
	}
	data := []byte(`[["x"],[1]]`)

	var compat []rec
	require.NoError(t, Unmarshal(data, &compat))
	require.Len(t, compat, 1)
	require.Equal(t, 0, compat[0].A)

	var strict []rec
	err := Unmarshal(data, &strict, WithMode(ModeStrict))
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown header")
}

func TestUnmarshal_ArityPolicy(t *testing.T) {
	type rec struct {
		A int `csvName:"a"`
		B int `csvName:"b"`
	}
	data := []byte(`[["a","b"],[1]]`)

	var compat []rec
	require.NoError(t, Unmarshal(data, &compat))
	require.Len(t, compat, 1)
	require.Equal(t, 1, compat[0].A)
	require.Equal(t, 0, compat[0].B)

	var strict []rec
	err := Unmarshal(data, &strict, WithMode(ModeStrict))
	require.Error(t, err)
	require.Contains(t, err.Error(), "arity mismatch")
}

func TestUnmarshal_MalformedNestedStrictPath(t *testing.T) {
	type child struct {
		ID int `csvName:"id"`
	}
	type rec struct {
		A        int     `csvName:"a"`
		Children []child `csvName:"children"`
	}
	data := []byte(`[["a","children"],[1,{"x":1}]]`)

	var compat []rec
	require.NoError(t, Unmarshal(data, &compat))
	require.Len(t, compat, 1)
	require.Equal(t, 1, compat[0].A)
	require.Nil(t, compat[0].Children)

	var strict []rec
	err := Unmarshal(data, &strict, WithMode(ModeStrict))
	require.Error(t, err)
	require.Contains(t, err.Error(), "path=Children")
	require.Contains(t, err.Error(), "row=1")
	require.Contains(t, err.Error(), "col=1")
}

func TestUnmarshal_SetMarker(t *testing.T) {
	type hasRec struct {
		A bool
		B bool
	}
	type rec struct {
		A   int     `csvName:"a"`
		B   string  `csvName:"b"`
		Has *hasRec `setMarker:"true"`
	}

	var out []rec
	err := Unmarshal([]byte(`[["a"],[1]]`), &out)
	require.NoError(t, err)
	require.Len(t, out, 1)
	require.NotNil(t, out[0].Has)
	require.True(t, out[0].Has.A)
	require.False(t, out[0].Has.B)
}
