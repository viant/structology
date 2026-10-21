package json_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
)

type selectionExcluder func([]string, string) bool

func (f selectionExcluder) ExcludePath(p []string, n string) bool { return f(p, n) }

type selectionBase struct {
	ID   int `json:"id"`
	Omit int `json:"omit,omitempty"`
}
type selectionConflict struct {
	ID int `json:"otherId"`
}
type selectionOpaque int

func (v selectionOpaque) MarshalJSON() ([]byte, error) { return []byte(`{"owned":true}`), nil }

type selectionZero struct{ N int }

func (*selectionZero) IsZero() bool { return true }

func TestStandardSelectionJSONSemantics(t *testing.T) {
	type row struct {
		*selectionBase
		selectionConflict
		Name        string          `json:"label,string"`
		Count       int             `json:"count,string"`
		Optional    *int            `json:"optional"`
		Fixed       [2]int          `json:"fixed,omitempty"`
		EmptyStruct struct{}        `json:"empty,omitempty"`
		Zero        selectionZero   `json:"zero,omitzero"`
		Opaque      selectionOpaque `json:"opaque"`
		Bytes       []byte          `json:"bytes"`
		Ignore      string          `json:"-"`
	}
	for _, value := range []any{row{}, row{selectionBase: &selectionBase{ID: 2}, Name: "<a>\n", Bytes: []byte{1, 2}}, &row{selectionBase: &selectionBase{}, Count: 0}} {
		want, err := json.Marshal(value)
		require.NoError(t, err)
		got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(selectionExcluder(func([]string, string) bool { return false })))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	}
}

func TestStandardSelectionAliasesPromotedAndSharedPaths(t *testing.T) {
	type row struct {
		selectionBase
		Name   string  `json:"display_name"`
		Secret string  `json:"secret"`
		Nil    *string `json:"nil"`
	}
	type output struct {
		First  []row `json:"first.branch"`
		Second []row `json:"second"`
	}
	value := output{First: []row{{selectionBase: selectionBase{ID: 0}, Secret: "hidden"}}, Second: []row{{Name: "two", Secret: "hidden"}}}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{
		{Path: []string{"First"}, Indexes: [][]int{{0, 0}}, Fields: []string{"Nil"}},
		{Path: []string{"Second"}, Fields: []string{"Name"}},
	})
	require.NoError(t, err)
	got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	require.NoError(t, err)
	require.JSONEq(t, `{"first.branch":[{"id":0,"nil":null}],"second":[{"display_name":"two"}]}`, string(got))
	require.Equal(t, "hidden", value.First[0].Secret)
}

func TestStandardSelectionCustomAndCycles(t *testing.T) {
	got, err := structjson.MarshalStandard(selectionOpaque(0), structjson.WithPathFieldExcluder(selectionExcluder(func([]string, string) bool { return true })))
	require.NoError(t, err)
	require.Equal(t, `{"owned":true}`, string(got))
	type node struct{ Next *node }
	value := &node{}
	value.Next = value
	_, err = structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(selectionExcluder(func([]string, string) bool { return false })))
	require.Error(t, err)
}

func TestStandardSelectionOmitZeroInterfaceAuthority(t *testing.T) {
	type row struct {
		Any  any                        `json:"any,omitzero"`
		Zero interface{ IsZero() bool } `json:"zero,omitzero"`
	}
	var missing *selectionZero
	for _, value := range []row{{Any: missing, Zero: missing}, {Any: selectionZero{}, Zero: &selectionZero{N: 1}}} {
		want, err := json.Marshal(value)
		require.NoError(t, err)
		got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(selectionExcluder(func([]string, string) bool { return false })))
		require.NoError(t, err)
		require.Equal(t, string(want), string(got))
	}
}

func TestStandardSelectionDistinguishesSameNamedPromotedFields(t *testing.T) {
	type A struct {
		Value int `json:"a"`
	}
	type B struct {
		Value int `json:"b"`
	}
	type row struct {
		A
		B
	}
	value := row{A: A{Value: 1}, B: B{Value: 2}}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Indexes: [][]int{{1, 0}}}})
	require.NoError(t, err)
	got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	require.NoError(t, err)
	require.JSONEq(t, `{"b":2}`, string(got))
}

func TestStandardSelectionHiddenRelationDoesNotCreateWireFields(t *testing.T) {
	type row struct {
		ID int `json:"id"`
	}
	type output struct {
		Rows   []row  `json:"-"`
		Status string `json:"status"`
	}
	value := output{Rows: []row{{ID: 1}}, Status: "ok"}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Rows"}, Fields: []string{"ID"}}})
	require.NoError(t, err)
	got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	require.NoError(t, err)
	require.JSONEq(t, `{"status":"ok"}`, string(got))
}

func TestStandardSelectionRejectsIncompatibleTransformedInline(t *testing.T) {
	type detail struct{ Name string }
	type row struct {
		Detail detail `jsonx:"inline"`
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[row](), []structjson.FieldSelection{{Fields: []string{"Detail"}}})
	require.NoError(t, err)
	_, err = filter.WithOptions()
	require.ErrorContains(t, err, "matching standard and transformed JSON")
}

func TestStandardSelectionCustomEncoderOwnsStringTaggedField(t *testing.T) {
	type row struct {
		Opaque selectionOpaque `json:"value,string"`
	}
	value := row{}
	want, err := json.Marshal(value)
	require.NoError(t, err)
	got, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(selectionExcluder(func([]string, string) bool { return false })))
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
}

func TestCanonicalSelectionKeepsRootRelativeEmbeddedIndexes(t *testing.T) {
	type Base struct {
		ID     int
		Secret string
	}
	type row struct {
		Base
		Name string
	}
	type Envelope struct{ Rows []row }
	type root struct{ Envelope }
	filter, err := structjson.NewFieldFilter(reflect.TypeFor[root](), []structjson.FieldSelection{{Path: []string{"Rows"}, Indexes: [][]int{{0, 0}}}})
	require.NoError(t, err)
	// Rows lives under Envelope, and ID lives under the row's Base.
	require.False(t, filter.ExcludeIndexes([]int{0, 0, 0, 0}))
	require.True(t, filter.ExcludeIndexes([]int{0, 0, 0, 1}))
	require.True(t, filter.ExcludeIndexes([]int{0, 0, 1}))
	require.False(t, filter.ExcludeIndexes([]int{0, 0, 0}))
}
