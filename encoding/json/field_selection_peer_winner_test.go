package json_test

import (
	structjson "github.com/viant/structology/encoding/json"
	"github.com/viant/tagly/format/text"
	"reflect"
	"testing"
)

func TestLowerCamelCanonicalSelectionDisambiguatesPeerNames(t *testing.T) {
	type row struct {
		SampleSeenDay  int
		SampleSeen_Day int
	}
	value := &row{11, 77}
	opts := []structjson.Option{structjson.WithCaseFormat(text.CaseFormatLowerCamel)}
	for _, tc := range []struct{ name, want string }{{"SampleSeenDay", `{"sampleSeenDay":11}`}, {"SampleSeen_Day", `{"sampleSeenDay":77}`}} {
		filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Fields: []string{tc.name}}})
		if err != nil {
			t.Fatal(err)
		}
		projected, err := filter.WithOptions(opts...)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := structjson.Marshal(value, append(opts, structjson.WithPathFieldExcluder(projected))...)
		if err != nil || string(raw) != tc.want {
			t.Fatalf("%s: %s %v", tc.name, raw, err)
		}
	}
	if _, err := structjson.Marshal(value, opts...); err == nil {
		t.Fatal("unprojected ambiguous names accepted")
	}
}
