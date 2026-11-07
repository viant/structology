package json

import (
	"github.com/viant/x/shape"
	"reflect"
	"testing"
	"time"
)

type timePolicyIdentity struct{}

func (timePolicyIdentity) Transform(_, name string) string { return name }

func TestFormattedTimePointerOutput(t *testing.T) {
	type row struct {
		Stamp *time.Time `json:"stamp,omitempty" format:"timeLayout=2006-01-02T15:04:05Z07:00,timezone=UTC,nullable=true"`
	}
	zero := time.Time{}
	offset := time.Date(2026, 11, 7, 8, 0, 0, 123456789, time.FixedZone("offset", -8*3600))
	for _, tc := range []struct {
		name  string
		value *time.Time
		want  string
	}{
		{"nil", nil, `{}`}, {"zero", &zero, `{"stamp":null}`}, {"offset", &offset, `{"stamp":"2026-11-07T16:00:00Z"}`},
	} {
		for _, mode := range []string{"static", "dynamic", "nested"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var value any = &row{tc.value}
				var opts []Option
				want := tc.want
				if mode == "dynamic" {
					opts = []Option{WithNameTransformer(timePolicyIdentity{})}
				}
				if mode == "nested" {
					value = []row{{tc.value}}
					want = "[" + want + "]"
				}
				got, err := Marshal(value, opts...)
				if err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Fatalf("got %s want %s", got, want)
				}
			})
		}
	}
}

type declaredTimeRow struct {
	Stamp *time.Time `json:"stamp,omitempty" format:"timeLayout=2006-01-02T15:04:05Z07:00,timeInputLayouts=2006-01-02T15:04:05.999999999Z07:00|2006-01-02T15:04:05|2006-01-02 15:04:05|2006-01-02,timeEpoch=milliseconds,timeEmpty=preserve,timezone=UTC,nullable=true"`
}

func TestDeclaredTimeNativeDecoders(t *testing.T) {
	for _, tc := range []struct {
		name, raw, want string
		fail            bool
	}{
		{"millis", `{"stamp":0}`, "1970-01-01T00:00:00Z", false},
		{"max epoch", `{"stamp":9223372036854775807}`, time.UnixMilli(9223372036854775807).UTC().Format(time.RFC3339Nano), false},
		{"negative", `{"stamp":-1}`, "1969-12-31T23:59:59.999Z", false},
		{"empty", `{"stamp":""}`, "0001-01-01T00:00:00Z", false},
		{"date", `{"stamp":"2026-11-07"}`, "2026-11-07T00:00:00Z", false},
		{"offset", `{"stamp":"2026-11-07T08:00:00-08:00"}`, "2026-11-07T16:00:00Z", false},
		{"date empty", `{"stamp":"2026-11-07","stamp":""}`, "2026-11-07T00:00:00Z", false},
		{"date null", `{"stamp":"2026-11-07","stamp":null}`, "nil", false},
		{"null empty", `{"stamp":null,"stamp":""}`, "0001-01-01T00:00:00Z", false},
		{"invalid earlier", `{"stamp":"bad","stamp":"2026-11-07"}`, "", true},
		{"fraction", `{"stamp":1.0}`, "", true}, {"exponent", `{"stamp":1e3}`, "", true},
		{"overflow", `{"stamp":9223372036854775808}`, "", true},
	} {
		for _, mode := range []string{"native", "standard", "nested"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				var row declaredTimeRow
				var err error
				if mode == "standard" {
					err = UnmarshalStandard([]byte(tc.raw), &row)
				} else if mode == "nested" {
					var rows []declaredTimeRow
					err = Unmarshal([]byte("["+tc.raw+"]"), &rows)
					if len(rows) > 0 {
						row = rows[0]
					}
				} else {
					err = Unmarshal([]byte(tc.raw), &row)
				}
				if (err != nil) != tc.fail {
					t.Fatalf("error=%v", err)
				}
				if err != nil {
					return
				}
				got := "nil"
				if row.Stamp != nil {
					got = row.Stamp.Format(time.RFC3339Nano)
				}
				if got != tc.want {
					t.Fatalf("got %s want %s", got, tc.want)
				}
			})
		}
	}
}

func TestDeclaredTimeNestedMaps(t *testing.T) {
	type envelope struct {
		Rows []declaredTimeRow        `json:"rows"`
		ByID map[int]*declaredTimeRow `json:"byId"`
	}
	for _, standard := range []bool{false, true} {
		for _, bad := range []bool{false, true} {
			stamp := `"2026-11-07"`
			if bad {
				stamp = `"bad"`
			}
			raw := []byte(`{"rows":[{"stamp":0}],"byId":{"7":{"STAMP":` + stamp + `,"stamp":""}}}`)
			var value envelope
			var err error
			if standard {
				err = UnmarshalStandard(raw, &value)
			} else {
				err = Unmarshal(raw, &value)
			}
			if (err != nil) != bad {
				t.Fatalf("standard=%v bad=%v error=%v", standard, bad, err)
			}
			if err == nil && (value.ByID[7].Stamp == nil || value.ByID[7].Stamp.Format(time.RFC3339) != "2026-11-07T00:00:00Z") {
				t.Fatalf("standard=%v value=%+v", standard, value.ByID)
			}
		}
	}
}

func TestDeclaredTimeCollectionDoesNotLoseMalformedJSON(t *testing.T) {
	for _, raw := range []string{`[{"stamp":0}] trailing`, `[{"stamp":0},]`, `{"x":{"stamp":0},"x":{"stamp":1}}`} {
		if raw[0] == '[' {
			var value []declaredTimeRow
			if err := Unmarshal([]byte(raw), &value); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		} else {
			var value map[string]declaredTimeRow
			if err := Unmarshal([]byte(raw), &value, WithDuplicateKeyPolicy(ErrorOnDuplicate)); err == nil {
				t.Fatalf("accepted duplicate %s", raw)
			}
		}
	}
}

func TestDeclaredTimeRepresentationInCollection(t *testing.T) {
	type row struct {
		Stamp *time.Time `json:"stamp"`
	}
	type envelope struct {
		Rows []row `json:"rows"`
	}
	walker, err := shape.CompileRewrite(reflect.TypeFor[envelope]())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, field *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if field.Name == "Stamp" {
			return &shape.FieldMetadata{Name: field.Name, Tag: `json:"stamp" format:"timeInputLayouts=2006-01-02,timeEpoch=milliseconds,timeEmpty=preserve"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, standard := range []bool{false, true} {
		var value envelope
		raw := []byte(`{"rows":[{"stamp":0}]}`)
		if standard {
			err = UnmarshalStandard(raw, &value, WithRepresentation(metadata))
		} else {
			err = Unmarshal(raw, &value, WithRepresentation(metadata))
		}
		if err != nil {
			t.Fatalf("standard=%v: %v", standard, err)
		}
		if len(value.Rows) != 1 || value.Rows[0].Stamp == nil || !value.Rows[0].Stamp.Equal(time.UnixMilli(0)) {
			t.Fatalf("standard=%v: %+v", standard, value)
		}
	}
}
