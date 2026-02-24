package json

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFormatTag_PerFieldMarshal(t *testing.T) {
	type meta struct {
		TraceID string `json:"traceId"`
	}
	type payload struct {
		UserName  string    `format:"caseFormat=lowerUnderscore"`
		CreatedAt time.Time `format:"dateFormat=yyyy-MM-dd"`
		Secret    string    `format:"ignore=true"`
		Note      string    `format:"omitempty=true"`
		Meta      meta      `format:"inline=true"`
	}

	in := payload{
		UserName:  "alice",
		CreatedAt: time.Date(2026, 2, 24, 10, 11, 12, 0, time.UTC),
		Secret:    "hidden",
		Meta:      meta{TraceID: "abc"},
	}
	data, err := Marshal(in)
	require.NoError(t, err)
	require.JSONEq(t, `{"user_name":"alice","CreatedAt":"2026-02-24","traceId":"abc"}`, string(data))
}

func TestFormatTag_PerFieldUnmarshal(t *testing.T) {
	type meta struct {
		TraceID string `json:"traceId"`
	}
	type payload struct {
		UserName  string    `format:"caseFormat=lowerUnderscore"`
		CreatedAt time.Time `format:"timeLayout=2006-01-02,name=created_at"`
		Secret    string    `format:"ignore=true"`
		Meta      meta      `format:"inline=true"`
	}

	var out payload
	err := Unmarshal([]byte(`{"user_name":"alice","created_at":"2026-02-24","traceId":"abc","Secret":"x"}`), &out)
	require.NoError(t, err)
	require.Equal(t, "alice", out.UserName)
	require.Equal(t, "abc", out.Meta.TraceID)
	require.Equal(t, "", out.Secret)
	require.Equal(t, time.Date(2026, 2, 24, 0, 0, 0, 0, time.UTC), out.CreatedAt.UTC())
}

func TestFormatTag_PerFieldNullableMarshal(t *testing.T) {
	type payload struct {
		Count int    `format:"name=count,nullable=true"`
		Name  string `format:"name=name,nullable=true"`
		Flag  bool   `format:"name=flag,nullable=true"`
	}
	data, err := Marshal(payload{})
	require.NoError(t, err)
	require.JSONEq(t, `{"count":null,"name":null,"flag":null}`, string(data))

	data, err = Marshal(payload{Count: 7, Name: "x", Flag: true})
	require.NoError(t, err)
	require.JSONEq(t, `{"count":7,"name":"x","flag":true}`, string(data))
}
