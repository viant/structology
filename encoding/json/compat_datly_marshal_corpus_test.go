package json

import (
	stdjson "encoding/json"
	"testing"
	"time"

	"github.com/viant/tagly/format/text"
)

func TestCompat_DatlyMarshalCorpus(t *testing.T) {
	type testCase struct {
		description string
		data        func() interface{}
		expect      string
		opts        []Option
	}

	cases := []testCase{
		{
			description: "primitive",
			data:        datlyCorpusEvent,
			expect:      `{"Int":1,"Int8":2,"Uint8":3,"Int16":4,"Uint16":5,"Int32":6,"Uint32":7,"Int64":8,"Uint64":9,"Byte":10,"String":"string","Float32":5.5,"Float64":11.5,"Bool":true}`,
		},
		{
			description: "primitive pointers",
			data:        datlyCorpusEventPtr,
			expect:      `{"Int":1,"Int8":2,"Uint8":3,"Int16":4,"Uint16":5,"Int32":6,"Uint32":7,"Int64":8,"Uint64":9,"Byte":10,"String":"string","Float32":5.5,"Float64":11.5,"Bool":true}`,
		},
		{
			description: "nils",
			data:        datlyCorpusNilsPtr,
			expect:      `{"Int":null,"Int8":null,"Uint8":null,"Int16":null,"Uint16":null,"Int32":null,"Uint32":null,"Int64":null,"Uint64":null,"Byte":null,"String":null,"Float32":null,"Float64":null,"Bool":null}`,
		},
		{
			description: "slice without relations",
			data:        datlyCorpusSliceWithoutRelations,
			expect:      `[{"Int":10,"String":"str - 1","Float64":20.5},{"Int":15,"String":"str - 2","Float64":40.5},{"Int":5,"String":"str - 0","Float64":0.5}]`,
		},
		{
			description: "slice with relations",
			data:        datlyCorpusSliceWithRelations,
			expect:      `{"Int":100,"String":"abc","Float64":0,"EventType":{"Id":200,"Type":"event-type-1"}}`,
		},
		{
			description: "nil slice and *T",
			data:        datlyCorpusNilNonPrimitives,
			expect:      `[{"Id":231,"EventTypesEmpty":null,"EventTypes":[{"Id":1,"Type":"t - 1"},null,{"Id":1,"Type":"t - 3"}],"Name":"","EventType":null}]`,
		},
		{
			description: "caser and json tags",
			data:        datlyCorpusCaserAndJson,
			expect:      `[{"id":1,"quantity":125.5,"EventName":"ev-1","time_ptr":"2012-07-12T00:00:00Z"},{"id":2,"quantity":250.5,"time":"2022-05-10T00:00:00Z"}]`,
			opts:        []Option{WithCaseFormat(text.CaseFormatLowerUnderscore), WithOmitEmpty(true)},
		},
		{
			description: "filtered fields",
			data:        datlyCorpusSliceWithRelations,
			expect:      `{"Int":100,"EventType":{"Type":"event-type-1"}}`,
			opts:        []Option{WithPathFieldExcluder(datlyCorpusFilterExcluder())},
		},
		{
			description: "interface",
			data:        datlyCorpusWithInterface,
			expect:      `{"Int":100,"EventType":{"Type":"event-type-1"}}`,
			opts:        []Option{WithPathFieldExcluder(datlyCorpusFilterExcluder())},
		},
		{
			description: "anonymous",
			data:        datlyCorpusAnonymous,
			expect:      `{"Id":10,"Quantity":125.5}`,
			opts:        []Option{WithPathFieldExcluder(datlyCorpusAnonymousExcluder())},
		},
		{
			description: "primitive slice",
			data:        datlyCorpusPrimitiveSlice,
			expect:      `["abc","def","ghi"]`,
		},
		{
			description: "primitive nested slice",
			data:        datlyCorpusPrimitiveNestedSlice,
			expect:      `[{"Name":"N - 1","Price":125.5,"Ints":[1,2,3]},{"Name":"N - 1","Price":250.5,"Ints":[4,5,6]}]`,
		},
		{
			description: "anonymous nested struct",
			data:        datlyCorpusAnonymousNestedStruct,
			expect:      `{"ResponseStatus":{"Message":"","Status":"","Error":""},"Foo":[{"ID":1,"Name":"abc","Quantity":0},{"ID":2,"Name":"def","Quantity":250}]}`,
		},
		{
			description: "anonymous nested struct with ptrs",
			data:        datlyCorpusAnonymousNestedStructWithPointers,
			expect:      `{"FooWrapperName":"","Foo":[{"ID":1,"Name":"abc","Quantity":0},{"ID":2,"Name":"def","Quantity":250}]}`,
		},
		{
			description: "anonymous nested complex struct with ptrs",
			data:        datlyCorpusComplexAnonymousNestedStructWithPointers,
			expect:      `{"Status":0,"Message":"","FooWrapperName":"","Foo":[{"ID":1,"Name":"abc","Quantity":0},{"ID":2,"Name":"def","Quantity":250}],"Timestamp":"0001-01-01T00:00:00Z"}`,
		},
		{
			description: "ID field",
			data:        datlyCorpusIDStruct,
			expect:      `[{"id":10,"name":"foo","price":125.5}]`,
			opts:        []Option{WithCaseFormat(text.CaseFormatLowerCamel)},
		},
		{
			description: "inlining",
			data:        datlyCorpusInlinable,
			expect:      `{"id":12,"name":"Foo name","price":125.567}`,
			opts:        []Option{WithCaseFormat(text.CaseFormatLowerCamel)},
		},
		{
			description: "json.RawMessage",
			data:        datlyCorpusJSONRawMessage,
			expect:      `{"id":12,"name":"Foo name","price":125.567}`,
			opts:        []Option{WithCaseFormat(text.CaseFormatLowerCamel)},
		},
		{
			description: "*json.RawMessage",
			data:        datlyCorpusJSONRawMessagePtr,
			expect:      `{"id":12,"name":"Foo name","price":125.567}`,
			opts:        []Option{WithCaseFormat(text.CaseFormatLowerCamel)},
		},
		{
			description: "interface slice",
			data:        datlyCorpusInterfaceSlice,
			expect:      `{"ID":1,"Name":"abc","MgrId":0,"AccountId":2,"Team":[{"ID":10,"Name":"xx","MgrId":0,"AccountId":2,"Team":null}]}`,
		},
		{
			description: "escaping special characters",
			data:        datlyCorpusEscapingSpecialCharacters,
			expect:      `{}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.description, func(t *testing.T) {
			data, err := Marshal(tc.data(), tc.opts...)
			if err != nil {
				t.Fatalf("marshal error: %v", err)
			}
			assertJSONEqual(t, tc.expect, string(data))
		})
	}
}

func datlyCorpusFilterExcluder() PathFieldExcluder {
	return PathFieldExcluderFunc(func(path []string, fieldName string) bool {
		if len(path) == 0 {
			return fieldName != "Int" && fieldName != "EventType"
		}
		if len(path) == 1 && path[0] == "EventType" {
			return fieldName != "Type"
		}
		return false
	})
}

func datlyCorpusAnonymousExcluder() PathFieldExcluder {
	return PathFieldExcluderFunc(func(path []string, fieldName string) bool {
		if len(path) == 0 {
			return fieldName != "Id" && fieldName != "Quantity"
		}
		return false
	})
}

func datlyCorpusJSONRawMessage() interface{} {
	type Foo struct {
		ID       int
		JSONBody stdjson.RawMessage `jsonx:"inline"`
		Name     string
	}
	jsonBody := stdjson.RawMessage([]byte(`{"id":12,"name":"Foo name","price":125.567}`))
	return &Foo{ID: 125, Name: "Abdef", JSONBody: jsonBody}
}

func datlyCorpusJSONRawMessagePtr() interface{} {
	type Foo struct {
		ID       int
		JSONBody *stdjson.RawMessage `jsonx:"inline"`
		Name     string
	}
	jsonBody := stdjson.RawMessage([]byte(`{"id":12,"name":"Foo name","price":125.567}`))
	return &Foo{ID: 125, Name: "Abdef", JSONBody: &jsonBody}
}

func datlyCorpusIDStruct() interface{} {
	type Foo struct {
		ID    int
		Name  string
		Price float64
	}
	return []*Foo{{ID: 10, Name: "foo", Price: 125.5}}
}

func datlyCorpusComplexAnonymousNestedStructWithPointers() interface{} {
	type Foo struct {
		ID       int
		Name     string
		Quantity int
	}
	type FooWrapper struct {
		FooWrapperName string
		Foo            []*Foo
	}
	type Response struct {
		Status  int
		Message string
		*FooWrapper
		Timestamp time.Time
	}
	return Response{FooWrapper: &FooWrapper{Foo: []*Foo{{ID: 1, Name: "abc"}, {ID: 2, Name: "def", Quantity: 250}}}}
}

func datlyCorpusAnonymousNestedStructWithPointers() interface{} {
	type Foo struct {
		ID       int
		Name     string
		Quantity int
	}
	type FooWrapper struct {
		FooWrapperName string
		Foo            []*Foo
	}
	type Response struct {
		*FooWrapper
	}
	return &Response{FooWrapper: &FooWrapper{Foo: []*Foo{{ID: 1, Name: "abc"}, {ID: 2, Name: "def", Quantity: 250}}}}
}

func datlyCorpusAnonymousNestedStruct() interface{} {
	type Foo struct {
		ID       int
		Name     string
		Quantity int
	}
	type FooWrapper struct {
		Foo []*Foo
	}
	type ResponseStatus struct {
		Message string
		Status  string
		Error   string
	}
	type Response struct {
		ResponseStatus ResponseStatus
		FooWrapper
	}
	return Response{FooWrapper: FooWrapper{Foo: []*Foo{{ID: 1, Name: "abc"}, {ID: 2, Name: "def", Quantity: 250}}}}
}

func datlyCorpusPrimitiveNestedSlice() interface{} {
	type Foo struct {
		Name  string
		Price float64
		Ints  []int
	}
	return []Foo{{Name: "N - 1", Price: 125.5, Ints: []int{1, 2, 3}}, {Name: "N - 1", Price: 250.5, Ints: []int{4, 5, 6}}}
}

func datlyCorpusPrimitiveSlice() interface{} { return []string{"abc", "def", "ghi"} }

func datlyCorpusAnonymous() interface{} {
	type Event struct {
		Id       int
		Name     string
		Quantity float64
	}
	type holder struct{ Event }
	return holder{Event{Id: 10, Name: "event - name", Quantity: 125.5}}
}

func datlyCorpusCaserAndJson() interface{} {
	type event struct {
		Id       int
		Quantity float64
		Name     string `json:"EventName,omitempty"`
		//lint:ignore SA5008 Datly parity: intentionally malformed legacy tag semantics.
		Type    string `json:"-,omitempty"`
		TimePtr *time.Time
		Time    time.Time
	}
	timePtr := datlyCorpusNewTime("12-07-2012")
	return []*event{
		{Id: 1, Quantity: 125.5, Name: "ev-1", Type: "removed from json", TimePtr: &timePtr},
		{Id: 2, Quantity: 250.5, Type: "removed from json", Time: datlyCorpusNewTime("10-05-2022")},
	}
}

func datlyCorpusNewTime(s string) time.Time {
	layout := "02-01-2006"
	ret, _ := time.Parse(layout, s)
	return ret
}

func datlyCorpusNilNonPrimitives() interface{} {
	type eventType struct {
		Id   int
		Type string
	}
	type event struct {
		Id              int
		EventTypesEmpty []*eventType
		EventTypes      []*eventType
		Name            string
		EventType       *eventType
	}
	return []*event{{Id: 231, EventTypes: []*eventType{{Id: 1, Type: "t - 1"}, nil, {Id: 1, Type: "t - 3"}}}}
}

func datlyCorpusWithInterface() interface{} {
	type eventType struct {
		Id   int
		Type string
	}
	type event struct {
		Int       int
		String    string
		Float64   float64
		EventType interface{}
	}
	return event{Int: 100, String: "abc", EventType: eventType{Id: 200, Type: "event-type-1"}}
}

func datlyCorpusSliceWithRelations() interface{} {
	type eventType struct {
		Id   int
		Type string
	}
	type event struct {
		Int       int
		String    string
		Float64   float64
		EventType eventType
	}
	return event{Int: 100, String: "abc", EventType: eventType{Id: 200, Type: "event-type-1"}}
}

func datlyCorpusSliceWithoutRelations() interface{} {
	type event struct {
		Int     int
		String  string
		Float64 float64
	}
	return []event{{Int: 10, String: "str - 1", Float64: 20.5}, {Int: 15, String: "str - 2", Float64: 40.5}, {Int: 5, String: "str - 0", Float64: 0.5}}
}

func datlyCorpusNilsPtr() interface{} {
	type event struct {
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
	return &event{}
}

func datlyCorpusEvent() interface{} {
	type event struct {
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
	return event{Int: 1, Int8: 2, Uint8: 3, Int16: 4, Uint16: 5, Int32: 6, Uint32: 7, Int64: 8, Uint64: 9, Byte: 10, String: "string", Float32: 5.5, Float64: 11.5, Bool: true}
}

func datlyCorpusEventPtr() interface{} {
	type event struct {
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
	intV := 1
	int8V := int8(2)
	uint8V := uint8(3)
	int16V := int16(4)
	uint16V := uint16(5)
	int32V := int32(6)
	uint32V := uint32(7)
	int64V := int64(8)
	uint64V := uint64(9)
	byteV := byte(10)
	stringV := "string"
	float32V := float32(5.5)
	float64V := 11.5
	boolV := true
	return event{
		Int:     &intV,
		Int8:    &int8V,
		Uint8:   &uint8V,
		Int16:   &int16V,
		Uint16:  &uint16V,
		Int32:   &int32V,
		Uint32:  &uint32V,
		Int64:   &int64V,
		Uint64:  &uint64V,
		Byte:    &byteV,
		String:  &stringV,
		Float32: &float32V,
		Float64: &float64V,
		Bool:    &boolV,
	}
}

func datlyCorpusInlinable() interface{} {
	type Foo struct {
		ID    int
		Name  string
		Price float64
	}
	type FooAudit struct {
		CreatedAt time.Time
		UpdatedAt time.Time
		Foo       Foo `jsonx:"inline"`
	}
	return &FooAudit{
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Foo: Foo{
			ID:    12,
			Name:  "Foo name",
			Price: 125.567,
		},
	}
}

func datlyCorpusInterfaceSlice() interface{} {
	type Member struct {
		ID        int
		Name      string
		MgrId     int
		AccountId int
		Team      []interface{}
	}
	return &Member{
		ID:        1,
		Name:      "abc",
		AccountId: 2,
		Team: []interface{}{
			&Member{ID: 10, Name: "xx", AccountId: 2},
		},
	}
}

func datlyCorpusEscapingSpecialCharacters() interface{} {
	type Member struct {
		escaped string
	}
	return &Member{escaped: "\\__\"__/__\b__\f__\n__\r__\t__"}
}

type PathFieldExcluderFunc func(path []string, fieldName string) bool

func (f PathFieldExcluderFunc) ExcludePath(path []string, fieldName string) bool {
	return f(path, fieldName)
}
