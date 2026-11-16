package json_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	structjson "github.com/viant/structology/encoding/json"
	"github.com/viant/x/shape"
)

func TestFieldFilterRepresentationOwnership(t *testing.T) {
	type row struct {
		ID      int    `json:"id"`
		Label   string `json:"label"`
		Backing string `json:"-"`
	}
	type envelope struct {
		Data []row `json:"data"`
	}
	value := envelope{Data: []row{{ID: 2, Label: "kept internally", Backing: "READY"}}}
	typ := reflect.TypeOf(value)
	filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Path: []string{"Data"}, Indexes: [][]int{{0}}}})
	if err != nil {
		t.Fatal(err)
	}
	if !filter.ExcludeIndexes([]int{0, 2}) {
		t.Fatal("canonical backing index was not excluded")
	}
	before, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name            string
		parent, backing bool
	}{
		{"unchanged", false, false}, {"exposed-backing", false, true},
		{"renamed-holder", true, false}, {"renamed-holder-and-backing", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			walker, err := shape.CompileRewrite(typ)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
				if tc.parent && parent == typ && f.Name == "Data" {
					return &shape.FieldMetadata{Name: f.Name, Tag: `json:"roles"`}, nil
				}
				if tc.backing && parent == reflect.TypeFor[row]() && f.Name == "Backing" {
					return &shape.FieldMetadata{Name: f.Name, Tag: `json:"state"`}, nil
				}
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := structjson.Marshal(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
			if err != nil {
				t.Fatal(err)
			}
			var actual any
			if err = json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			key := "data"
			if tc.parent {
				key = "roles"
			}
			want := map[string]any{key: []any{map[string]any{"id": float64(2)}}}
			if !reflect.DeepEqual(actual, want) {
				t.Errorf("unselected members escaped effective filter: %s", encoded)
			}
		})
	}
	after, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || value.Data[0].Label != "kept internally" || value.Data[0].Backing != "READY" {
		t.Fatal("original filter/model changed")
	}
}

type filterRepresentationRow struct {
	ID      int    `json:"id"`
	Label   string `json:"label"`
	Backing string `json:"-"`
	Code    int    `json:"-"`
}

type filterRepresentationHolder struct {
	Data []*filterRepresentationRow `json:"data"`
}

type filterRepresentationEnvelope struct {
	Payload []filterRepresentationHolder `json:"payload"`
	Other   []filterRepresentationRow    `json:"other"`
}

