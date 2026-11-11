package json_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
	"github.com/viant/tagly/format/text"
)

func TestIgnoredFieldsDoNotCompeteForDecodeNames(t *testing.T) {
	type row struct {
		Name    string `json:"name"`
		Private string `json:"-"`
		Secret  int    `json:"-"`
		Ignored string `json:"name" format:"ignore=true"`
	}
	for range 2 {
		value := row{Private: "retained", Secret: 7, Ignored: "unchanged"}
		err := structjson.Unmarshal([]byte(`{"name":"public","-":"injected","Private":"injected","Secret":9}`), &value, structjson.WithCaseFormat(text.CaseFormatLowerCamel))
		require.NoError(t, err)
		require.Equal(t, row{Name: "public", Private: "retained", Secret: 7, Ignored: "unchanged"}, value)
		encoded, err := structjson.Marshal(value)
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"public"}`, string(encoded))
	}
	// The ordinary JSON contract also permits multiple ignored fields.
	var baseline struct {
		Name string `json:"name"`
		A    string `json:"-"`
		B    string `json:"-"`
	}
	require.NoError(t, json.Unmarshal([]byte(`{"name":"public"}`), &baseline))
	require.Equal(t, "public", baseline.Name)
}
