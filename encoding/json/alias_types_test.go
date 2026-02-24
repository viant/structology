package json

import (
	stdjson "encoding/json"
	"testing"
)

type aliasInt int
type aliasString string
type aliasSlice []int
type aliasArray [3]int
type aliasInner struct {
	Name string
}
type aliasStruct aliasInner

type aliasPayload struct {
	I  aliasInt
	S  aliasString
	Sl aliasSlice
	Ar aliasArray
	St aliasStruct
}

func TestAliasTypes_Marshal(t *testing.T) {
	in := aliasPayload{
		I:  7,
		S:  "abc",
		Sl: aliasSlice{1, 2, 3},
		Ar: aliasArray{4, 5, 6},
		St: aliasStruct{Name: "inner"},
	}
	data, err := Marshal(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var got map[string]interface{}
	if err := stdjson.Unmarshal(data, &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got["I"].(float64) != 7 {
		t.Fatalf("unexpected I: %v", got["I"])
	}
	if got["S"].(string) != "abc" {
		t.Fatalf("unexpected S: %v", got["S"])
	}
	if len(got["Sl"].([]interface{})) != 3 {
		t.Fatalf("unexpected Sl: %v", got["Sl"])
	}
	if len(got["Ar"].([]interface{})) != 3 {
		t.Fatalf("unexpected Ar: %v", got["Ar"])
	}
	st := got["St"].(map[string]interface{})
	if st["Name"].(string) != "inner" {
		t.Fatalf("unexpected St.Name: %v", st["Name"])
	}
}

func TestAliasTypes_Unmarshal(t *testing.T) {
	var out aliasPayload
	data := []byte(`{"I":7,"S":"abc","Sl":[1,2,3],"Ar":[4,5,6],"St":{"Name":"inner"}}`)
	if err := Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if out.I != aliasInt(7) {
		t.Fatalf("unexpected I: %v", out.I)
	}
	if out.S != aliasString("abc") {
		t.Fatalf("unexpected S: %v", out.S)
	}
	if len(out.Sl) != 3 || out.Sl[2] != 3 {
		t.Fatalf("unexpected Sl: %#v", out.Sl)
	}
	if out.Ar != (aliasArray{4, 5, 6}) {
		t.Fatalf("unexpected Ar: %#v", out.Ar)
	}
	if out.St.Name != "inner" {
		t.Fatalf("unexpected St: %#v", out.St)
	}
}
