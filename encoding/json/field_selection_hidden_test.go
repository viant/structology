package json_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
)

func TestTransformedSelectionKeepsHiddenFieldsExcluded(t *testing.T) {
	type row struct {
		ID       int    `json:"id"`
		Internal string `json:"internal" internal:"true"`
		Ignored  string `json:"ignored" format:"ignore=true"`
		Name     string `json:"name"`
	}
	type envelope struct {
		Rows []row `json:"data"`
	}
	value := envelope{Rows: []row{{ID: 7, Internal: "private", Ignored: "ignored", Name: "Example"}}}
	for _, fields := range [][]string{{"ID"}, {"ID", "Internal", "Ignored"}} {
		t.Run(fields[len(fields)-1], func(t *testing.T) {
			filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Rows"}, Fields: fields}})
			require.NoError(t, err)
			before, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
			require.NoError(t, err)
			transformed, err := filter.WithOptions()
			require.NoError(t, err)
			got, err := structjson.Marshal(value, structjson.WithPathFieldExcluder(transformed))
			require.NoError(t, err)
			require.JSONEq(t, `{"data":[{"id":7}]}`, string(got))
			after, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
			require.NoError(t, err)
			require.Equal(t, string(before), string(after), "transformed selection must not mutate the original")
		})
	}
}
