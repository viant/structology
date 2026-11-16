package csv

import (
	"reflect"
	"strings"
	"testing"
)

func TestMarshallerPointerContainers(t *testing.T) {
	type fee struct {
		Amount      int
		Description string
	}
	type fees = []fee
	type namedFees []*fee
	type row struct {
		ID   int
		Fees *fees
	}
	populated := fees{{19, "invented"}, {23, "second"}}
	empty := fees{}
	var nilSlice fees
	rows := []row{{ID: 1}, {ID: 2, Fees: &nilSlice}, {ID: 3, Fees: &empty}, {ID: 4, Fees: &populated}}
	m, err := NewMarshaller(reflect.TypeFor[row](), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := m.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	want := "\"ID\",\"Fees.Amount\",\"Fees.Description\"\n1,null,null\n2,null,null\n3,null,null\n4,19,\"invented\"\n4,23,\"second\""
	if string(got) != want {
		t.Fatalf("got %s; want %s", got, want)
	}
	if rows[3].Fees != &populated || len(populated) != 2 || populated[0].Description != "invented" {
		t.Fatal("input mutated")
	}
	denied, err := NewMarshaller(reflect.TypeFor[row](), &Config{ExcludedPaths: []string{"Fees"}})
	if err != nil {
		t.Fatal(err)
	}
	deniedCSV, err := denied.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	if string(deniedCSV) != "\"ID\"\n1\n2\n3\n4" || strings.Contains(string(deniedCSV), "invented") {
		t.Fatalf("excluded relation leaked: %s", deniedCSV)
	}

	t.Run("nested pointers and sibling products", func(t *testing.T) {
		type parent struct {
			ID      int
			A       **namedFees
			B       []*fee
			Profile **fee
		}
		a := namedFees{nil, {Amount: 7, Description: "a"}}
		ap := &a
		profile := &fee{Amount: 11, Description: "profile"}
		input := []parent{{ID: 1}, {ID: 2, A: &ap, B: []*fee{{Amount: 13, Description: "b"}, {Amount: 17, Description: "c"}}, Profile: &profile}}
		m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.Marshal(input)
		if err != nil {
			t.Fatal(err)
		}
		want := "\"ID\",\"A.Amount\",\"A.Description\",\"B.Amount\",\"B.Description\",\"Profile.Amount\",\"Profile.Description\"\n1,null,null,null,null,null,null\n2,null,null,13,\"b\",11,\"profile\"\n2,7,\"a\",13,\"b\",11,\"profile\"\n2,null,null,17,\"c\",11,\"profile\"\n2,7,\"a\",17,\"c\",11,\"profile\""
		if string(got) != want {
			t.Fatalf("got %s; want %s", got, want)
		}
		if input[1].A != &ap || ap != &a || a[0] != nil || input[1].Profile != &profile || profile.Amount != 11 {
			t.Fatal("input graph mutated")
		}
	})
	t.Run("pointer elements with nil inner pointers", func(t *testing.T) {
		type parent struct {
			ID   int
			Fees *[]**fee
		}
		value := &fee{31, "deep"}
		var nilFee *fee
		items := []**fee{nil, &nilFee, &value}
		m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.Marshal([]parent{{ID: 8, Fees: &items}, {ID: 9}})
		if err != nil {
			t.Fatal(err)
		}
		want := "\"ID\",\"Fees.Amount\",\"Fees.Description\"\n8,null,null\n8,null,null\n8,31,\"deep\"\n9,null,null"
		if string(got) != want {
			t.Fatalf("got %s; want %s", got, want)
		}
		if items[0] != nil || *items[1] != nil || *items[2] != value {
			t.Fatal("input graph mutated")
		}
	})
	t.Run("nil owner clears descendant slice", func(t *testing.T) {
		type group struct{ Fees *[]fee }
		type parent struct {
			ID     int
			Groups *[]*group
		}
		values := []fee{{37, "nested"}, {41, "other"}}
		groups := []*group{{Fees: &values}, nil}
		m, err := NewMarshaller(reflect.TypeFor[parent](), nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := m.Marshal([]parent{{ID: 5, Groups: &groups}, {ID: 6}})
		if err != nil {
			t.Fatal(err)
		}
		want := "\"ID\",\"Groups.Fees.Amount\",\"Groups.Fees.Description\"\n5,37,\"nested\"\n5,41,\"other\"\n5,null,null\n6,null,null"
		if string(got) != want {
			t.Fatalf("got %s; want %s", got, want)
		}
	})
}

func BenchmarkMarshalPointerContainer1000(b *testing.B) {
	type fee struct {
		Amount      int
		Description string
	}
	type directRow struct {
		ID       int
		Fees     []fee
		Siblings []fee
	}
	type pointerRow struct {
		ID       int
		Fees     *[]fee
		Siblings *[]fee
	}
	direct := make([]directRow, 1000)
	pointers := make([]pointerRow, 1000)
	for i := range direct {
		direct[i] = directRow{i, []fee{{19, "first"}, {23, "second"}}, []fee{{29, "sibling"}, {31, "other"}}}
		pointers[i] = pointerRow{i, &direct[i].Fees, &direct[i].Siblings}
	}
	for _, tc := range []struct {
		name string
		typ  reflect.Type
		rows any
	}{{"direct", reflect.TypeFor[directRow](), direct}, {"pointer", reflect.TypeFor[pointerRow](), pointers}} {
		b.Run(tc.name, func(b *testing.B) {
			m, err := NewMarshaller(tc.typ, nil)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := m.Marshal(tc.rows); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
