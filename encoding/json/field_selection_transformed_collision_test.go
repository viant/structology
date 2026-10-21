package json_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
)

func TestSelectedHolderRejectsTransformedInlineSiblingCollision(t *testing.T) {
	type row struct {
		ID int `json:"id"`
	}
	type extra struct {
		Rows []row `json:"data"`
	}
	type output struct {
		Extra extra `json:"extra" jsonx:"inline"`
		Data  []row `json:"data"`
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	_, err = filter.WithOptions()
	require.ErrorContains(t, err, "not uniquely visible at the same transformed wire path")
}

func TestSelectedHolderRejectsWholeValueInlineAncestor(t *testing.T) {
	type row struct {
		ID int `json:"id"`
	}
	type output struct {
		Extra struct {
			Other int
		} `jsonx:"inline"`
		Data []row
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	_, err = filter.WithOptions()
	require.ErrorContains(t, err, "whole-value inline")
}

func TestSelectedNestedHolderRejectsNestedWholeValueInlineAncestor(t *testing.T) {
	type row struct {
		ID int `json:"id"`
	}
	type payload struct {
		Extra struct {
			Other int
		} `jsonx:"inline"`
		Data []row
	}
	type output struct {
		Payload payload `json:"payload"`
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	_, err = filter.WithOptions()
	require.ErrorContains(t, err, "whole-value inline")
}
