package json

import (
	"testing"

	"github.com/viant/tagly/format/text"
)

func TestCaseFormatPrecompute_Marshal(t *testing.T) {
	type sample struct {
		UserName string
		Nick     string `json:"nickName"`
	}
	in := sample{UserName: "alice", Nick: "a"}
	data, err := Marshal(in, WithCaseFormat(text.CaseFormatLowerUnderscore))
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	got := string(data)
	if got != `{"user_name":"alice","nickName":"a"}` {
		t.Fatalf("unexpected output: %s", got)
	}
}

func TestCaseFormatPrecompute_Unmarshal(t *testing.T) {
	type sample struct {
		UserName string
		Nick     string `json:"nickName"`
	}
	var out sample
	err := Unmarshal([]byte(`{"user_name":"alice","nickName":"a"}`), &out, WithCaseFormat(text.CaseFormatLowerUnderscore))
	if err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if out.UserName != "alice" || out.Nick != "a" {
		t.Fatalf("unexpected value: %#v", out)
	}
}
