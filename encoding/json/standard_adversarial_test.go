package json_test

import (
	"encoding/json"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	structjson "github.com/viant/structology/encoding/json"
)

type dualSelectionCodec int

func (dualSelectionCodec) MarshalText() ([]byte, error)  { return []byte("text"), nil }
func (*dualSelectionCodec) MarshalJSON() ([]byte, error) { return []byte(`{"json":true}`), nil }

type adversarialSelectionRow struct {
	ID     int                `json:"id"`
	Text   string             `json:"text,string"`
	Float  float64            `json:"float"`
	Maybe  *int               `json:"maybe"`
	Bytes  []byte             `json:"bytes"`
	Values []int              `json:"values"`
	Raw    json.RawMessage    `json:"raw"`
	When   time.Time          `json:"when"`
	Dual   dualSelectionCodec `json:"dual,string"`
	Any    any                `json:"any"`
	Empty  [2]int             `json:"empty,omitempty"`
}

func TestStandardSelectionAdversarialDifferential(t *testing.T) {
	rng := rand.New(rand.NewSource(7419))
	texts := []string{"", "<>&\u2028\u2029", string([]byte{0xff, 0, 0xc0}), "quote\"slash\\\n\t", "日本語"}
	identity := selectionExcluder(func([]string, string) bool { return false })
	for i := 0; i < 512; i++ {
		number := rng.Intn(200) - 100
		value := adversarialSelectionRow{ID: number, Text: texts[i%len(texts)], Float: rng.NormFloat64(), Bytes: []byte{0, 1, 255}, Raw: json.RawMessage(` { "x": "<>&" } `), When: time.Unix(int64(number), 999).UTC(), Values: []int{number, 0}}
		if i%3 == 0 {
			value.Maybe = &number
			value.Any = map[string]any{"x": number, "nil": nil}
		} else if i%3 == 1 {
			value.Any = (*int)(nil)
			value.Bytes = nil
			value.Values = nil
		} else {
			value.Any = []any{value.Text, float64(number)}
		}
		if i%17 == 0 {
			value.Float = math.Inf(1)
		}
		for _, input := range []any{value, &value, []adversarialSelectionRow{value}} {
			want, werr := json.Marshal(input)
			got, gerr := structjson.MarshalStandard(input, structjson.WithPathFieldExcluder(identity))
			require.Equal(t, werr == nil, gerr == nil, "case %d %T", i, input)
			if werr == nil {
				require.Equal(t, string(want), string(got), "case %d %T", i, input)
			}
			plain, perr := structjson.MarshalStandard(input)
			require.Equal(t, werr == nil, perr == nil)
			if werr == nil {
				require.Equal(t, string(want), string(plain))
			}
		}
	}
}

type selectionBenchRow struct {
	ID     int     `json:"id"`
	Name   string  `json:"name"`
	Secret string  `json:"secret"`
	Zero   int     `json:"zero"`
	Maybe  *string `json:"maybe"`
}

func BenchmarkStandardSelection(b *testing.B) {
	rows := make([]selectionBenchRow, 100)
	for i := range rows {
		rows[i] = selectionBenchRow{ID: i, Name: "visible <value>", Secret: "hidden"}
	}
	identity := selectionExcluder(func([]string, string) bool { return false })
	narrow, err := structjson.NewFieldFilter(reflect.TypeOf(rows), []structjson.FieldSelection{{Fields: []string{"ID", "Name", "Zero", "Maybe"}}})
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		encode func() ([]byte, error)
	}{
		{"encoding_json", func() ([]byte, error) { return json.Marshal(rows) }},
		{"standard_no_mask", func() ([]byte, error) { return structjson.MarshalStandard(rows) }},
		{"identity_mask", func() ([]byte, error) {
			return structjson.MarshalStandard(rows, structjson.WithPathFieldExcluder(identity))
		}},
		{"narrow_mask", func() ([]byte, error) {
			return structjson.MarshalStandard(rows, structjson.WithPathFieldExcluder(narrow))
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := tc.encode(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