func filterRepresentation(t testing.TB, typ reflect.Type, variant string) *shape.Representation {
	t.Helper()
	walker, err := shape.CompileRewrite(typ)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		switch {
		case f.Name == "Payload":
			if variant == "hidden-parent" {
				return &shape.FieldMetadata{Name: "PublicPayload", Tag: `json:"-"`}, nil
			}
			return &shape.FieldMetadata{Name: "PublicPayload", Tag: `json:"containers"`}, nil
		case parent == reflect.TypeFor[filterRepresentationHolder]() && f.Name == "Data":
			if variant == "hidden-holder" {
				return &shape.FieldMetadata{Name: "PublicData", Tag: `json:"-"`}, nil
			}
			if variant == "opposite" {
				return &shape.FieldMetadata{Name: "PublicData", Tag: `json:"opposite"`}, nil
			}
			return &shape.FieldMetadata{Name: "PublicData", Tag: `json:"roles"`}, nil
		case parent == reflect.TypeFor[filterRepresentationRow]() && f.Name == "Backing":
			if variant == "opposite" {
				return &shape.FieldMetadata{Name: "PublicBacking", Tag: `json:"-"`}, nil
			}
			return &shape.FieldMetadata{Name: "PublicBacking", Tag: `json:"state"`}, nil
		case parent == reflect.TypeFor[filterRepresentationRow]() && f.Name == "Code":
			return &shape.FieldMetadata{Name: "PublicCode", Tag: `json:"mode" choice:"0:draft" choice:"10:active" enumStyle:"literal"`}, nil
		case parent == reflect.TypeFor[filterRepresentationRow]() && f.Name == "ID":
			if variant == "opposite" {
				return &shape.FieldMetadata{Name: "PublicID", Tag: `json:"-"`}, nil
			}
			return &shape.FieldMetadata{Name: "PublicID", Tag: `json:"identifier"`}, nil
		case parent == reflect.TypeFor[filterRepresentationRow]() && f.Name == "Label":
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

func TestFieldFilterRepresentationNestedSelection(t *testing.T) {
	row := &filterRepresentationRow{ID: 7, Label: "private", Backing: "READY", Code: 10}
	payload := []filterRepresentationHolder{{Data: []*filterRepresentationRow{row, nil}}}
	for _, value := range []any{
		&filterRepresentationEnvelope{Payload: payload, Other: []filterRepresentationRow{}},
		&struct {
			Payload []filterRepresentationHolder `json:"payload"`
			Other   []filterRepresentationRow    `json:"other"`
		}{Payload: payload, Other: []filterRepresentationRow{}},
	} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			typ := reflect.TypeOf(value)
			for _, tc := range []struct {
				name, variant, want string
				fields              []string
				indexes             [][]int
			}{
				{"id-only", "exposed", `{"containers":[{"roles":[{"identifier":7},null]}],"other":[]}`, nil, [][]int{{0}}},
				{"backing-and-enum", "exposed", `{"containers":[{"roles":[{"state":"READY","mode":"active"},null]}],"other":[]}`, []string{"Backing", "Code"}, nil},
				{"selected-hidden", "exposed", `{"containers":[{"roles":[{},null]}],"other":[]}`, []string{"Label"}, nil},
				{"opposite", "opposite", `{"containers":[{"opposite":[{"mode":"active"},null]}],"other":[]}`, []string{"ID", "Backing", "Code"}, nil},
				{"hidden-holder", "hidden-holder", `{"containers":[{}],"other":[]}`, []string{"ID", "Backing", "Code"}, nil},
				{"hidden-parent", "hidden-parent", `{"other":[]}`, []string{"ID", "Backing", "Code"}, nil},
			} {
				t.Run(tc.name, func(t *testing.T) {
					filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: tc.fields, Indexes: tc.indexes}})
					if err != nil {
						t.Fatal(err)
					}
					metadata := filterRepresentation(t, typ, tc.variant)
					configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
					if err != nil {
						t.Fatal(err)
					}
					for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
						encoded, err := encode(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
						if err != nil {
							t.Fatal(err)
						}
						assertFilterRepresentationJSON(t, tc.want, encoded)
					}
				})
			}
		})
	}
	if *row != (filterRepresentationRow{ID: 7, Label: "private", Backing: "READY", Code: 10}) {
		t.Fatal("storage changed")
	}
}

func assertFilterRepresentationJSON(t *testing.T, want string, encoded []byte) {
	t.Helper()
	var expected, actual any
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(encoded, &actual); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("got %s, want %s", encoded, want)
	}
}

func TestFieldFilterRepresentationVariantsConcurrent(t *testing.T) {
	value := &filterRepresentationEnvelope{Payload: []filterRepresentationHolder{{Data: []*filterRepresentationRow{{ID: 7, Label: "private", Backing: "READY", Code: 10}}}}, Other: []filterRepresentationRow{}}
	typ := reflect.TypeOf(value)
	filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: []string{"ID", "Backing"}}})
	if err != nil {
		t.Fatal(err)
	}
	before, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	if err != nil {
		t.Fatal(err)
	}
	metadata := []*shape.Representation{filterRepresentation(t, typ, "exposed"), filterRepresentation(t, typ, "opposite")}
	want := []string{`{"containers":[{"roles":[{"identifier":7,"state":"READY"}]}],"other":[]}`, `{"containers":[{"opposite":[{}]}],"other":[]}`}
	var group sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		group.Add(1)
		go func(variant int) {
			defer group.Done()
			for iteration := 0; iteration < 10; iteration++ {
				configured, err := filter.WithOptions(structjson.WithRepresentation(metadata[variant]))
				if err != nil {
					t.Error(err)
					return
				}
				if configured.ExcludeIndexes([]int{0, 0, 2}) || !configured.ExcludeIndexes([]int{0, 0, 3}) {
					t.Error("canonical index policy changed")
					return
				}
				for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
					encoded, err := encode(value, structjson.WithRepresentation(metadata[variant]), structjson.WithPathFieldExcluder(configured))
					if err != nil {
						t.Error(err)
						return
					}
					assertFilterRepresentationJSON(t, want[variant], encoded)
				}
			}
		}(worker % 2)
	}
	group.Wait()
	after, err := structjson.MarshalStandard(value, structjson.WithPathFieldExcluder(filter))
	if err != nil || string(before) != string(after) {
		t.Fatalf("original standard filter changed: %s %v", after, err)
	}
	configured, err := filter.WithOptions()
	if err != nil {
		t.Fatal(err)
	}
	native, err := structjson.Marshal(value, structjson.WithPathFieldExcluder(configured))
	if err != nil || string(native) != string(before) {
		t.Fatalf("original native filter changed: %s %v", native, err)
	}
}

