package json

import (
	"github.com/viant/x/shape"
	"reflect"
	"testing"
)

type StandardPresenceEmbedded struct {
	Label string `json:"label"`
}
type standardPresenceRow struct {
	*StandardPresenceEmbedded
	Value  int                                  `json:"value"`
	Secret string                               `json:"secret"`
	Has    *struct{ Label, Value, Secret bool } `json:"-" setMarker:"true"`
}

func TestStandardPresenceOptInWithMetadata(t *testing.T) {
	typ := reflect.TypeFor[map[string][][]*standardPresenceRow]()
	walker, err := shape.CompileRewrite(typ)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := walker.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent != reflect.TypeFor[standardPresenceRow]() {
			return nil, nil
		}
		switch f.Name {
		case "Value":
			return &shape.FieldMetadata{Name: "Amount", Tag: `json:"amount"`}, nil
		case "Secret":
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte(`{"a":[[{"label":"ok","amount":0,"secret":"hidden"}]]}`)
	for _, tracking := range []bool{false, true} {
		var result map[string][][]*standardPresenceRow
		if err = UnmarshalStandard(raw, &result, WithRepresentation(metadata), WithPresenceTracking(tracking)); err != nil {
			t.Fatal(err)
		}
		row := result["a"][0][0]
		if row.Value != 0 || row.Secret != "" || row.Label != "ok" {
			t.Fatalf("%+v", row)
		}
		if tracking {
			if row.Has == nil || !row.Has.Value || !row.Has.Label || row.Has.Secret {
				t.Fatalf("markers: %+v", row.Has)
			}
		} else if row.Has != nil {
			t.Fatal("presence must remain opt-in")
		}
	}
	var plain standardPresenceRow
	if err = UnmarshalStandard([]byte(`{"value":null}`), &plain, WithPresenceTracking(true)); err != nil {
		t.Fatal(err)
	}
	if plain.Has == nil || !plain.Has.Value {
		t.Fatal("explicit null not marked")
	}
	var absent standardPresenceRow
	if err = UnmarshalStandard([]byte(`{}`), &absent, WithPresenceTracking(true)); err != nil {
		t.Fatal(err)
	}
	if absent.Has != nil {
		t.Fatal("absent field marked")
	}
}
