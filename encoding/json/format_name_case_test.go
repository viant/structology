package json

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/viant/tagly/format"
	"github.com/viant/tagly/format/text"
)

func TestFormatNameCasePolicy(t *testing.T) {
	for _, tc := range []struct {
		name, tag, unchanged, camel, underscore string
	}{
		{"format name", `format:"name=CustomerName"`, "CustomerName", "customerName", "customer_name"},
		{"shorthand name", `format:"CustomerName"`, "CustomerName", "customerName", "customer_name"},
		{"empty json", `json:"" format:"name=CustomerName"`, "CustomerName", "customerName", "customer_name"},
		{"json options only", `json:",omitempty" format:"name=CustomerName"`, "CustomerName", "customerName", "customer_name"},
		{"explicit json", `json:"Exact_Name" format:"name=CustomerName,caseFormat=lu"`, "Exact_Name", "Exact_Name", "Exact_Name"},
		{"explicit json matching format", `json:"CustomerName" format:"name=CustomerName"`, "CustomerName", "CustomerName", "CustomerName"},
		{"field case override", `format:"name=CustomerName,caseFormat=lu"`, "customer_name", "customer_name", "customer_name"},
		{"field case without name", `format:"caseFormat=lu"`, "user_name", "user_name", "user_name"},
		{"disabled field case", `format:"name=CustomerName,caseFormat=-"`, "CustomerName", "CustomerName", "CustomerName"},
		{"ignore case formatter", `format:"name=CustomerName,ignoreCaseFormatter=true"`, "CustomerName", "CustomerName", "CustomerName"},
		{"empty field case", `format:"name=CustomerName,caseFormat="`, "CustomerName", "customerName", "customer_name"},
		{"empty format name", `format:"name="`, "UserName", "userName", "user_name"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.StructOf([]reflect.StructField{{Name: "UserName", Type: reflect.TypeFor[string](), Tag: reflect.StructTag(tc.tag)}})
			value := reflect.New(typ)
			value.Elem().Field(0).SetString("Ada")
			for _, global := range []struct {
				name, want string
				opts       []Option
			}{
				{"absent", tc.unchanged, nil},
				{"lc", tc.camel, []Option{WithCaseFormat(text.NewCaseFormat("lc"))}},
				{"lu", tc.underscore, []Option{WithCaseFormat(text.NewCaseFormat("lu"))}},
				{"format option lc", tc.camel, []Option{WithFormatTag(&format.Tag{CaseFormat: "lc"})}},
			} {
				t.Run(global.name, func(t *testing.T) {
					want := `{"` + global.want + `":"Ada"}`
					encoder, err := NewMarshaller(typ, global.opts...)
					require.NoError(t, err)
					shape, err := encoder.Wire()
					require.NoError(t, err)
					require.Len(t, shape.Properties(), 1)
					require.Equal(t, global.want, shape.Properties()[0].Name())
					for _, input := range []any{value.Interface(), value.Elem().Interface()} {
						raw, err := Marshal(input, global.opts...)
						require.NoError(t, err)
						require.JSONEq(t, want, string(raw))
						raw, err = encoder.Marshal(input)
						require.NoError(t, err)
						require.JSONEq(t, want, string(raw))
					}
					decoded := reflect.New(typ)
					opts := append(append([]Option(nil), global.opts...), WithUnknownFieldPolicy(ErrorOnUnknown))
					require.NoError(t, Unmarshal([]byte(want), decoded.Interface(), opts...))
					require.Equal(t, value.Interface(), decoded.Interface())
				})
			}
		})
	}
}

func TestFormatNameCaseUnmarshalAliasesAndPrecedence(t *testing.T) {
	type record struct {
		Name  string `format:"name=CustomerName"`
		Exact string `json:"Exact_Name" format:"name=IgnoredName"`
	}
	opts := []Option{WithCaseFormat(text.NewCaseFormat("lu")), WithUnknownFieldPolicy(ErrorOnUnknown)}
	for _, name := range []string{"customer_name", "CustomerName"} {
		var got record
		require.NoError(t, Unmarshal([]byte(`{"`+name+`":"Ada","Exact_Name":"fixed"}`), &got, opts...))
		require.Equal(t, record{"Ada", "fixed"}, got)
	}
	for _, name := range []string{"IgnoredName", "ignored_name"} {
		var got record
		require.Error(t, Unmarshal([]byte(`{"`+name+`":"wrong"}`), &got, opts...))
		require.Empty(t, got.Exact)
	}
}

