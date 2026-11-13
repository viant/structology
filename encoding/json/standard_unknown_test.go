package json

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStandardUnknownPolicyNestedChoicesAndMaps(t *testing.T) {
	type item struct {
		Mode int `json:"mode" choice:"1:ENABLED" choice:"2:DISABLED" enumAccept:"literal" enumStyle:"literal"`
	}
	type envelope struct {
		Items  []item          `json:"items"`
		ByName map[string]item `json:"byName"`
	}
	for _, raw := range []string{
		`{"items":[{"mode":"ENABLED","extra":true}]}`,
		`{"byName":{"arbitrary-key":{"mode":"DISABLED","extra":true}}}`,
	} {
		var strict envelope
		err := UnmarshalStandard([]byte(raw), &strict, WithUnknownFieldPolicy(ErrorOnUnknown))
		require.ErrorContains(t, err, `json: unknown field "extra"`)
		var ordinary envelope
		require.NoError(t, UnmarshalStandard([]byte(raw), &ordinary))
	}
	var valid envelope
	require.NoError(t, UnmarshalStandard([]byte(`{"items":[{"mode":"ENABLED"}],"byName":{"arbitrary-key":{"mode":"DISABLED"}}}`), &valid, WithUnknownFieldPolicy(ErrorOnUnknown)))
	require.Equal(t, 1, valid.Items[0].Mode)
	require.Equal(t, 2, valid.ByName["arbitrary-key"].Mode)
	var typed struct {
		ID int `json:"id"`
	}
	require.Error(t, UnmarshalStandard([]byte(`{"id":"7"}`), &typed, WithUnknownFieldPolicy(ErrorOnUnknown)), "strict unknown policy must retain ordinary numeric decoding")
}
