package marshal

import (
	"encoding/json"
	"reflect"
	"testing"
)

type wireFee struct {
	FeeTypeID int    `json:"feeTypeId"`
	Hidden    string `json:"-"`
}
type wireAudienceID int64

func TestWireIntegerMapMatchesNativePayload(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value any
		key   string
	}{
		{"signed", map[int][]wireFee{19: {{FeeTypeID: 3, Hidden: "PRIVATE"}}}, "19"},
		{"negative", map[int64][]wireFee{-9223372036854775808: {{FeeTypeID: 3}}}, "-9223372036854775808"},
		{"unsigned", map[uint64][]wireFee{18446744073709551615: {{FeeTypeID: 3}}}, "18446744073709551615"},
		{"named", map[wireAudienceID][]wireFee{41: {{FeeTypeID: 3}}}, "41"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			engine := New(nil, nil, false, true, "", "", nil)
			shape, err := engine.Wire(reflect.TypeOf(tc.value))
			if err != nil {
				t.Fatal(err)
			}
			if shape.Kind() != reflect.Map || !shape.Nullable() || shape.Element().Kind() != reflect.Slice {
				t.Fatalf("wrong map shape: %+v", shape)
			}
			fields := shape.Element().Element().Properties()
			if len(fields) != 1 || fields[0].Name() != "feeTypeId" {
				t.Fatalf("wrong value properties: %+v", fields)
			}
			raw, err := engine.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string][]map[string]any
			if err = json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if len(decoded) != 1 || len(decoded[tc.key]) != 1 || decoded[tc.key][0]["feeTypeId"] != float64(3) || len(decoded[tc.key][0]) != 1 {
				t.Fatalf("unexpected payload: %s", raw)
			}
		})
	}
}

func TestWireIntegerMapRetainsUnsupportedBoundaries(t *testing.T) {
	engine := New(nil, nil, false, true, "", "", nil)
	for _, typ := range []reflect.Type{reflect.TypeFor[map[bool]int](), reflect.TypeFor[map[float64]int](), reflect.TypeFor[map[struct{ ID int }]int]()} {
		if _, err := engine.Wire(typ); err == nil {
			t.Fatalf("unsupported key admitted: %v", typ)
		}
	}
	engine = New(nil, func(_ []string, _ string) bool { return false }, false, true, "", "", nil)
	if _, err := engine.Wire(reflect.TypeFor[map[int][]wireFee]()); err == nil {
		t.Fatal("path-dependent structured values must remain guarded")
	}
}
