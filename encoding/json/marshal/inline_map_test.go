package marshal

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

type InlineMapStatus struct {
	Status string         `json:"status,omitempty"`
	Errors any            `json:"errors,omitempty"`
	Extras map[string]any `json:"extras,omitempty" format:"inline=true"`
}
type inlineMapEnvelope struct {
	*InlineMapStatus
	Messages []string `json:"messages"`
	Data     int      `json:"data"`
}
type inlineMapDetail struct {
	SourceCode int
	Enabled    bool
}

func TestInlineMapPreservesAnonymousEnvelopeAndCollisionOrder(t *testing.T) {
	var typedNil *int
	value := inlineMapEnvelope{InlineMapStatus: &InlineMapStatus{Status: "original", Errors: inlineMapDetail{SourceCode: 7}, Extras: map[string]any{
		"zero": 0, "false": false, "empty": "", "nil": nil, "typedNil": typedNil,
		"status": "from-extra", "messages": []string{"injected"}, "data": 999,
		"UpperMapKey": inlineMapDetail{SourceCode: 8},
	}}, Messages: []string{}, Data: 123}
	lower := func(_ []string, name string) string { return strings.ToLower(name[:1]) + name[1:] }
	for _, dynamic := range []bool{false, true} {
		t.Run(map[bool]string{false: "static", true: "dynamic"}[dynamic], func(t *testing.T) {
			var transform func([]string, string) string
			if dynamic {
				transform = lower
			}
			e := New(transform, nil, false, true, "", "", nil)
			for _, mode := range []string{"value", "pointer", "append", "typed-pointer"} {
				t.Run(mode, func(t *testing.T) {
					var raw []byte
					var err error
					switch mode {
					case "value":
						raw, err = e.Marshal(value)
					case "pointer":
						raw, err = e.Marshal(&value)
					case "append":
						raw, err = e.MarshalTo(nil, value)
					case "typed-pointer":
						raw, err = e.MarshalPtr(&value)
					}
					if err != nil {
						t.Fatal(err)
					}
					var got map[string]json.RawMessage
					if err = json.Unmarshal(raw, &got); err != nil {
						t.Fatalf("invalid JSON %s: %v", raw, err)
					}
					for name, want := range map[string]string{"status": `"from-extra"`, "messages": "[]", "data": "123", "zero": "0", "false": "false", "empty": `""`, "nil": "null", "typedNil": "null"} {
						if string(got[name]) != want {
							t.Fatalf("%s=%s want%s: %s", name, got[name], want, raw)
						}
					}
					if _, exists := got["extras"]; exists {
						t.Fatalf("inline map gained wrapper: %s", raw)
					}
					for _, key := range []string{"errors", "UpperMapKey"} {
						var detail map[string]any
						if err := json.Unmarshal(got[key], &detail); err != nil {
							t.Fatal(err)
						}
						codeKey := "SourceCode"
						if dynamic {
							codeKey = "sourceCode"
						}
						want := float64(7)
						if key == "UpperMapKey" {
							want = 8
						}
						if detail[codeKey] != want {
							t.Fatalf("dynamic value authority lost: %s", raw)
						}
					}
					// Preserve ordinary map keys; only struct fields use NameTransform.
					if _, exists := got["upperMapKey"]; exists {
						t.Fatalf("inline map key transformed: %s", raw)
					}
				})
			}
		})
	}
}

func TestInlineMapNestedNilEmptyAndInterface(t *testing.T) {
	type holder struct {
		Extras any `format:"inline=true"`
		Tail   int `json:"tail"`
	}
	type nested struct {
		Child holder `json:"child"`
		Tail  int    `json:"tail"`
	}
	type namedMap map[string]any
	var nilMap map[string]any
	var nilPointer *map[string]any
	cases := []struct {
		name  string
		value any
		want  map[string]any
	}{
		{"nil-interface", nil, map[string]any{"tail": float64(9)}},
		{"typed-nil-map", nilMap, map[string]any{"tail": float64(9)}},
		{"nil-map-pointer", nilPointer, map[string]any{"tail": float64(9)}},
		{"empty-map", map[string]any{}, map[string]any{"tail": float64(9)}},
		{"named-map", namedMap{"value": false}, map[string]any{"tail": float64(9), "value": false}},
		{"integer-keys", map[int]any{0: "zero"}, map[string]any{"tail": float64(9), "0": "zero"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, dynamic := range []bool{false, true} {
				var transform func([]string, string) string
				if dynamic {
					transform = func(_ []string, name string) string { return strings.ToLower(name) }
				}
				e := New(transform, nil, false, true, "", "", nil)
				raw, err := e.Marshal(nested{Child: holder{Extras: tc.value, Tail: 9}, Tail: 5})
				if err != nil {
					t.Fatal(err)
				}
				var got map[string]any
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Fatalf("invalid JSON %s", raw)
				}
				if !reflect.DeepEqual(got["child"], tc.want) || got["tail"] != float64(5) {
					t.Fatalf("nested fields changed: %s", raw)
				}
			}
		})
	}
}

func TestInlineMapUsesExistingValueErrorsAndExclusionPaths(t *testing.T) {
	type value struct {
		Extra map[string]any `format:"inline=true"`
		Tail  int            `json:"tail"`
	}
	e := New(func(_ []string, name string) string { return strings.ToLower(name[:1]) + name[1:] }, func(path []string, name string) bool {
		return len(path) == 0 && name == "secret" || len(path) == 1 && path[0] == "nested" && name == "Enabled"
	}, false, true, "", "", nil)
	raw, err := e.Marshal(value{Extra: map[string]any{"secret": "hidden", "nested": inlineMapDetail{SourceCode: 3, Enabled: true}}, Tail: 1})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["secret"]; ok {
		t.Fatal("inline exclusion ignored")
	}
	if !reflect.DeepEqual(got["nested"], map[string]any{"sourceCode": float64(3)}) {
		t.Fatalf("inline path changed: %s", raw)
	}
	if _, err := e.Marshal(value{Extra: map[string]any{"unsupported": make(chan int)}}); err == nil {
		t.Fatal("inline value error swallowed")
	}
}
