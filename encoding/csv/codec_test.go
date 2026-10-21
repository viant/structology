package csv

import (
	"bytes"
	stdcsv "encoding/csv"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStandardEscapingRoundTrip(t *testing.T) {
	type row struct {
		ID    int    `csvName:"id"`
		Value string `csvName:"value"`
	}
	values := []string{`plain`, `a,b`, `say "hello"`, `C:\folder\file`, "first\nsecond", "first\r\nsecond", "", "日本語 ☕", `a,"b"\c`}
	input := make([]row, len(values))
	for i, value := range values {
		input[i] = row{i, value}
	}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	records, err := stdcsv.NewReader(bytes.NewReader(encoded)).ReadAll()
	if err != nil {
		t.Fatalf("%v\n%s", err, encoded)
	}
	if !reflect.DeepEqual(records[0], []string{"id", "value"}) {
		t.Fatal(records[0])
	}
	for i, value := range values {
		expected := strings.ReplaceAll(value, "\r\n", "\n")
		if records[i+1][1] != expected {
			t.Fatalf("row %d: %q != %q", i, records[i+1][1], expected)
		}
	}
	var decoded []row
	if err = Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	for i := range input {
		input[i].Value = strings.ReplaceAll(input[i].Value, "\r\n", "\n")
	}
	if !reflect.DeepEqual(input, decoded) {
		t.Fatalf("round trip: %#v", decoded)
	}
}
func TestNullZeroAndLiteralNull(t *testing.T) {
	type row struct {
		ID    int
		Text  *string
		Count *int
		Flag  bool
		Date  *time.Time
	}
	literal := "null"
	empty := ""
	zero := 0
	now := time.Date(2026, 10, 21, 10, 20, 30, 0, time.UTC)
	input := []row{{ID: 1}, {ID: 2, Text: &literal, Count: &zero, Date: &now}, {ID: 3, Text: &empty}}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output []row
	if err = Unmarshal(encoded, &output); err != nil {
		t.Fatalf("%v: %s", err, encoded)
	}
	if !reflect.DeepEqual(input, output) {
		t.Fatalf("null/zero mismatch: %#v", output)
	}
}
func TestAliasesArraysNilAndHiddenFields(t *testing.T) {
	type number int64
	type text string
	type row struct {
		Number  number
		Text    text
		Ignored string                 `csvName:"-"`
		Has     *struct{ Number bool } `setMarker:"true"`
		private string
	}
	input := [2]row{{Number: 3, Text: "three"}, {Number: 4, Text: "four"}}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output []row
	if err = Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if len(output) != 2 || output[0].Number != 3 || output[1].Text != "four" {
		t.Fatal(output)
	}
	for _, value := range []any{[]row{}, (*row)(nil), []*row{nil}} {
		empty, err := Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if string(empty) != "\"Number\",\"Text\"" {
			t.Fatalf("empty CSV: %q", empty)
		}
	}
	encoded, err = Marshal([]*row{nil, {Number: 5, Text: "five"}})
	if err != nil {
		t.Fatal(err)
	}
	var pointers []*row
	if err = Unmarshal(encoded, &pointers); err != nil {
		t.Fatal(err)
	}
	if len(pointers) != 1 || pointers[0].Number != 5 {
		t.Fatal(pointers)
	}
}
func TestConfigSnapshotAndDelimiter(t *testing.T) {
	type row struct {
		A string
		B int
	}
	config := &Config{FieldSeparator: ";", ExcludedPaths: []string{"B"}}
	m, err := NewMarshaller(reflect.TypeFor[row](), config)
	if err != nil {
		t.Fatal(err)
	}
	if config.EncloseBy != "" || config.EscapeBy != "" {
		t.Fatal("constructor mutated config")
	}
	config.FieldSeparator = ","
	config.ExcludedPaths[0] = "A"
	encoded, err := m.Marshal([]row{{A: "a;\"b\"", B: 1}})
	if err != nil {
		t.Fatal(err)
	}
	var output []row
	if err = m.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	if len(output) != 1 || output[0].A != "a;\"b\"" || output[0].B != 0 {
		t.Fatal(output)
	}
}
func TestNamedNestedFields(t *testing.T) {
	type child struct {
		ID    int    `csvName:"id"`
		Value string `csvName:"value"`
	}
	type row struct {
		ID    int   `csvName:"id"`
		Child child `csvName:"child"`
	}
	input := []row{{1, child{2, `a,"b"`}}}
	encoded, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output []row
	if err = Unmarshal(encoded, &output); err != nil {
		t.Fatalf("%v: %s", err, encoded)
	}
	if !reflect.DeepEqual(input, output) {
		t.Fatalf("%#v", output)
	}
}
func TestInvalidInputs(t *testing.T) {
	type duplicate struct {
		A string `csvName:"same"`
		B string `csvName:"same"`
	}
	type recursive struct {
		ID       int
		Children []*recursive
	}
	for _, typ := range []reflect.Type{nil, reflect.TypeFor[int](), reflect.TypeFor[duplicate](), reflect.TypeFor[recursive]()} {
		if _, err := NewMarshaller(typ, nil); err == nil {
			t.Fatalf("accepted %v", typ)
		}
	}
	type row struct{ A int8 }
	m, err := NewMarshaller(reflect.TypeFor[row](), nil)
	if err != nil {
		t.Fatal(err)
	}
	var output []row
	for _, data := range []string{"A\n999", "A,A\n1,2", "missing\n1", "A\n\"unterminated", "A\n1,2"} {
		if err = m.Unmarshal([]byte(data), &output); err == nil {
			t.Fatalf("accepted %q", data)
		}
	}
	if err = m.Unmarshal([]byte("A\n1"), output); err == nil {
		t.Fatal("accepted non-pointer destination")
	}
	if _, err = NewMarshaller(reflect.TypeFor[row](), &Config{FieldSeparator: "::"}); err == nil {
		t.Fatal("accepted invalid delimiter")
	}
}
func TestConcurrentCompiledMarshaller(t *testing.T) {
	type row struct {
		ID    int
		Value string
	}
	m, err := NewMarshaller(reflect.TypeFor[row](), nil)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func(id int) {
			defer group.Done()
			input := []row{{id, `quote " comma, slash\`}}
			for n := 0; n < 10; n++ {
				data, err := m.Marshal(input)
				if err != nil {
					t.Error(err)
					return
				}
				var output []row
				if err = m.Unmarshal(data, &output); err != nil || !reflect.DeepEqual(input, output) {
					t.Errorf("concurrent roundtrip: %v %#v", err, output)
					return
				}
			}
		}(i)
	}
	group.Wait()
}
func TestStreamingReaderEscaping(t *testing.T) {
	type row struct{ Value string }
	reader, _, err := NewReader([]row{{`a,"b"\c`}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	records, err := stdcsv.NewReader(bytes.NewReader(data)).ReadAll()
	if err != nil || len(records) != 1 || records[0][0] != `a,"b"\c` {
		t.Fatalf("stream: %v %q", err, data)
	}
}

func TestNestedSlicesDecodeWithoutDuplicateChildren(t *testing.T) {
	type leaf struct{ Value string }
	type child struct {
		ID     int
		Leaves []leaf
	}
	type parent struct {
		ID       int
		Children []child
	}
	input := []parent{{1, []child{{2, []leaf{{"x"}}}}}}
	data, err := Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var output []parent
	if err = Unmarshal(data, &output); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, output) {
		t.Fatalf("nested decode: %#v", output)
	}
}
func TestUnsupportedShapesFailWithoutUnsafeAccess(t *testing.T) {
	type row struct{ ID int }
	for _, value := range []any{[][]row{{{1}}}, []**row{}, map[string]int{"a": 1}, 42} {
		if _, err := Marshal(value); err == nil {
			t.Fatalf("accepted %T", value)
		}
	}
	if _, err := NewMarshaller(reflect.TypeFor[row](), &Config{UniqueFields: []string{"ID"}}); err == nil {
		t.Fatal("unsupported grouping accepted")
	}
}
