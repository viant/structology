package json

import (
	stdjson "encoding/json"
	"fmt"
	"testing"
)

type testCustomMarshaler struct {
	ID int
}

func (t testCustomMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"custom":%d}`, t.ID)), nil
}

type testCustomUnmarshaler struct {
	ID int
}

func (t *testCustomUnmarshaler) UnmarshalJSON(data []byte) error {
	var tmp struct {
		Custom int `json:"custom"`
	}
	if err := stdjson.Unmarshal(data, &tmp); err != nil {
		return err
	}
	t.ID = tmp.Custom
	return nil
}

type testTextCodec string

func (t testTextCodec) MarshalText() ([]byte, error) { return []byte("txt:" + string(t)), nil }
func (t *testTextCodec) UnmarshalText(text []byte) error {
	*t = testTextCodec(string(text))
	return nil
}

func TestStdlibCompat_CustomMarshal(t *testing.T) {
	data, err := Marshal(testCustomMarshaler{ID: 7})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"custom":7}`, string(data))
}

func TestStdlibCompat_CustomMarshalNested(t *testing.T) {
	type holder struct {
		Item testCustomMarshaler
	}
	data, err := Marshal(holder{Item: testCustomMarshaler{ID: 9}})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Item":{"custom":9}}`, string(data))
}

func TestStdlibCompat_CustomUnmarshal(t *testing.T) {
	var out testCustomUnmarshaler
	if err := Unmarshal([]byte(`{"custom":11}`), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.ID != 11 {
		t.Fatalf("unexpected value: %+v", out)
	}
}

func TestStdlibCompat_CustomUnmarshalNested(t *testing.T) {
	type holder struct {
		Item testCustomUnmarshaler
	}
	var out holder
	if err := Unmarshal([]byte(`{"Item":{"custom":13}}`), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.Item.ID != 13 {
		t.Fatalf("unexpected value: %+v", out)
	}
}

func TestStdlibCompat_TextCodec(t *testing.T) {
	type holder struct {
		Code testTextCodec
	}

	data, err := Marshal(holder{Code: testTextCodec("abc")})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Code":"txt:abc"}`, string(data))

	var out holder
	if err := Unmarshal([]byte(`{"Code":"xyz"}`), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if string(out.Code) != "xyz" {
		t.Fatalf("unexpected text codec value: %q", out.Code)
	}
}
