package json

import (
	"encoding/json"
	"testing"
)

type openMode int64
type openModeRow struct {
	Mode  *openMode                   `json:"mode" choice:"1:FIXED_BUDGET" choice:"2:AUTO_BUDGET" enumAccept:"scalar" enumStyle:"ordinal" enumInteger:"passthrough" enumUnknownString:"0" enumCase:"upper"`
	Value openMode                    `json:"value" choice:"1:FIXED_BUDGET" choice:"2:AUTO_BUDGET" enumAccept:"scalar" enumStyle:"ordinal" enumInteger:"passthrough" enumUnknownString:"0" enumCase:"upper"`
	Has   *struct{ Mode, Value bool } `json:"-" setMarker:"true"`
}

func TestOpenScalarNativeAndStandard(t *testing.T) {
	for _, standard := range []bool{false, true} {
		decode := func(data []byte, v any) error {
			if standard {
				return UnmarshalStandard(data, v, WithPresenceTracking(true))
			}
			return Unmarshal(data, v, WithPresenceTracking(true))
		}
		for _, tc := range []struct {
			raw  string
			want openMode
		}{{`0`, 0}, {`3`, 3}, {`-1`, -1}, {`9223372036854775807`, 9223372036854775807}, {`-9223372036854775808`, -9223372036854775808}, {`"fıxed_budget"`, 1}, {`"AUTO_BUDGET"`, 2}, {`"other"`, 0}, {`"1"`, 0}, {`""`, 0}} {
			var value struct {
				Rows  []openModeRow          `json:"rows"`
				Named map[string]openModeRow `json:"named"`
			}
			data := []byte(`{"rows":[{"mode":` + tc.raw + `}],"named":{"a":{"mode":` + tc.raw + `}}}`)
			if err := decode(data, &value); err != nil {
				t.Fatalf("standard=%v raw=%s err=%v", standard, tc.raw, err)
			}
			for _, row := range []openModeRow{value.Rows[0], value.Named["a"]} {
				if row.Mode == nil || *row.Mode != tc.want || row.Has == nil || !row.Has.Mode {
					t.Fatalf("row=%+v", row)
				}
			}
			encoded, err := Marshal(value.Rows[0])
			if err != nil {
				t.Fatal(err)
			}
			var wire struct{ Mode int64 }
			if err = json.Unmarshal(encoded, &wire); err != nil || wire.Mode != int64(tc.want) {
				t.Fatalf("output=%s err=%v", encoded, err)
			}
		}
		n := openMode(7)
		row := openModeRow{Mode: &n, Value: 7}
		if err := decode([]byte(`{"mode":null,"value":null}`), &row); err != nil {
			t.Fatal(err)
		}
		if row.Mode != nil || row.Value != 7 || row.Has == nil || !row.Has.Mode || !row.Has.Value {
			t.Fatalf("null standard=%v row=%+v", standard, row)
		}
		for _, raw := range []string{`true`, `1.0`, `1e0`, `[]`, `{}`, `9223372036854775808`} {
			n := openMode(7)
			row := openModeRow{Mode: &n}
			if err := decode([]byte(`{"mode":`+raw+`}`), &row); err == nil {
				t.Fatalf("accepted %s", raw)
			}
			if row.Mode == nil || *row.Mode != 7 {
				t.Fatalf("failed conversion changed value for%s", raw)
			}
		}
	}
}
