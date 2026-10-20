package structology

import (
	"encoding/json"
	"github.com/viant/xunsafe"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestMapsDynamicNestedRoundTrip(t *testing.T) {
	in := Maps{
		{"id": json.Number("9007199254740993"), "active": false, "age": nil, "profile": map[string]any{"name": "", "score": 0}, "events": []map[string]any{{"kind": "open"}, {"count": 0}}},
		{"profile": nil, "events": []map[string]any{}, "active": true},
		{"profile": map[string]any{"score": nil}, "events": []map[string]any{{"count": 2}}},
		{},
	}
	var batch any
	if err := in.Load(&batch); err != nil {
		t.Fatal(err)
	}
	v := reflect.ValueOf(batch)
	if v.Kind() != reflect.Slice || v.Len() != len(in) || v.Type().Elem().Kind() != reflect.Ptr {
		t.Fatalf("not a typed pointer slice: %T", batch)
	}
	row := v.Index(0).Elem()
	if !HasSetMarker(row.Type()) || row.FieldByName("Has").IsNil() {
		t.Fatal("missing allocated root marker")
	}
	marker, err := NewMarker(row.Type())
	if err != nil {
		t.Fatal(err)
	}
	first := xunsafe.AsPointer(v.Index(0).Interface())
	second := xunsafe.AsPointer(v.Index(1).Interface())
	if !marker.IsFieldSet(first, "Id") || marker.IsFieldSet(second, "Id") || !marker.IsFieldSet(second, "Active") {
		t.Fatal("generated marker lookup failed")
	}
	profile := row.FieldByName("Profile").Elem()
	if !HasSetMarker(profile.Type()) || profile.FieldByName("Has").IsNil() {
		t.Fatal("missing allocated nested marker")
	}
	var out Maps
	if err := out.Store(batch); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip\n got: %#v\nwant: %#v", out, in)
	}
}

func TestMapsJSONStyleRepeatedAndNullElement(t *testing.T) {
	in := Maps{{"items": []any{map[string]any{"a": json.Number("0")}, nil, map[string]any{"b": false}}}, {"items": []any{}}}
	var batch any
	if err := in.Load(&batch); err != nil {
		t.Fatal(err)
	}
	var out Maps
	if err := out.Store(batch); err != nil {
		t.Fatal(err)
	}
	items := out[0]["items"].([]map[string]any)
	if len(items) != 3 || items[1] != nil || items[0]["a"] != json.Number("0") || items[2]["b"] != false {
		t.Fatalf("wrong repeated records: %#v", items)
	}
	if len(out[1]["items"].([]map[string]any)) != 0 {
		t.Fatal("empty repeated record lost")
	}
}

func TestMapsSuppliedTypeAndEmpty(t *testing.T) {
	type flags struct{ Name, Age, Child, Items bool }
	type childFlags struct{ Code bool }
	type child struct {
		Code string      `json:"code"`
		Has  *childFlags `setMarker:"true"`
	}
	type row struct {
		Name  string   `json:"name"`
		Age   *int64   `json:"age"`
		Child *child   `json:"child"`
		Items []*child `json:"items"`
		Has   *flags   `setMarker:"true"`
	}
	in := Maps{{"name": "", "age": nil, "child": map[string]any{}, "items": []map[string]any{{"code": ""}, {}}}, {"items": []map[string]any{}}}
	var typed []*row
	if err := in.Load(&typed); err != nil {
		t.Fatal(err)
	}
	if typed[0].Has == nil || !typed[0].Has.Name || !typed[0].Has.Age || !typed[0].Has.Child || !typed[0].Has.Items || typed[1].Has.Name {
		t.Fatalf("wrong root presence: %+v %+v", typed[0].Has, typed[1].Has)
	}
	if typed[0].Child == nil || typed[0].Child.Has == nil || typed[0].Child.Has.Code {
		t.Fatalf("wrong empty child: %+v", typed[0].Child)
	}
	var out Maps
	if err := out.Store(typed); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip got %#v want %#v", out, in)
	}
	var empty []row
	if err := (Maps{}).Load(&empty); err != nil {
		t.Fatal(err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("wrong empty slice: %#v", empty)
	}
}

