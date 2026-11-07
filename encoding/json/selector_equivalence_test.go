package json

import (
	"reflect"
	"testing"

	jsonunmarshal "github.com/viant/structology/encoding/json/unmarshal"
	"github.com/viant/tagly/format/text"
)

func TestSelectorOptionsRetainExplicitNativePolicies(t *testing.T) {
	for name, option := range map[string]Option{
		"unknown":           WithUnknownFieldPolicy(ErrorOnUnknown),
		"duplicates":        WithDuplicateKeyPolicy(ErrorOnDuplicate),
		"null":              WithNullPolicy(StrictNulls),
		"numbers":           WithNumberPolicy(ExactNumbers),
		"malformed":         WithMalformedPolicy(FailFast),
		"explicit tolerant": WithMalformedPolicy(Tolerant),
		"case":              WithCaseFormat(text.CaseFormatLowerUnderscore),
		"mode":              WithMode(ModeStrict),
		"path":              WithPathTracking(PathTrackingFull),
	} {
		t.Run(name, func(t *testing.T) {
			if standardSelectorOptions(resolveOptions(nil, []Option{option})) {
				t.Fatal("explicit native policy bypassed")
			}
		})
	}
	type selector struct {
		Metric bool `json:"metric" format:"inputNames=metric_alias"`
	}
	for _, tc := range []struct {
		option           Option
		canonical, alias string
	}{
		{WithUnknownFieldPolicy(ErrorOnUnknown), `{"metric":true,"unknown":true}`, `{"metric_alias":true,"unknown":true}`},
		{WithDuplicateKeyPolicy(ErrorOnDuplicate), `{"metric":true,"metric":false}`, `{"metric_alias":true,"metric_alias":false}`},
		{WithNullPolicy(StrictNulls), `{"metric":null}`, `{"metric_alias":null}`},
	} {
		var canonical, alias selector
		left := Unmarshal([]byte(tc.canonical), &canonical, tc.option)
		right := Unmarshal([]byte(tc.alias), &alias, tc.option)
		if (left == nil) != (right == nil) || reflect.TypeOf(left) != reflect.TypeOf(right) || canonical != alias {
			t.Fatalf("native policy differs: canonical=%+v/%T alias=%+v/%T", canonical, left, alias, right)
		}
	}
}

func TestSelectorEquivalenceExcludesUnprovenGraphs(t *testing.T) {
	for _, value := range []any{
		&struct {
			First  bool `json:"same" format:"inputNames=first_alias"`
			Second bool `json:"same"`
		}{},
		&struct {
			Flag bool `format:"inputNames=flag_alias" enumAccept:"scalar"`
		}{},
		&struct {
			Flag  bool `format:"inputNames=flag_alias"`
			Count int
		}{},
		&struct {
			Flag bool `format:"inputNames=flag_alias,name=renamed"`
		}{},
		&struct {
			Flag bool                 `format:"inputNames=flag_alias"`
			Has  *struct{ Flag bool } `json:"-" setMarker:"true"`
		}{},
		&struct {
			Nested struct {
				Flag bool `format:"inputNames=flag_alias"`
			}
		}{},
	} {
		if jsonunmarshal.StandardBoolSelector(reflect.TypeOf(value), nil) {
			t.Fatalf("unproven graph admitted: %T", value)
		}
	}
}
