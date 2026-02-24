package json

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format/text"
)

func TestPresenceMarker_FieldNameMapping_AliasCaseInsensitive(t *testing.T) {
	type Has struct {
		UserID bool
	}
	type Item struct {
		UserID int  `json:"user_id"`
		Has    *Has `setMarker:"true"`
	}

	cases := []string{
		`{"user_id":7}`,
		`{"USER_ID":7}`,
	}
	for _, data := range cases {
		var out Item
		require.NoError(t, Unmarshal([]byte(data), &out))
		require.NotNil(t, out.Has)
		require.True(t, out.Has.UserID, data)
	}
}

func TestPresenceMarker_FieldNameMapping_AliasWithCaseFormat(t *testing.T) {
	type Has struct {
		UserID bool
	}
	type Item struct {
		UserID int  `json:"user_id"`
		Has    *Has `setMarker:"true"`
	}

	var out Item
	require.NoError(t, Unmarshal([]byte(`{"USER_ID":7}`), &out, WithCaseFormat(text.CaseFormatLowerCamel)))
	require.NotNil(t, out.Has)
	require.True(t, out.Has.UserID)
}

func TestPresenceMarker_InlineEmbeddedEdgeCases(t *testing.T) {
	type OuterHas struct {
		Code bool
	}
	type Embedded struct {
		Code int
	}
	type Outer struct {
		Embedded `jsonx:"inline"`
		Has      *OuterHas `setMarker:"true"`
	}

	var out Outer
	require.NoError(t, Unmarshal([]byte(`{"Code":10}`), &out))
	require.Equal(t, 10, out.Code)
	require.NotNil(t, out.Has)
	require.True(t, out.Has.Code)
}

func TestPresenceMarker_InlineEmbeddedPointerEdgeCases(t *testing.T) {
	type OuterHas struct {
		Code bool
	}
	type Embedded struct {
		Code int
	}
	type Outer struct {
		*Embedded `jsonx:"inline"`
		Has       *OuterHas `setMarker:"true"`
	}

	var out Outer
	require.NoError(t, Unmarshal([]byte(`{"Code":10}`), &out))
	require.NotNil(t, out.Embedded)
	require.Equal(t, 10, out.Code)
	require.NotNil(t, out.Has)
	require.True(t, out.Has.Code)
}
