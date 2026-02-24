package jsontab

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format/text"
)

func TestMarshalUnmarshal_Flat(t *testing.T) {
	type item struct {
		ID   int    `csvName:"id"`
		Name string `csvName:"name"`
	}
	in := []item{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}}

	data, err := Marshal(in)
	require.NoError(t, err)
	require.JSONEq(t, `[["id","name"],[1,"a"],[2,"b"]]`, string(data))

	var out []item
	err = Unmarshal(data, &out)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestMarshalUnmarshal_NestedSlice(t *testing.T) {
	type child struct {
		ID int `csvName:"id"`
	}
	type parent struct {
		ID       int     `csvName:"id"`
		Children []child `csvName:"children"`
	}
	in := []parent{{ID: 1, Children: []child{{ID: 10}, {ID: 11}}}}

	data, err := Marshal(in)
	require.NoError(t, err)
	require.JSONEq(t, `[["id","children"],[1,[["id"],[10],[11]]]]`, string(data))

	var out []parent
	err = Unmarshal(data, &out)
	require.NoError(t, err)
	require.Equal(t, in, out)
}

func TestMarshal_TimeLayout(t *testing.T) {
	type item struct {
		At time.Time `csvName:"at"`
	}
	in := item{At: time.Date(2026, 2, 24, 10, 11, 12, 0, time.UTC)}

	data, err := Marshal(in, WithTimeLayout("2006-01-02"))
	require.NoError(t, err)
	require.JSONEq(t, `[["at"],["2026-02-24"]]`, string(data))

	var out item
	err = Unmarshal(data, &out, WithTimeLayout("2006-01-02"))
	require.NoError(t, err)
	require.Equal(t, in.At.UTC().Truncate(24*time.Hour), out.At.UTC())
}

func TestMarshal_CaseFormatDefaultNames(t *testing.T) {
	type item struct {
		UserID int
	}
	in := item{UserID: 7}

	data, err := Marshal(in, WithCaseFormat(text.CaseFormatLowerUnderscore))
	require.NoError(t, err)
	require.JSONEq(t, `[["user_id"],[7]]`, string(data))

	var out item
	err = Unmarshal(data, &out, WithCaseFormat(text.CaseFormatLowerUnderscore))
	require.NoError(t, err)
	require.Equal(t, in, out)
}
