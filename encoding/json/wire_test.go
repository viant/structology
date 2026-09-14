package json

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"github.com/viant/tagly/format/text"
)

type WireEmbedded struct {
	Label  string
	Secret string
}
type wireChild struct {
	Visible string
	Secret  string
}
type wireExample struct {
	*WireEmbedded
	Child  *wireChild   `format:"name=payload"`
	Rows   []*wireChild `json:"rows"`
	Bytes  []byte
	Zero   int                   `format:"nullable=true"`
	Alias  string                `json:"exactName" format:"name=ignored"`
	Hidden string                `internal:"true"`
	Has    *struct{ Alias bool } `setMarker:"true"`
	When   time.Time             `format:"dateFormat=yyyy-MM-dd"`
}

func TestCompiledWireMatchesNativeEncoding(t *testing.T) {
	options := []Option{WithCaseFormat(text.CaseFormatLowerCamel), WithExcludedFields("Child.Secret", "Rows.Secret", "WireEmbedded.Secret")}
	encoder, err := NewMarshaller(reflect.TypeFor[wireExample](), options...)
	require.NoError(t, err)
	shape, err := encoder.Wire()
	require.NoError(t, err)
	fields := map[string]jsonmarshal.WireProperty{}
	for _, field := range shape.Properties() {
		fields[field.Name()] = field
	}
	require.NotContains(t, fields, "secret")
	require.NotContains(t, fields, "Hidden")
	require.NotContains(t, fields, "Has")
	require.False(t, fields["label"].Required(), "nil embedded holder omits its children")
	require.True(t, fields["payload"].Required())
	require.True(t, fields["payload"].Shape().Nullable())
	require.Equal(t, reflect.Slice, fields["bytes"].Shape().Kind(), "this encoder writes numeric byte arrays")
	require.True(t, fields["zero"].Shape().Nullable())
	require.Equal(t, "date", fields["when"].Shape().Format())
	child := fields["payload"].Shape().Element()
	require.Len(t, child.Properties(), 1)
	require.Equal(t, "visible", child.Properties()[0].Name())
	for _, value := range []wireExample{{}, {WireEmbedded: &WireEmbedded{Label: "L", Secret: "private"}, Child: &wireChild{"V", "private"}, Rows: []*wireChild{{"R", "private"}, nil}, Bytes: []byte{1, 2}, Alias: "exact", When: time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)}} {
		compiled, err := encoder.Marshal(&value)
		require.NoError(t, err)
		direct, err := Marshal(&value, options...)
		require.NoError(t, err)
		require.JSONEq(t, string(direct), string(compiled))
		var object map[string]any
		require.NoError(t, json.Unmarshal(compiled, &object))
		require.NotContains(t, string(compiled), "private")
		for name := range object {
			require.Contains(t, fields, name)
		}
		for name, field := range fields {
			if field.Required() {
				require.Contains(t, object, name)
			}
		}
	}
	copied := shape.Properties()
	copied[0] = jsonmarshal.WireProperty{}
	require.NotEmpty(t, shape.Properties()[0].Name())
}

func TestWireCollisionAndOmission(t *testing.T) {
	type collision struct {
		FooBar  int
		Foo_Bar int
	}
	encoder, err := NewMarshaller(reflect.TypeFor[collision](), WithCaseFormat(text.CaseFormatLowerCamel))
	require.NoError(t, err)
	_, err = encoder.Wire()
	require.ErrorContains(t, err, "collision")
	type sample struct {
		Value *int
		Note  string
	}
	encoder, err = NewMarshaller(reflect.TypeFor[sample](), WithOmitEmpty(true))
	require.NoError(t, err)
	shape, err := encoder.Wire()
	require.NoError(t, err)
	for _, field := range shape.Properties() {
		require.False(t, field.Required())
	}
	zero := 0
	bytes, err := encoder.Marshal(sample{Value: &zero})
	require.NoError(t, err)
	require.JSONEq(t, `{}`, string(bytes), "native primitive pointers omit pointee zero")
}

func TestCanonicalExclusionDoesNotDropOtherOwner(t *testing.T) {
	type Holder struct {
		Value string `json:"fromHolder"`
	}
	type sample struct {
		Holder
		Value string `json:"fromRoot"`
	}
	encoder, err := NewMarshaller(reflect.TypeFor[sample](), WithExcludedFields("Holder.Value"))
	require.NoError(t, err)
	shape, err := encoder.Wire()
	require.NoError(t, err)
	require.Len(t, shape.Properties(), 1)
	require.Equal(t, "fromRoot", shape.Properties()[0].Name())
	bytes, err := encoder.Marshal(sample{Holder: Holder{Value: "private"}, Value: "public"})
	require.NoError(t, err)
	require.JSONEq(t, `{"fromRoot":"public"}`, string(bytes))
}

func TestWireRejectsAmbiguousInlineExclusion(t *testing.T) {
	type leaf struct{ Value string }
	type body struct {
		Left  leaf `jsonx:"inline"`
		Right leaf `jsonx:"inline"`
	}
	_, err := NewMarshaller(reflect.TypeFor[body](), WithExcludedFields("Left.Value"))
	require.ErrorContains(t, err, "ambiguous inline exclusion")
}

type wireRecursive struct {
	Value  string
	Secret string
	Next   *wireRecursive
}

func TestWireRecursiveScopedExclusion(t *testing.T) {
	encoder, err := NewMarshaller(reflect.TypeFor[wireRecursive](), WithExcludedFields("Next.Secret"))
	require.NoError(t, err)
	shape, err := encoder.Wire()
	require.NoError(t, err)
	require.Len(t, shape.Properties(), 3)
	next := shape.Properties()[2].Shape().Element()
	require.Len(t, next.Properties(), 2)
	unscoped := next.Properties()[1].Shape().Element()
	require.Len(t, unscoped.Properties(), 3)
	value := wireRecursive{Secret: "top", Next: &wireRecursive{Secret: "hidden", Next: &wireRecursive{Secret: "bottom"}}}
	raw, err := encoder.Marshal(value)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "hidden")
	require.Contains(t, string(raw), "bottom")
}
