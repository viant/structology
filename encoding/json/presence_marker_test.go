package json

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnmarshal_AllocatesPresencePointerOnDemand(t *testing.T) {
	type Has struct {
		ID bool
	}
	type Item struct {
		ID  int
		Has *Has `setMarker:"true"`
	}

	var item Item
	require.Nil(t, item.Has)
	require.NoError(t, Unmarshal([]byte(`{"ID":123}`), &item))
	require.NotNil(t, item.Has)
	require.True(t, item.Has.ID)
}
