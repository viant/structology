package json_test

import (
	structjson "github.com/viant/structology/encoding/json"
	"github.com/viant/tagly/format/text"
	"testing"
)

type DecodeWinnerEmbedded struct{ Id int }
type decodeWinnerOuter struct {
	*DecodeWinnerEmbedded
	Id int `json:"id"`
}

func TestLowerCamelEmbeddedDecodeWinner(t *testing.T) {
	for _, key := range []string{"id", "Id", "ID"} {
		value := decodeWinnerOuter{&DecodeWinnerEmbedded{7}, 0}
		if err := structjson.Unmarshal([]byte(`{"`+key+`":42}`), &value, structjson.WithCaseFormat(text.CaseFormatLowerCamel)); err != nil {
			t.Fatal(err)
		}
		if value.Id != 42 || value.DecodeWinnerEmbedded.Id != 7 {
			t.Fatalf("%s chose wrong field: %+v embedded=%+v", key, value, value.DecodeWinnerEmbedded)
		}
	}
	var value decodeWinnerOuter
	if err := structjson.Unmarshal([]byte(`{"id":42}`), &value, structjson.WithCaseFormat(text.CaseFormatLowerCamel)); err != nil {
		t.Fatal(err)
	}
	if value.Id != 42 || value.DecodeWinnerEmbedded != nil {
		t.Fatalf("shadowed pointer allocated: %+v", value)
	}
}

type DecodeWinnerTagged struct {
	Other int `json:"id"`
}
type DecodeWinnerSecond struct{ ID int }

func TestLowerCamelDecodeWinnerNestedAndAmbiguity(t *testing.T) {
	type peers struct {
		DecodeWinnerEmbedded
		DecodeWinnerTagged
	}
	var peer peers
	if err := structjson.Unmarshal([]byte(`{"id":42}`), &peer, structjson.WithCaseFormat(text.CaseFormatLowerCamel)); err != nil {
		t.Fatal(err)
	}
	if peer.Other != 42 || peer.Id != 0 {
		t.Fatalf("tag winner: %+v", peer)
	}
	type ambiguous struct {
		DecodeWinnerEmbedded
		DecodeWinnerSecond
	}
	var bad ambiguous
	if err := structjson.Unmarshal([]byte(`{"id":42}`), &bad, structjson.WithCaseFormat(text.CaseFormatLowerCamel)); err == nil {
		t.Fatal("same-depth ambiguity accepted")
	}
	var nested struct {
		Rows []*decodeWinnerOuter `json:"rows"`
		Item *decodeWinnerOuter   `json:"item"`
	}
	if err := structjson.Unmarshal([]byte(`{"rows":[{"id":42},null],"item":null}`), &nested, structjson.WithCaseFormat(text.CaseFormatLowerCamel)); err != nil {
		t.Fatal(err)
	}
	if len(nested.Rows) != 2 || nested.Rows[0].Id != 42 || nested.Rows[0].DecodeWinnerEmbedded != nil || nested.Rows[1] != nil || nested.Item != nil {
		t.Fatalf("nested pointers/null: %+v", nested)
	}
}