func TestMapsExplicitSchemaAndErrors(t *testing.T) {
	fields := []MapField{{Key: "a-b", Type: reflect.TypeOf(int8(0))}, {Key: "a_b", Type: reflect.TypeOf("")}, {Key: "nested", Fields: []MapField{{Key: "value", Type: reflect.TypeOf(int64(0))}}}}
	p, err := NewMapPlan(fields)
	if err != nil {
		t.Fatal(err)
	}
	if p.Type().Field(0).Name == p.Type().Field(1).Name {
		t.Fatal("name collision")
	}
	var empty any
	if err := (Maps{}).Load(&empty, WithMapPlan(p)); err != nil {
		t.Fatal(err)
	}
	if reflect.ValueOf(empty).Len() != 0 {
		t.Fatal("wrong empty batch")
	}
	var got any
	if err := (Maps{{"a-b": json.Number("127"), "a_b": "", "nested": map[string]any{"value": json.Number("9007199254740993")}}}).Load(&got, WithMapPlan(p)); err != nil {
		t.Fatal(err)
	}
	var out Maps
	if err := out.Store(got, WithMapPlan(p)); err != nil {
		t.Fatal(err)
	}
	if out[0]["a-b"] != int8(127) || out[0]["nested"].(map[string]any)["value"] != int64(9007199254740993) {
		t.Fatalf("wrong conversion: %#v", out)
	}
	original := reflect.ValueOf(got).Pointer()
	if err := (Maps{{"a-b": json.Number("128")}}).Load(&got, WithMapPlan(p)); err == nil || !strings.Contains(err.Error(), "overflows") {
		t.Fatalf("expected overflow, got %v", err)
	}
	if reflect.ValueOf(got).Pointer() != original {
		t.Fatal("destination changed after failed load")
	}
	if err := (Maps{{"nested": map[string]any{"bad": 1}}}).Load(&got, WithMapPlan(p)); err == nil || !strings.Contains(err.Error(), `key "nested": unknown key "bad"`) {
		t.Fatalf("missing nested context: %v", err)
	}
	if _, err := InferMapPlan(Maps{{"x": 1}, {"x": "1"}}); err == nil {
		t.Fatal("accepted mixed types")
	}
	if err := (Maps{{"x": json.Number("9007199254740993")}}).Load(&got, WithMapFields(MapField{Key: "x", Type: reflect.TypeOf(float64(0))})); err == nil || !strings.Contains(err.Error(), "precision") {
		t.Fatalf("expected precision error: %v", err)
	}
}

func TestMapsStoreAtomicAndPartialMarker(t *testing.T) {
	type flags struct{ ID bool }
	type row struct {
		ID    int    `json:"id"`
		Extra string `json:"extra"`
		Has   *flags `setMarker:"true"`
	}
	src := []*row{{ID: 0, Extra: "kept", Has: &flags{ID: true}}, nil}
	out := Maps{{"original": true}}
	if err := out.Store(src); err == nil {
		t.Fatal("expected nil row error")
	}
	if !reflect.DeepEqual(out, Maps{{"original": true}}) {
		t.Fatalf("receiver changed: %#v", out)
	}
	if err := out.Store(src[:1]); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, Maps{{"id": 0, "extra": "kept"}}) {
		t.Fatalf("untracked field was omitted: %#v", out)
	}
	if _, err := InferMapPlan(Maps{{"items": []any{}}}); err == nil || !strings.Contains(err.Error(), "no nested shape") {
		t.Fatalf("expected explicit nested schema requirement: %v", err)
	}
}

func TestMapPlanConcurrentUse(t *testing.T) {
	p, err := NewMapPlan([]MapField{{Key: "n", Type: reflect.TypeOf(int64(0))}, {Key: "child", Fields: []MapField{{Key: "x", Type: reflect.TypeOf("")}}}})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			in := Maps{{"n": int64(0), "child": map[string]any{"x": ""}}, {"child": map[string]any{}}}
			var dst any
			if err := p.Load(in, &dst); err != nil {
				errs <- err
				return
			}
			out, err := p.Store(dst)
			if err != nil {
				errs <- err
				return
			}
			if !reflect.DeepEqual(in, out) {
				errs <- errMismatch{}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func TestInferMapPlanReusesSchema(t *testing.T) {
	in := Maps{{"x": 1, "child": map[string]any{"y": true}}}
	a, err := InferMapPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	b, err := InferMapPlan(in)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("inferred plan was not reused")
	}
}

type errMismatch struct{}

func (errMismatch) Error() string { return "concurrent round trip mismatch" }

func BenchmarkMapPlanLoadStore(b *testing.B) {
	in := Maps{{"id": int64(4), "profile": map[string]any{"name": "Ada", "active": true}}}
	p, err := InferMapPlan(in)
	if err != nil {
		b.Fatal(err)
	}
	b.Run("compiled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var batch any
			if err := p.Load(in, &batch); err != nil {
				b.Fatal(err)
			}
			if _, err := p.Store(batch); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("reflection_compile_each", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			plan, err := NewMapPlan([]MapField{{Key: "id", Type: reflect.TypeOf(int64(0))}, {Key: "profile", Fields: []MapField{{Key: "name", Type: reflect.TypeOf("")}, {Key: "active", Type: reflect.TypeOf(true)}}}})
			if err != nil {
				b.Fatal(err)
			}
			var batch any
			if err := plan.Load(in, &batch); err != nil {
				b.Fatal(err)
			}
			if _, err := plan.Store(batch); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("json_map_round_trip", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			data, err := json.Marshal(in)
			if err != nil {
				b.Fatal(err)
			}
			var out Maps
			if err := json.Unmarshal(data, &out); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestMapPlanFieldsDefensiveProjection(t *testing.T) {
	original, err := InferMapPlan(Maps{{"name": "a", "child": map[string]any{"id": int64(1)}}, {"other": true}})
	if err != nil {
		t.Fatal(err)
	}
	fields := original.Fields()
	projected, err := NewMapPlan(fields)
	if err != nil {
		t.Fatal(err)
	}
	fields[0].Key = "modified"
	var adapted any
	if err := projected.Load(Maps{{"child": map[string]any{"id": int64(0)}}}, &adapted); err != nil {
		t.Fatal(err)
	}
	var restored Maps
	if err := restored.Store(adapted); err != nil {
		t.Fatal(err)
	}
	if len(restored[0]) != 1 || restored[0]["child"].(map[string]any)["id"] != int64(0) {
		t.Fatal(restored)
	}
	if original.Fields()[0].Key == "modified" {
		t.Fatal("schema mutated")
	}
}
