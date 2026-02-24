package unmarshal

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHasCustomUnmarshalType_Time(t *testing.T) {
	require.False(t, hasCustomUnmarshalType(reflect.TypeOf(time.Time{})))
	require.False(t, hasCustomUnmarshalType(reflect.TypeOf((*time.Time)(nil))))
}
