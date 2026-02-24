package json

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format/text"
)

func TestTagPrecedence_JSONBeatsFormatNameCase(t *testing.T) {
	type payload struct {
		A int `json:"json_name" format:"name=format_name,caseFormat=upperUnderscore"`
	}

	data, err := Marshal(payload{A: 7})
	require.NoError(t, err)
	require.JSONEq(t, `{"json_name":7}`, string(data))

	var out payload
	require.NoError(t, Unmarshal([]byte(`{"json_name":9}`), &out))
	require.Equal(t, 9, out.A)

	out = payload{}
	require.NoError(t, Unmarshal([]byte(`{"format_name":9}`), &out))
	require.Equal(t, 0, out.A)
}

func TestTagPrecedence_JSONExplicitEmptyBeatsFormatCase(t *testing.T) {
	type payload struct {
		UserID int `json:",omitempty" format:"caseFormat=lowerUnderscore"`
	}

	data, err := Marshal(payload{UserID: 1})
	require.NoError(t, err)
	require.JSONEq(t, `{"UserID":1}`, string(data))

	data, err = Marshal(payload{UserID: 1}, WithCaseFormat(text.CaseFormatLowerCamel))
	require.NoError(t, err)
	require.JSONEq(t, `{"UserID":1}`, string(data))
}

func TestTagPrecedence_IgnoreBeatsInline(t *testing.T) {
	type embedded struct {
		X int `json:"x"`
	}
	type payload struct {
		Embedded embedded `jsonx:"inline" format:"ignore=true"`
		Y        int      `json:"y"`
	}

	data, err := Marshal(payload{Embedded: embedded{X: 1}, Y: 2})
	require.NoError(t, err)
	require.JSONEq(t, `{"y":2}`, string(data))

	var out payload
	require.NoError(t, Unmarshal([]byte(`{"x":9,"y":3}`), &out))
	require.Equal(t, 0, out.Embedded.X)
	require.Equal(t, 3, out.Y)
}

func TestTagPrecedence_FormatInlineWithoutJSONX(t *testing.T) {
	type embedded struct {
		X int `json:"x"`
	}
	type payload struct {
		Embedded embedded `format:"inline=true"`
		Y        int      `json:"y"`
	}

	data, err := Marshal(payload{Embedded: embedded{X: 1}, Y: 2})
	require.NoError(t, err)
	require.JSONEq(t, `{"x":1,"y":2}`, string(data))

	var out payload
	require.NoError(t, Unmarshal([]byte(`{"x":9,"y":3}`), &out))
	require.Equal(t, 9, out.Embedded.X)
	require.Equal(t, 3, out.Y)
}
