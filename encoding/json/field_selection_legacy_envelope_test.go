package json_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
)

type LegacySelectionStatus struct {
	Status  string         `json:"status,omitempty"`
	Message string         `json:"message,omitempty"`
	Errors  any            `json:"errors,omitempty"`
	Warning []string       `json:"warning,omitempty"`
	Extras  map[string]any `json:"extras,omitempty" format:"inline=true"`
}
type legacySelectionRow struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Secret string `json:"secret"`
}
type legacySelectionOutput struct {
	LegacySelectionStatus
	Data []legacySelectionRow `json:"data"`
}

func TestLegacyStatusInlineExtrasAllowsSelectingData(t *testing.T) {
	cases := []struct {
		name   string
		extras map[string]any
	}{
		{name: "nil"},
		{name: "empty", extras: map[string]any{}},
		{name: "nonempty", extras: map[string]any{"status": "from-extra", "data": []any{"shadow"}, "other": false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := legacySelectionOutput{
				LegacySelectionStatus: LegacySelectionStatus{Status: "ok", Extras: tc.extras},
				Data:                  []legacySelectionRow{{ID: 7, Name: "Ada", Secret: "hidden"}},
			}
			filter, err := structjson.NewFieldFilter(reflect.TypeFor[legacySelectionOutput](), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID", "Name"}}})
			require.NoError(t, err)
			filter, err = filter.WithOptions()
			require.NoError(t, err)
			for _, marshal := range []struct {
				name string
				fn   func(any, ...structjson.Option) ([]byte, error)
			}{
				{name: "standard", fn: structjson.MarshalStandard},
				{name: "transformed", fn: structjson.Marshal},
			} {
				raw, err := marshal.fn(value, structjson.WithPathFieldExcluder(filter))
				require.NoError(t, err)
				baseline, err := marshal.fn(value)
				require.NoError(t, err)
				var got map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(raw, &got))
				var want map[string]json.RawMessage
				require.NoError(t, json.Unmarshal(baseline, &want))
				if tc.extras == nil || len(tc.extras) == 0 {
					require.JSONEq(t, `{"status":"ok","data":[{"id":7,"name":"Ada"}]}`, string(raw), marshal.name)
				} else {
					require.Equal(t, string(want["status"]), string(got["status"]))
					require.JSONEq(t, `[{"id":7,"name":"Ada"}]`, string(got["data"]))
					require.Equal(t, string(want["other"]), string(got["other"]))
				}
				require.NotContains(t, string(raw), "secret")
			}
		})
	}
}

func TestLegacyStatusSelectedHolderRejectsAmbiguousPromotion(t *testing.T) {
	type Left struct{ Data []legacySelectionRow }
	type Right struct{ Data []legacySelectionRow }
	type ambiguous struct {
		Left
		Right
	}
	_, err := structjson.NewFieldFilter(reflect.TypeFor[ambiguous](), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	require.Error(t, err)
}

func TestSelectedHolderUsesCanonicalIdentityWithRenamedWirePath(t *testing.T) {
	type output struct {
		Meta LegacySelectionStatus `json:"meta"`
		Rows []legacySelectionRow  `json:"records"`
	}
	value := output{
		Meta: LegacySelectionStatus{Status: "ok", Extras: map[string]any{"unrelated": true}},
		Rows: []legacySelectionRow{{ID: 3, Name: "Lin", Secret: "hidden"}},
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Rows"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	filter, err = filter.WithOptions()
	require.NoError(t, err)
	raw, err := structjson.Marshal(value, structjson.WithPathFieldExcluder(filter))
	require.NoError(t, err)
	require.JSONEq(t, `{"meta":{"status":"ok","unrelated":true},"records":[{"id":3}]}`, string(raw))
}

func TestSelectedHolderRejectsTransformedAwayPath(t *testing.T) {
	type details struct{ Data []legacySelectionRow }
	type output struct {
		Details details `jsonx:"inline"`
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Details"}, Fields: []string{"Data"}}})
	require.NoError(t, err)
	_, err = filter.WithOptions()
	require.Error(t, err)
}

func TestLegacyInlineStatusSelectionSupportsNestedHolders(t *testing.T) {
	type payload struct {
		Data []legacySelectionRow `json:"data"`
	}
	type output struct {
		LegacySelectionStatus
		Payload payload `json:"payload"`
	}
	value := output{
		LegacySelectionStatus: LegacySelectionStatus{Status: "ok", Extras: map[string]any{"note": "kept"}},
		Payload:               payload{Data: []legacySelectionRow{{ID: 9, Name: "Sam", Secret: "hidden"}}},
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[output](), []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: []string{"Name"}}})
	require.NoError(t, err)
	filter, err = filter.WithOptions()
	require.NoError(t, err)
	raw, err := structjson.Marshal(value, structjson.WithPathFieldExcluder(filter))
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok","note":"kept","payload":{"data":[{"name":"Sam"}]}}`, string(raw))
}

func TestLegacyStatusFieldFilterConcurrentReuse(t *testing.T) {
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[legacySelectionOutput](), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	filter, err = filter.WithOptions()
	require.NoError(t, err)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			value := legacySelectionOutput{
				LegacySelectionStatus: LegacySelectionStatus{Status: "ok", Extras: map[string]any{"worker": id}},
				Data:                  []legacySelectionRow{{ID: id, Name: "unused", Secret: "hidden"}},
			}
			raw, marshalErr := structjson.Marshal(value, structjson.WithPathFieldExcluder(filter))
			if marshalErr != nil {
				t.Errorf("worker %d: %v", id, marshalErr)
				return
			}
			var got map[string]json.RawMessage
			if marshalErr = json.Unmarshal(raw, &got); marshalErr != nil {
				t.Errorf("worker %d decode: %v", id, marshalErr)
				return
			}
			if want := fmt.Sprintf("[{\"id\":%d}]", id); string(got["data"]) != want {
				t.Errorf("worker %d data=%s want %s", id, got["data"], want)
			}
		}(i)
	}
	group.Wait()
}

func TestLegacyStatusFieldFilterRejectsDuplicateScopes(t *testing.T) {
	_, err := structjson.NewFieldFilter(reflect.TypeFor[legacySelectionOutput](), []structjson.FieldSelection{
		{Path: []string{"Data"}, Fields: []string{"ID"}},
		{Path: []string{"Data"}, Fields: []string{"Name"}},
	})
	require.ErrorContains(t, err, "duplicate JSON selection scope")
}
