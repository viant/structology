package json

import (
	stdjson "encoding/json"
	"fmt"
	"github.com/viant/x/shape"
	"os"
	"reflect"
	"testing"
)

// Captured from Datly report.UnmarshalSelectorJSON at ce10b695b292215e525245383b90b981fa14a01c.
// This fixture records the removed-method contract independently of this decoder.
func TestInputNamesOriginalSelectorOracle(t *testing.T) {
	type selector struct {
		Metric25   bool `json:"metric25,omitempty" format:"inputNames=metric_25"`
		OtherValue bool `json:"otherValue,omitempty" format:"inputNames=other_value"`
	}
	var rows []struct {
		Case      int
		Reused    bool
		Input     string
		Value     selector
		ErrorType string
	}
	data, err := os.ReadFile("testdata/input_names_oracle.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = stdjson.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 62 {
		t.Fatalf("oracle inventory changed: %d", len(rows))
	}
	for name, decode := range map[string]func([]byte, any) error{
		"standard": func(b []byte, v any) error { return UnmarshalStandard(b, v) },
		"native":   func(b []byte, v any) error { return Unmarshal(b, v) },
	} {
		for _, row := range rows {
			t.Run(fmt.Sprintf("%s/%02d/reused=%t", name, row.Case, row.Reused), func(t *testing.T) {
				got := selector{Metric25: row.Reused, OtherValue: row.Reused}
				err := decode([]byte(row.Input), &got)
				kind := ""
				if err != nil {
					kind = fmt.Sprintf("%T", err)
				}
				if got != row.Value || kind != row.ErrorType {
					t.Errorf("%s: got %+v error=%T; want %+v error=%s", row.Input, got, err, row.Value, row.ErrorType)
				}
			})
		}
	}
}

type inputAliasRow struct {
	Metric25 bool `json:"metric25,omitempty" format:"inputNames=metric_25"`
	Other    bool `json:"other,omitempty"`
	Hidden   bool `json:"-" format:"inputNames=hidden_alias"`
}

func TestNativeInputNamesExactAndDuplicateOrder(t *testing.T) {
	for _, tc := range []struct {
		body string
		want bool
	}{
		{`{"metric_25":true}`, true},
		{`{"metric25":true,"metric_25":false}`, false},
		{`{"metric_25":false,"metric25":true}`, true},
		{`{"METRIC_25":true,"hidden_alias":true}`, false},
	} {
		var got inputAliasRow
		if err := Unmarshal([]byte(tc.body), &got); err != nil {
			t.Fatal(err)
		}
		if got.Metric25 != tc.want || got.Hidden {
			t.Errorf("%s: got %+v", tc.body, got)
		}
	}
}

func TestStandardInputNamesOrderedNullAndErrorParity(t *testing.T) {
	for _, tc := range []struct {
		body          string
		initial, want inputAliasRow
		typeError     bool
	}{
		{body: `{"metric_25":true}`, want: inputAliasRow{Metric25: true}},
		{body: `{"metric25":true,"metric_25":false}`},
		{body: `{"metric_25":false,"metric25":true}`, want: inputAliasRow{Metric25: true}},
		{body: `{"metric_25":null}`, initial: inputAliasRow{Metric25: true}, want: inputAliasRow{Metric25: true}},
		{body: `{"METRIC_25":true,"hidden_alias":true}`},
		{body: `{"METRIC25":true}`, want: inputAliasRow{Metric25: true}},
		{body: `{"metric_25":"wrong","other":true}`, want: inputAliasRow{Other: true}, typeError: true},
	} {
		got := tc.initial
		err := UnmarshalStandard([]byte(tc.body), &got)
		if got != tc.want {
			t.Errorf("%s: got %+v want %+v", tc.body, got, tc.want)
		}
		if tc.typeError {
			if e, ok := err.(*stdjson.UnmarshalTypeError); !ok || e.Field != "metric25" {
				t.Errorf("wrong type error: %T %v", err, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", tc.body, err)
		}
	}
}

func TestStandardInputNamesNestedAndRepresentation(t *testing.T) {
	type envelope struct {
		Rows   []*inputAliasRow         `json:"rows"`
		ByName map[string]inputAliasRow `json:"byName"`
	}
	plan, err := shape.CompileRewrite(reflect.TypeFor[envelope]())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := plan.RewriteRepresentation(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, options := range [][]Option{nil, {WithRepresentation(metadata)}} {
		var got envelope
		err := UnmarshalStandard([]byte(`{"rows":[{"metric_25":true}],"byName":{"a":{"metric_25":true}}}`), &got, options...)
		if err != nil {
			t.Fatal(err)
		}
		if len(got.Rows) != 1 || !got.Rows[0].Metric25 || !got.ByName["a"].Metric25 {
			t.Errorf("nested alias not decoded: %+v", got)
		}
	}
}

func TestStandardInputNamesSyntaxBeforeMutationAndCollision(t *testing.T) {
	row := inputAliasRow{}
	if err := UnmarshalStandard([]byte(`{"metric_25":true,`), &row); err == nil || row.Metric25 {
		t.Fatalf("invalid JSON mutated target: %+v %v", row, err)
	}
	var collision struct {
		One bool `json:"one" format:"inputNames=two"`
		Two bool `json:"two"`
	}
	if err := UnmarshalStandard([]byte(`{"two":true}`), &collision); err == nil {
		t.Fatal("ambiguous alias accepted")
	}
}

func TestStandardInputNamesRepresentationIsolationAndPresence(t *testing.T) {
	type row struct {
		Value  bool                          `json:"value" format:"inputNames=old_value"`
		Secret bool                          `json:"secret" format:"inputNames=secret_alias"`
		Has    *struct{ Value, Secret bool } `json:"-" setMarker:"true"`
	}
	walker, err := shape.CompileRewrite(reflect.TypeFor[row]())
	if err != nil {
		t.Fatal(err)
	}
	for _, alias := range []string{"first_value", "second_value"} {
		metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
			switch f.Name {
			case "Value":
				return &shape.FieldMetadata{Name: f.Name, Tag: reflect.StructTag(`json:"visible" format:"inputNames=` + alias + `"`)}, nil
			case "Secret":
				return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-" format:"inputNames=secret_alias"`}, nil
			}
			return nil, nil
		})
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range []string{`{"` + alias + `":false,"secret_alias":true}`, `{"` + alias + `":null,"secret":true}`} {
			var got row
			if err := UnmarshalStandard([]byte(input), &got, WithRepresentation(metadata), WithPresenceTracking(true)); err != nil {
				t.Fatal(err)
			}
			if got.Value || got.Secret || got.Has == nil || !got.Has.Value || got.Has.Secret {
				t.Errorf("%s: %+v markers=%+v", input, got, got.Has)
			}
		}
		var ignored row
		if err := UnmarshalStandard([]byte(`{"old_value":true,"value":true}`), &ignored, WithRepresentation(metadata), WithPresenceTracking(true)); err != nil {
			t.Fatal(err)
		}
		if ignored.Value || ignored.Has != nil {
			t.Errorf("stale representation names accepted: %+v", ignored)
		}
	}
	var original row
	if err := UnmarshalStandard([]byte(`{"old_value":true}`), &original, WithPresenceTracking(true)); err != nil {
		t.Fatal(err)
	}
	if !original.Value || original.Has == nil || !original.Has.Value {
		t.Errorf("representation leaked to original: %+v", original)
	}
}
