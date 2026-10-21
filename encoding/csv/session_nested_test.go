package csv

import (
	"reflect"
	"strings"
	"testing"
)

func TestMarshallerNestedObjectSession(t *testing.T) {
	type profile struct{ Name string }
	type valueRow struct {
		ID      int
		Profile profile
	}
	type pointerRow struct {
		ID      int
		Profile *profile
	}
	for _, tc := range []struct {
		name string
		row  any
		rows any
	}{{"value", valueRow{}, []valueRow{{ID: 1, Profile: profile{Name: "Ada"}}}}, {"pointer", pointerRow{}, []pointerRow{{ID: 1, Profile: &profile{Name: "Ada"}}}}} {
		t.Run(tc.name, func(t *testing.T) {
			marshaller, err := NewMarshaller(reflect.TypeOf(tc.row), nil)
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := marshaller.Marshal(tc.rows)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(encoded), "Ada") {
				t.Fatalf("encoded=%s", encoded)
			}
		})
	}
}

func TestMarshallerProjectionWithoutParentScalars(t *testing.T) {
	type child struct {
		Value int
		Note  *string
	}
	type parent struct{ Children []child }
	m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Marshal([]parent{{Children: []child{{Value: 0}, {Value: 2}}}})
	if err != nil {
		t.Fatal(err)
	}
	want := "\"Children.Value\",\"Children.Note\"\n0,null\n2,null"
	if string(got) != want {
		t.Fatalf("%s; want %s", got, want)
	}
}

func TestMarshallerSharedTypedHolders(t *testing.T) {
	type child struct{ Value int }
	type parent struct {
		A child
		B child
	}
	m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Marshal([]parent{{A: child{1}, B: child{2}}})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\"A.Value\",\"B.Value\"\n1,2" {
		t.Fatal(string(got))
	}
}

func TestMarshallerSiblingSlicesRetainCartesianValues(t *testing.T) {
	type child struct{ Value int }
	type parent struct {
		ID int
		A  []child
		B  []child
	}
	m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Marshal([]parent{{ID: 7, A: []child{{1}, {2}}, B: []child{{10}, {20}}}})
	if err != nil {
		t.Fatal(err)
	}
	want := "\"ID\",\"A.Value\",\"B.Value\"\n7,1,10\n7,2,10\n7,1,20\n7,2,20"
	if string(got) != want {
		t.Fatalf("%s; want %s", got, want)
	}
}