func TestFieldFilterRepresentationNameWithoutJSONAlias(t *testing.T) {
	type row struct {
		ID      int
		Backing string `json:"-"`
	}
	type envelope struct{ Data []row }
	value := envelope{Data: []row{{ID: 2, Backing: "hidden"}}}
	walker, _ := shape.CompileRewrite(reflect.TypeOf(value))
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Data" {
			return &shape.FieldMetadata{Name: "PublicData"}, nil
		}
		if f.Name == "Backing" {
			return &shape.FieldMetadata{Name: "PublicBacking"}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
	if err != nil {
		t.Fatal(err)
	}
	for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
		encoded, err := encode(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
		if err != nil {
			t.Fatal(err)
		}
		assertFilterRepresentationJSON(t, `{"PublicData":[{"ID":2}]}`, encoded)
	}
	if _, err := filter.WithOptions(structjson.WithRepresentation(filterRepresentation(t, reflect.TypeFor[filterRepresentationEnvelope](), "exposed"))); err == nil {
		t.Fatal("unrelated storage representation accepted")
	}
}

type FilterRepresentationEmbedded struct {
	Backing string `json:"-"`
	ID      int    `json:"id"`
}

type filterRepresentationPromoted struct {
	*FilterRepresentationEmbedded
	Label string `json:"label"`
}