func TestFormatNameCaseSelectionAndExclusion(t *testing.T) {
	type record struct {
		Name   string `format:"name=CustomerName"`
		Exact  string `json:"Exact_Name" format:"name=IgnoredName"`
		Note   *string
		Secret string `format:"name=PrivateDetail"`
	}
	for _, tc := range []struct{ name, tag, want string }{
		{"format holder", `format:"name=CustomerRecords"`, "customerRecords"},
		{"empty json holder", `json:",omitempty" format:"name=CustomerRecords"`, "customerRecords"},
		{"explicit json holder", `json:"Exact_Records" format:"name=IgnoredRecords"`, "Exact_Records"},
		{"field case holder", `format:"name=CustomerRecords,caseFormat=lu"`, "customer_records"},
		{"disabled case holder", `format:"name=CustomerRecords,caseFormat=-"`, "CustomerRecords"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			typ := reflect.StructOf([]reflect.StructField{{Name: "Records", Type: reflect.TypeFor[[]record](), Tag: reflect.StructTag(tc.tag)}})
			value := reflect.New(typ)
			value.Elem().Field(0).Set(reflect.ValueOf([]record{{Name: "Ada", Exact: "fixed", Secret: "private"}}))
			caseOption := WithCaseFormat(text.NewCaseFormat("lc"))
			filter, err := NewFieldFilter(typ, []FieldSelection{{Path: []string{"Records"}, Fields: []string{"Name", "Exact", "Note"}}})
			require.NoError(t, err)
			filter, err = filter.WithOptions(caseOption)
			require.NoError(t, err)
			require.True(t, filter.ExcludePath([]string{tc.want}, "Secret"))
			require.False(t, filter.ExcludePath([]string{tc.want}, "Name"))
			for _, exclusion := range []struct {
				name string
				opt  Option
			}{
				{"selection", WithPathFieldExcluder(filter)},
				{"canonical exclusion", WithExcludedFields("Records.Secret")},
				{"wire path exclusion", WithFieldExcluder(compatExcluder{tc.want: {"Secret": true}})},
			} {
				t.Run(exclusion.name, func(t *testing.T) {
					opts := []Option{caseOption, exclusion.opt}
					encoder, err := NewMarshaller(typ, opts...)
					require.NoError(t, err)
					shape, err := encoder.Wire()
					require.NoError(t, err)
					require.Len(t, shape.Properties(), 1)
					holder := shape.Properties()[0]
					require.Equal(t, tc.want, holder.Name())
					fields := holder.Shape().Element().Properties()
					require.Len(t, fields, 3)
					require.Equal(t, "customerName", fields[0].Name())
					require.Equal(t, "Exact_Name", fields[1].Name())
					require.Equal(t, "note", fields[2].Name())
					require.True(t, fields[2].Shape().Nullable())
					want := `{"` + tc.want + `":[{"customerName":"Ada","Exact_Name":"fixed","note":null}]}`
					raw, err := encoder.Marshal(value.Interface())
					require.NoError(t, err)
					require.JSONEq(t, want, string(raw))
					raw, err = Marshal(value.Interface(), opts...)
					require.NoError(t, err)
					require.JSONEq(t, want, string(raw))
				})
			}
		})
	}
}

type formatNameCaseTransformer struct{}

func (formatNameCaseTransformer) Transform(_ string, name string) string { return "custom_" + name }
func (formatNameCaseTransformer) TransformPath(_ []string, name string) string {
	return "custom_" + name
}

func TestFormatNameCasePreservesCustomTransformers(t *testing.T) {
	type row struct{ Value, Secret string }
	type payload struct {
		Rows  []row `format:"name=CustomerRecords"`
		Other string
	}
	for _, opt := range []Option{WithNameTransformer(formatNameCaseTransformer{}), WithPathNameTransformer(formatNameCaseTransformer{})} {
		opts := []Option{WithCaseFormat(text.NewCaseFormat("lc")), opt}
		filter, err := NewFieldFilter(reflect.TypeFor[payload](), []FieldSelection{{Path: []string{"Rows"}, Fields: []string{"Value"}}})
		require.NoError(t, err)
		filter, err = filter.WithOptions(opts...)
		require.NoError(t, err)
		require.True(t, filter.ExcludePath([]string{"CustomerRecords"}, "Secret"))
		opts = append(opts, WithPathFieldExcluder(filter))
		encoder, err := NewMarshaller(reflect.TypeFor[payload](), opts...)
		require.NoError(t, err)
		shape, err := encoder.Wire()
		require.NoError(t, err)
		require.Equal(t, "CustomerRecords", shape.Properties()[0].Name())
		raw, err := encoder.Marshal(payload{Rows: []row{{"public", "private"}}, Other: "other"})
		require.NoError(t, err)
		require.JSONEq(t, `{"CustomerRecords":[{"custom_Value":"public"}],"custom_Other":"other"}`, string(raw))
	}
}

func TestFormatNameCasePreservesInlineSibling(t *testing.T) {
	type child struct{ Value string }
	type payload struct {
		Name string `format:"name=CustomerName"`
		Body child  `jsonx:"inline"`
	}
	value := payload{Name: "Ada", Body: child{Value: "body"}}
	for _, tc := range []struct {
		want string
		opts []Option
	}{
		{`{"CustomerName":"Ada","Value":"body"}`, nil},
		{`{"customerName":"Ada","value":"body"}`, []Option{WithCaseFormat(text.NewCaseFormat("lc"))}},
	} {
		raw, err := Marshal(value, tc.opts...)
		require.NoError(t, err)
		require.JSONEq(t, tc.want, string(raw))
		encoder, err := NewMarshaller(reflect.TypeFor[payload](), tc.opts...)
		require.NoError(t, err)
		shape, err := encoder.Wire()
		require.NoError(t, err)
		require.Len(t, shape.Properties(), 2)
	}
}
