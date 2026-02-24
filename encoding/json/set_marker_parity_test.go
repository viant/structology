package json

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format/text"
)

func TestSetMarker_UnmarshalPointerHolderMatrix(t *testing.T) {
	type Has struct {
		ID    bool
		Name  bool
		Count bool
	}
	type Item struct {
		ID    int
		Name  string
		Count *int
		Has   *Has `setMarker:"true"`
	}

	t.Run("marks present fields", func(t *testing.T) {
		var out Item
		require.NoError(t, Unmarshal([]byte(`{"ID":1,"Name":"x"}`), &out))
		require.NotNil(t, out.Has)
		require.True(t, out.Has.ID)
		require.True(t, out.Has.Name)
		require.False(t, out.Has.Count)
	})

	t.Run("null still marks presence", func(t *testing.T) {
		var out Item
		require.NoError(t, Unmarshal([]byte(`{"Count":null}`), &out))
		require.NotNil(t, out.Has)
		require.True(t, out.Has.Count)
	})

	t.Run("empty object allocates holder with false flags", func(t *testing.T) {
		var out Item
		require.NoError(t, Unmarshal([]byte(`{}`), &out))
		require.NotNil(t, out.Has)
		require.False(t, out.Has.ID)
		require.False(t, out.Has.Name)
		require.False(t, out.Has.Count)
	})

	t.Run("input marker field is ignored", func(t *testing.T) {
		var out Item
		require.NoError(t, Unmarshal([]byte(`{"ID":1,"Has":{"ID":false,"Name":false}}`), &out))
		require.NotNil(t, out.Has)
		require.True(t, out.Has.ID)
		require.False(t, out.Has.Name)
	})
}

func TestSetMarker_UnmarshalValueHolder(t *testing.T) {
	type Has struct {
		ID   bool
		Name bool
	}
	type Item struct {
		ID   int
		Name string
		Has  Has `setMarker:"true"`
	}

	var out Item
	require.NoError(t, Unmarshal([]byte(`{"Name":"x"}`), &out))
	require.False(t, out.Has.ID)
	require.True(t, out.Has.Name)
}

func TestSetMarker_UnmarshalNestedSlice(t *testing.T) {
	type ChildHas struct {
		ID bool
	}
	type Child struct {
		ID  int
		Has *ChildHas `setMarker:"true"`
	}
	type ParentHas struct {
		Children bool
	}
	type Parent struct {
		Children []Child
		Has      *ParentHas `setMarker:"true"`
	}

	var out Parent
	require.NoError(t, Unmarshal([]byte(`{"Children":[{"ID":10}]}`), &out))
	require.NotNil(t, out.Has)
	require.True(t, out.Has.Children)
	require.Len(t, out.Children, 1)
	require.NotNil(t, out.Children[0].Has)
	require.True(t, out.Children[0].Has.ID)
}

func TestSetMarker_UnmarshalWithCaseFormat(t *testing.T) {
	type Has struct {
		UserID bool
	}
	type Item struct {
		UserID int
		Has    *Has `setMarker:"true"`
	}

	var out Item
	require.NoError(t, Unmarshal([]byte(`{"userID":7}`), &out, WithCaseFormat(text.CaseFormatLowerCamel)))
	require.NotNil(t, out.Has)
	require.True(t, out.Has.UserID)
}
