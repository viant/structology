package json

import (
	stdjson "encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

// Test that marshaling a struct with an embedded pointer and a setMarker presence holder
// produces valid JSON with promoted fields and does not choke on the marker.
func TestMarshal_Marker_EmbeddedPtr(t *testing.T) {
	type Foo struct {
		Id   int
		Name string
	}
	type HasFoo struct {
		Id   bool
		Name bool
	}
	type MutableFoo struct {
		*Foo
		Has *HasFoo `setMarker:"true"`
	}

	v := MutableFoo{
		Foo: &Foo{Id: 1, Name: "Alice"},
		Has: &HasFoo{Id: true},
	}

	out, err := Marshal(v)
	require.NoError(t, err)

	// Verify JSON shape using stdlib
	var got map[string]any
	require.NoError(t, stdjson.Unmarshal(out, &got))

	require.Equal(t, float64(1), got["Id"])
	require.Equal(t, "Alice", got["Name"])

	// Presence marker is omitted by default
	_, present := got["Has"]
	require.False(t, present)
}

// Test the same scenario with a value-embedded Foo (non-pointer) to ensure both forms work.
func TestMarshal_Marker_EmbeddedValue(t *testing.T) {
	type Foo struct {
		Id   int
		Name string
	}
	type HasFoo struct {
		Id   bool
		Name bool
	}
	type MutableFoo struct {
		Foo
		Has *HasFoo `setMarker:"true"`
	}

	v := MutableFoo{
		Foo: Foo{Id: 2, Name: "Bob"},
		Has: &HasFoo{Name: true},
	}

	out, err := Marshal(v)
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, stdjson.Unmarshal(out, &got))

	require.Equal(t, float64(2), got["Id"])
	require.Equal(t, "Bob", got["Name"])

	// Presence marker is omitted by default
	_, present := got["Has"]
	require.False(t, present)
}