func TestFieldFilterRepresentationPromotedCanonicalIndexes(t *testing.T) {
	value := filterRepresentationPromoted{FilterRepresentationEmbedded: &FilterRepresentationEmbedded{Backing: "READY", ID: 9}, Label: "private"}
	walker, _ := shape.CompileRewrite(reflect.TypeOf(value))
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Backing" {
			return &shape.FieldMetadata{Name: "PublicBacking", Tag: `json:"state"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Indexes: [][]int{{0, 0}}}})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
	if err != nil {
		t.Fatal(err)
	}
	for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
		encoded, err := encode(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
		if err != nil {
			t.Fatal(err)
		}
		assertFilterRepresentationJSON(t, `{"state":"READY"}`, encoded)
	}
}

func TestFieldFilterRepresentationPreservesHolderCollisionGuard(t *testing.T) {
	type row struct {
		ID int `json:"id"`
	}
	type extra struct {
		Rows []row `json:"roles"`
	}
	type envelope struct {
		Extra extra `json:"extra"`
		Data  []row `json:"data"`
	}
	typ := reflect.TypeFor[envelope]()
	walker, _ := shape.CompileRewrite(typ)
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == typ && f.Name == "Data" {
			return &shape.FieldMetadata{Name: "PublicData", Tag: `json:"roles"`}, nil
		}
		if parent == typ && f.Name == "Extra" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"extra" jsonx:"inline"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = filter.WithOptions(structjson.WithRepresentation(metadata)); err == nil || !strings.Contains(err.Error(), "not uniquely visible") {
		t.Fatalf("effective inline collision was not rejected: %v", err)
	}
}

func TestFieldFilterRepresentationKeepsSameTypeOccurrencesIndependent(t *testing.T) {
	row := filterRepresentationRow{ID: 7, Label: "private", Backing: "READY", Code: 10}
	value := filterRepresentationEnvelope{Payload: []filterRepresentationHolder{{Data: []*filterRepresentationRow{&row}}}, Other: []filterRepresentationRow{row}}
	metadata := filterRepresentation(t, reflect.TypeOf(value), "exposed")
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: []string{"ID"}}})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
	if err != nil {
		t.Fatal(err)
	}
	for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
		encoded, err := encode(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
		if err != nil {
			t.Fatal(err)
		}
		assertFilterRepresentationJSON(t, `{"containers":[{"roles":[{"identifier":7}]}],"other":[{"identifier":7,"state":"READY","mode":"active"}]}`, encoded)
	}
}

func BenchmarkFieldFilterRepresentation(b *testing.B) {
	value := &filterRepresentationEnvelope{Payload: make([]filterRepresentationHolder, 100), Other: []filterRepresentationRow{}}
	for i := range value.Payload {
		value.Payload[i].Data = []*filterRepresentationRow{{ID: i, Label: "private", Backing: "READY", Code: 10}}
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Payload", "Data"}, Fields: []string{"ID"}}})
	if err != nil {
		b.Fatal(err)
	}
	for _, variant := range []string{"unchanged", "exposed"} {
		var options []structjson.Option
		if variant == "exposed" {
			options = []structjson.Option{structjson.WithRepresentation(filterRepresentation(b, reflect.TypeOf(value), variant))}
		}
		b.Run(variant+"/configure", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := filter.WithOptions(options...); err != nil {
					b.Fatal(err)
				}
			}
		})
		configured, err := filter.WithOptions(options...)
		if err != nil {
			b.Fatal(err)
		}
		options = append(options, structjson.WithPathFieldExcluder(configured))
		b.Run(variant+"/marshal-100", func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := structjson.Marshal(value, options...); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func TestFieldFilterRepresentationExposesHiddenHolder(t *testing.T) {
	type row struct {
		ID    int    `json:"id"`
		Label string `json:"label"`
	}
	type envelope struct {
		Data []row `json:"-"`
	}
	value := envelope{Data: []row{{ID: 2, Label: "private"}}}
	walker, _ := shape.CompileRewrite(reflect.TypeOf(value))
	metadata, err := walker.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Data" {
			return &shape.FieldMetadata{Name: "PublicData", Tag: `json:"roles"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(reflect.TypeOf(value), []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	if err != nil {
		t.Fatal(err)
	}
	configured, err := filter.WithOptions(structjson.WithRepresentation(metadata))
	if err != nil {
		t.Fatal(err)
	}
	for _, encode := range []func(any, ...structjson.Option) ([]byte, error){structjson.Marshal, structjson.MarshalStandard} {
		encoded, err := encode(value, structjson.WithRepresentation(metadata), structjson.WithPathFieldExcluder(configured))
		if err != nil {
			t.Fatal(err)
		}
		assertFilterRepresentationJSON(t, `{"roles":[{"id":2}]}`, encoded)
	}
}

type FilterRepresentationLeft struct {
	Backing string `json:"-"`
}
type FilterRepresentationRight struct {
	Backing string `json:"-"`
}

func TestFieldFilterRepresentationPreservesPromotedNameGuard(t *testing.T) {
	type row struct {
		FilterRepresentationLeft
		FilterRepresentationRight
	}
	typ := reflect.TypeFor[row]()
	walker, _ := shape.CompileRewrite(typ)
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == reflect.TypeFor[FilterRepresentationLeft]() && f.Name == "Backing" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"left"`}, nil
		}
		if parent == reflect.TypeFor[FilterRepresentationRight]() && f.Name == "Backing" {
			return &shape.FieldMetadata{Name: "OtherBacking", Tag: `json:"right"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Indexes: [][]int{{0, 0}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = filter.WithOptions(structjson.WithRepresentation(metadata)); err == nil || !strings.Contains(err.Error(), "cannot distinguish promoted fields") {
		t.Fatalf("canonical source-name collision was not rejected: %v", err)
	}
}

func TestFieldFilterRepresentationPreservesWholeInlineGuard(t *testing.T) {
	type row struct{ ID int }
	type envelope struct {
		Extra struct{ Other int }
		Data  []row
	}
	typ := reflect.TypeFor[envelope]()
	walker, _ := shape.CompileRewrite(typ)
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == typ && f.Name == "Extra" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `jsonx:"inline"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	filter, err := structjson.NewFieldFilter(typ, []structjson.FieldSelection{{Path: []string{"Data"}, Fields: []string{"ID"}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = filter.WithOptions(structjson.WithRepresentation(metadata)); err == nil || !strings.Contains(err.Error(), "whole-value inline") {
		t.Fatalf("effective whole-value inline ancestor was not rejected: %v", err)
	}
}
