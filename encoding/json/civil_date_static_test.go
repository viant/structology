package json

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/viant/tagly/format/text"
)

type CivilStaticRow struct {
	Day   *time.Time `json:"day" format:"dateFormat=YYYY-MM-DD"`
	Value time.Time  `json:"value" format:"timeLayout=2006-01-02"`
	Raw   *time.Time `json:"raw"`
}
type civilStaticInline struct{ CivilStaticRow }
type civilStaticEnvelope struct {
	Rows   []*CivilStaticRow   `json:"rows"`
	Values []CivilStaticRow    `json:"values"`
	Inline []civilStaticInline `json:"inline"`
}

func TestPrecompiledNestedCivilDatePreservesFieldLayout(t *testing.T) {
	for _, zone := range []struct {
		name   string
		offset int
	}{{"west", -7 * 3600}, {"east", 14 * 3600}} {
		t.Run(zone.name, func(t *testing.T) {
			date := time.Date(2026, 9, 1, 23, 30, 0, 0, time.FixedZone(zone.name, zone.offset))
			row := CivilStaticRow{Day: &date, Value: date, Raw: &date}
			input := civilStaticEnvelope{Rows: []*CivilStaticRow{&row, {}}, Values: []CivilStaticRow{row, {}}, Inline: []civilStaticInline{{row}, {}}}
			encoder, e := NewMarshaller(reflect.TypeFor[civilStaticEnvelope](), WithCaseFormat(text.CaseFormatLowerCamel))
			if e != nil {
				t.Fatal(e)
			}
			for _, value := range []any{input, &input} {
				raw, e := encoder.Marshal(value)
				if e != nil {
					t.Fatal(e)
				}
				var output map[string][]map[string]any
				if e = json.Unmarshal(raw, &output); e != nil {
					t.Fatal(e)
				}
				for _, name := range []string{"rows", "values", "inline"} {
					rows := output[name]
					if len(rows) != 2 || rows[0]["day"] != "2026-09-01" || rows[0]["value"] != "2026-09-01" || rows[0]["raw"] != date.Format(time.RFC3339) || rows[1]["day"] != nil || rows[1]["raw"] != nil || rows[1]["value"] != "0001-01-01" {
						t.Fatalf("%s: field layout/null/raw mismatch: %s", name, raw)
					}
				}
			}
		})
	}
}
