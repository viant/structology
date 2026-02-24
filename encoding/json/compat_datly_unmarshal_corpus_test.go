package json

import (
	"reflect"
	"testing"
)

func TestCompat_DatlyUnmarshalCorpus(t *testing.T) {
	t.Run("invalid conversion object to slice", func(t *testing.T) {
		type Foo struct {
			ID   int
			Name string
		}
		out := []*Foo{}
		err := Unmarshal([]byte(`{"Name":"Foo","ID":1}`), &out)
		if err == nil {
			t.Fatalf("expected error for object to slice conversion")
		}
	})

	t.Run("invalid conversion slice to object", func(t *testing.T) {
		type Foo struct {
			ID   int
			Name string
		}
		var out Foo
		err := Unmarshal([]byte(`[{"Name":"Foo","ID":1}]`), &out)
		if err == nil {
			t.Fatalf("expected error for slice to object conversion")
		}
	})

	t.Run("basic struct with missing comma compat tolerant", func(t *testing.T) {
		type Foo struct {
			ID   int
			Name string
		}
		var out Foo
		if err := Unmarshal([]byte(`{"Name":"Foo" "ID":2}`), &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.ID != 2 || out.Name != "Foo" {
			t.Fatalf("unexpected value: %+v", out)
		}
	})

	t.Run("basic struct with missing comma strict", func(t *testing.T) {
		type Foo struct {
			ID   int
			Name string
		}
		var out Foo
		err := Unmarshal([]byte(`{"Name":"Foo" "ID":2}`), &out, WithMode(ModeStrict))
		if err == nil {
			t.Fatalf("expected strict mode error")
		}
	})

	t.Run("primitive slice", func(t *testing.T) {
		var out []int
		if err := Unmarshal([]byte(`[1,2,3,4,5]`), &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(out) != 5 || out[0] != 1 || out[4] != 5 {
			t.Fatalf("unexpected slice: %#v", out)
		}
	})

	t.Run("null pointers", func(t *testing.T) {
		type Foo struct {
			ID   *int
			Name *string
		}
		var out Foo
		if err := Unmarshal([]byte(`{"ID":null,"Name":null}`), &out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.ID != nil || out.Name != nil {
			t.Fatalf("expected nil pointers, got: %#v", out)
		}
	})

	t.Run("broken case 17a", func(t *testing.T) {
		rType := reflect.TypeOf(struct {
			Id       int     "sqlx:\"name=ID,autoincrement,primaryKey,required\""
			Name     *string "sqlx:\"name=NAME\" json:\",omitempty\""
			Quantity *int    "sqlx:\"name=QUANTITY\" json:\",omitempty\""
			Has      *struct {
				Id       bool
				Name     bool
				Quantity bool
			} "setMarker:\"true\" typeName:\"EventsHas\" json:\"-\" sqlx:\"presence=true\""
		}{})
		out := reflect.New(rType).Interface()
		if err := Unmarshal([]byte(`{"Name":"017_"}`), out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := Marshal(out)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		assertJSONEqual(t, `{"Id":0,"Name":"017_"}`, string(data))
	})

	t.Run("broken case 17b", func(t *testing.T) {
		rType := reflect.TypeOf(struct {
			Data *struct {
				Id       int     "sqlx:\"name=ID,autoincrement,primaryKey,required\""
				Name     *string "sqlx:\"name=NAME\" json:\",omitempty\""
				Quantity *int    "sqlx:\"name=QUANTITY\" json:\",omitempty\""
				Has      *struct {
					Id       bool
					Name     bool
					Quantity bool
				} "setMarker:\"true\" typeName:\"EventsHas\" json:\"-\" sqlx:\"presence=true\""
			} `json:"data"`
		}{})
		out := reflect.New(rType).Interface()
		if err := Unmarshal([]byte(`{"data":null}`), out); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, err := Marshal(out)
		if err != nil {
			t.Fatalf("marshal error: %v", err)
		}
		assertJSONEqual(t, `{"data":null}`, string(data))
	})
}
