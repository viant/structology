package json

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format"
	"github.com/viant/tagly/format/text"
)

func TestFormatTag_Marshal_AppliesCaseAndDateFormat(t *testing.T) {
	type payload struct {
		UserName  string
		CreatedAt time.Time
	}

	in := payload{
		UserName:  "alice",
		CreatedAt: time.Date(2026, 2, 24, 10, 11, 12, 0, time.UTC),
	}
	tag := &format.Tag{
		CaseFormat: string(text.CaseFormatLowerUnderscore),
		DateFormat: "yyyy-MM-dd",
	}

	data, err := Marshal(in, WithFormatTag(tag))
	require.NoError(t, err)
	require.JSONEq(t, `{"user_name":"alice","created_at":"2026-02-24"}`, string(data))
}

func TestFormatTag_Unmarshal_AppliesCaseAndTimeLayout(t *testing.T) {
	type payload struct {
		UserName  string
		CreatedAt time.Time
	}

	var out payload
	tag := &format.Tag{
		CaseFormat: string(text.CaseFormatLowerUnderscore),
		TimeLayout: "2006-01-02",
	}
	err := Unmarshal([]byte(`{"user_name":"alice","created_at":"2026-02-24"}`), &out, WithFormatTag(tag))
	require.NoError(t, err)
	require.Equal(t, "alice", out.UserName)
	require.Equal(t, time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC), out.CreatedAt.UTC())
}

func TestFormatTag_WithCaseFormatOverride(t *testing.T) {
	type payload struct {
		UserName string
	}

	in := payload{UserName: "alice"}
	tag := &format.Tag{CaseFormat: string(text.CaseFormatLowerUnderscore)}

	data, err := Marshal(in, WithFormatTag(tag), WithCaseFormat(text.CaseFormatLowerCamel))
	require.NoError(t, err)
	require.JSONEq(t, `{"userName":"alice"}`, string(data))
}

func TestFormatTag_TopLevelTimeLayout(t *testing.T) {
	tm := time.Date(2026, 2, 24, 10, 11, 12, 0, time.UTC)
	tag := &format.Tag{TimeLayout: "2006-01-02"}

	data, err := Marshal(tm, WithFormatTag(tag))
	require.NoError(t, err)
	require.Equal(t, `"2026-02-24"`, string(data))

	var out time.Time
	err = Unmarshal([]byte(`"2026-02-24"`), &out, WithFormatTag(tag))
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC), out.UTC())
}
