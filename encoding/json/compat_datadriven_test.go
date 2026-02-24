package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/viant/tagly/format/text"
)

type compatExcluder map[string]map[string]bool

func (c compatExcluder) Exclude(path, fieldName string) bool {
	if byPath, ok := c[path]; ok {
		return byPath[fieldName]
	}
	return false
}

func TestCompatDataDriven_Marshal(t *testing.T) {
	type eventType struct {
		Id   int
		Type string
	}
	type primitive struct {
		Int     int
		Int8    int8
		Uint8   uint8
		Int16   int16
		Uint16  uint16
		Int32   int32
		Uint32  uint32
		Int64   int64
		Uint64  uint64
		Byte    byte
		String  string
		Float32 float32
		Float64 float64
		Bool    bool
	}
	type primitivePtr struct {
		Int     *int
		Int8    *int8
		Uint8   *uint8
		Int16   *int16
		Uint16  *uint16
		Int32   *int32
		Uint32  *uint32
		Int64   *int64
		Uint64  *uint64
		Byte    *byte
		String  *string
		Float32 *float32
		Float64 *float64
		Bool    *bool
	}
	type event struct {
		Int       int
		String    string
		Float64   float64
		EventType *eventType
	}
	type withCase struct {
		ID       int
		Quantity float64
		TimePtr  *time.Time `json:"time_ptr,omitempty"`
	}
	type inlineBody struct {
		Name  string `json:"name"`
		Price float64
	}
	type inline struct {
		ID   int        `json:"id"`
		Body inlineBody `jsonx:"inline"`
	}

	i, i8, u8 := 1, int8(2), uint8(3)
	i16, u16 := int16(4), uint16(5)
	i32, u32 := int32(6), uint32(7)
	i64, u64 := int64(8), uint64(9)
	b := byte(10)
	s := "string"
	f32, f64 := float32(5.5), 11.5
	bl := true
	tm := time.Date(2012, 7, 12, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name   string
		value  interface{}
		expect string
		opts   []Option
	}{
		{
			name: "primitive",
			value: primitive{
				Int: 1, Int8: 2, Uint8: 3, Int16: 4, Uint16: 5, Int32: 6, Uint32: 7,
				Int64: 8, Uint64: 9, Byte: 10, String: "string", Float32: 5.5, Float64: 11.5, Bool: true,
			},
			expect: `{"Int":1,"Int8":2,"Uint8":3,"Int16":4,"Uint16":5,"Int32":6,"Uint32":7,"Int64":8,"Uint64":9,"Byte":10,"String":"string","Float32":5.5,"Float64":11.5,"Bool":true}`,
		},
		{
			name: "primitive pointers",
			value: primitivePtr{
				Int: &i, Int8: &i8, Uint8: &u8, Int16: &i16, Uint16: &u16, Int32: &i32, Uint32: &u32,
				Int64: &i64, Uint64: &u64, Byte: &b, String: &s, Float32: &f32, Float64: &f64, Bool: &bl,
			},
			expect: `{"Int":1,"Int8":2,"Uint8":3,"Int16":4,"Uint16":5,"Int32":6,"Uint32":7,"Int64":8,"Uint64":9,"Byte":10,"String":"string","Float32":5.5,"Float64":11.5,"Bool":true}`,
		},
		{
			name:   "nil pointers",
			value:  primitivePtr{},
			expect: `{"Int":null,"Int8":null,"Uint8":null,"Int16":null,"Uint16":null,"Int32":null,"Uint32":null,"Int64":null,"Uint64":null,"Byte":null,"String":null,"Float32":null,"Float64":null,"Bool":null}`,
		},
		{
			name: "slice with relations",
			value: event{
				Int: 100, String: "abc", Float64: 0,
				EventType: &eventType{Id: 200, Type: "event-type-1"},
			},
			expect: `{"Int":100,"String":"abc","Float64":0,"EventType":{"Id":200,"Type":"event-type-1"}}`,
		},
		{
			name:   "case format",
			value:  []withCase{{ID: 1, Quantity: 125.5, TimePtr: &tm}},
			expect: `[{"id":1,"quantity":125.5,"time_ptr":"2012-07-12T00:00:00Z"}]`,
			opts:   []Option{WithCaseFormat(text.CaseFormatLowerUnderscore)},
		},
		{
			name: "field excluder",
			value: event{
				Int: 100, String: "abc", Float64: 0,
				EventType: &eventType{Id: 200, Type: "event-type-1"},
			},
			expect: `{"Int":100,"EventType":{"Type":"event-type-1"}}`,
			opts: []Option{WithFieldExcluder(compatExcluder{
				"":          {"String": true, "Float64": true},
				"EventType": {"Id": true},
			})},
		},
		{
			name:   "inline",
			value:  inline{ID: 12, Body: inlineBody{Name: "Foo name", Price: 125.567}},
			expect: `{"id":12,"name":"Foo name","price":125.567}`,
			opts:   []Option{WithCaseFormat(text.CaseFormatLowerCamel)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, err := Marshal(tc.value, tc.opts...)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			assertJSONEqual(t, tc.expect, string(data))
		})
	}
}

func TestCompatDataDriven_Unmarshal(t *testing.T) {
	type eventType struct {
		Id   int
		Type string
	}
	type sample struct {
		Int       int
		String    string
		Float64   float64
		EventType *eventType
		Tags      []string
	}

	cases := []struct {
		name    string
		input   string
		checkFn func(t *testing.T, got sample)
	}{
		{
			name:  "basic",
			input: `{"Int":100,"String":"abc","Float64":0.5,"EventType":{"Id":200,"Type":"event-type-1"},"Tags":["x","y"]}`,
			checkFn: func(t *testing.T, got sample) {
				if got.Int != 100 || got.String != "abc" || got.Float64 != 0.5 {
					t.Fatalf("unexpected primitive fields: %+v", got)
				}
				if got.EventType == nil || got.EventType.Id != 200 || got.EventType.Type != "event-type-1" {
					t.Fatalf("unexpected nested struct: %+v", got.EventType)
				}
				if !reflect.DeepEqual(got.Tags, []string{"x", "y"}) {
					t.Fatalf("unexpected tags: %#v", got.Tags)
				}
			},
		},
		{
			name:  "case insensitive keys",
			input: `{"int":10,"STRING":"A","eventtype":{"id":2,"type":"T"},"tags":["k"]}`,
			checkFn: func(t *testing.T, got sample) {
				if got.Int != 10 || got.String != "A" {
					t.Fatalf("unexpected primitive fields: %+v", got)
				}
				if got.EventType == nil || got.EventType.Id != 2 || got.EventType.Type != "T" {
					t.Fatalf("unexpected nested struct: %+v", got.EventType)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out sample
			if err := Unmarshal([]byte(tc.input), &out); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}
			tc.checkFn(t, out)
		})
	}
}

func assertJSONEqual(t *testing.T, expect, actual string) {
	t.Helper()
	var e interface{}
	var a interface{}
	if err := stdjson.Unmarshal([]byte(expect), &e); err != nil {
		t.Fatalf("invalid expected JSON: %v", err)
	}
	if err := stdjson.Unmarshal([]byte(actual), &a); err != nil {
		t.Fatalf("invalid actual JSON: %v\nactual=%s", err, actual)
	}
	if !reflect.DeepEqual(e, a) {
		t.Fatalf("json mismatch\nexpect=%s\nactual=%s", expect, actual)
	}
}
