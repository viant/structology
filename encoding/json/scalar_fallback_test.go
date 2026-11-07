package json

import (
	stdjson "encoding/json"
	"testing"
)

type fallbackName string
type fallbackRow struct {
	Mode *fallbackName        `json:"mode" choice:"0:ASAP" choice:"1:SPEND_EVENLY" enumAccept:"scalar" enumStyle:"literal" enumCase:"fold" enumUnknownString:"0" enumUnknownInteger:"0"`
	Has  *struct{ Mode bool } `json:"-" setMarker:"true"`
}

func TestScalarFallbackNativeAndStandard(t *testing.T) {
	for _, standard := range []bool{false, true} {
		decode := func(raw []byte, v any) error {
			if standard {
				return UnmarshalStandard(raw, v, WithPresenceTracking(true))
			}
			return Unmarshal(raw, v, WithPresenceTracking(true))
		}
		for _, tc := range []struct {
			raw, want string
			present   bool
		}{
			{`{}`, "", false}, {`{"mode":null}`, "", true}, {`{"mode":1}`, "SPEND_EVENLY", true},
			{`{"mode":99}`, "ASAP", true}, {`{"mode":"Even"}`, "ASAP", true}, {`{"mode":"1"}`, "ASAP", true},
			{`{"mode":"ſPEND_EVENLY"}`, "SPEND_EVENLY", true}, {`{"mode":9223372036854775807}`, "ASAP", true},
			{`{"mode":-9223372036854775808}`, "ASAP", true},
		} {
			var envelope struct {
				Rows []*fallbackRow `json:"rows"`
			}
			if err := decode([]byte(`{"rows":[`+tc.raw+`,null]}`), &envelope); err != nil {
				t.Fatalf("standard=%v %s: %v", standard, tc.raw, err)
			}
			row := envelope.Rows[0]
			if envelope.Rows[1] != nil || (row.Mode != nil) != (tc.want != "") || row.Mode != nil && string(*row.Mode) != tc.want || (row.Has != nil && row.Has.Mode) != tc.present {
				t.Fatalf("standard=%v %s => %+v", standard, tc.raw, row)
			}
			var encoded []byte
			var err error
			if standard {
				encoded, err = MarshalStandard(row)
			} else {
				encoded, err = Marshal(row)
			}
			var wire struct{ Mode *string }
			if err != nil {
				t.Fatal(err)
			}
			if err = stdjson.Unmarshal(encoded, &wire); err != nil || (wire.Mode != nil) != (tc.want != "") || wire.Mode != nil && *wire.Mode != tc.want {
				t.Fatalf("wire=%s err=%v", encoded, err)
			}
		}
		for _, raw := range []string{`9223372036854775808`, `-9223372036854775809`, `1.0`, `1e0`, `true`, `[]`, `{}`} {
			mode := fallbackName("ASAP")
			row := fallbackRow{Mode: &mode}
			if err := decode([]byte(`{"mode":`+raw+`}`), &row); err == nil {
				t.Fatalf("accepted %s", raw)
			}
			if row.Mode == nil || *row.Mode != "ASAP" {
				t.Fatalf("failed conversion mutated %s", raw)
			}
		}
	}
}
