package json_test

import (
	json "github.com/viant/structology/encoding/json"
	"testing"
)

type guardedChild struct {
	Value int `json:"value" format:"inputUnless=../Canonical"`
}
type guardedRoot struct {
	Canonical *int                       `json:"canonical"`
	Children  map[string][]*guardedChild `json:"children"`
}

func TestAncestorInputGuard(t *testing.T) {
	for _, decode := range []func([]byte, any) error{func(b []byte, v any) error { return json.UnmarshalStandard(b, v) }, func(b []byte, v any) error { return json.Unmarshal(b, v) }} {
		for _, raw := range []string{`{"canonical":null,"children":{"a":[{"value":"ignored"}]}}`, `{"children":{"a":[{"value":{}}]},"canonical":0}`} {
			var target guardedRoot
			if err := decode([]byte(raw), &target); err != nil {
				t.Fatal(err)
			}
			if target.Children["a"][0].Value != 0 {
				t.Fatal("guarded value set")
			}
		}
		var target guardedRoot
		if err := decode([]byte(`{"children":{"a":[{"value":"bad"}]}}`), &target); err == nil {
			t.Fatal("active malformed value accepted")
		}
		if err := decode([]byte(`{}`), &guardedChild{}); err == nil {
			t.Fatal("invalid ancestor context accepted")
		}
	}
}

func TestAncestorGuardContextAndIsolation(t *testing.T) {
	for _, decode := range []func([]byte, any) error{func(b []byte, v any) error { return json.UnmarshalStandard(b, v) }, func(b []byte, v any) error { return json.Unmarshal(b, v) }} {
		var rows []guardedRoot
		if err := decode([]byte(`[{"canonical":1,"children":{"x":[{"value":"ignored"}]}},{"children":{"x":[{"value":7}]}}]`), &rows); err != nil {
			t.Fatal(err)
		}
		if rows[1].Children["x"][0].Value != 7 {
			t.Fatal("ancestor state leaked")
		}
		for _, raw := range []string{`{"canonical":1,"children":{"a":[{"value":invalid}]}}`, `{"children":{"a":[{"value":"bad"}]}}`} {
			var row guardedRoot
			if err := decode([]byte(raw), &row); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		}
		type invalid struct {
			Canonical int `json:"-"`
			Children  []*guardedChild
		}
		if err := decode([]byte(`{}`), &invalid{}); err == nil {
			t.Fatal("hidden ancestor target accepted")
		}
	}
}
