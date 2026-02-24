package json

import "testing"

type rawCaptureUnmarshaler struct {
	Raw string
}

func (r *rawCaptureUnmarshaler) UnmarshalJSON(data []byte) error {
	r.Raw = string(data)
	return nil
}

func TestCustomUnmarshal_ReceivesRawJSONSpan(t *testing.T) {
	type holder struct {
		Item rawCaptureUnmarshaler
	}
	input := `{"Item": { "custom" : 1, "arr":[1, 2] }}`
	var out holder
	if err := Unmarshal([]byte(input), &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	want := `{ "custom" : 1, "arr":[1, 2] }`
	if out.Item.Raw != want {
		t.Fatalf("unexpected raw payload: got=%q want=%q", out.Item.Raw, want)
	}
}
