package json

import (
	"context"
	"reflect"
	"sync"
	"testing"

	jsonunmarshal "github.com/viant/structology/encoding/json/unmarshal"
	"github.com/viant/x/shape"
)

type RepresentationEmbedded struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func (*RepresentationEmbedded) Identity() string { return "linked" }

type representationFlags struct{ Value, Private bool }
type representationRecord struct {
	*RepresentationEmbedded
	Value   int                  `json:"value"`
	Private string               `json:"private"`
	Has     *representationFlags `setMarker:"true" json:"-"`
}
type representationEnvelope struct {
	Rows   []*representationRecord          `json:"rows"`
	ByName map[string]*representationRecord `json:"byName"`
}

func representationDecoder() *jsonunmarshal.Engine {
	return jsonunmarshal.New(context.Background(), scalarScannerHooks{}, jsonunmarshal.IgnoreUnknown, jsonunmarshal.CoerceNumbers, jsonunmarshal.CompatNulls, jsonunmarshal.LastWins, jsonunmarshal.Tolerant, "", "", nil, nil)
}
func TestDecoderRepresentationPreservesEmbeddedStorageAndPresence(t *testing.T) {
	typ := reflect.TypeFor[representationEnvelope]()
	plan, _ := shape.CompileRewrite(typ)
	effective, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Private" || f.Name == "Secret" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		if f.Name == "Value" {
			return &shape.FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	decoder := representationDecoder().WithRepresentation(effective)
	raw := []byte(`{"rows":[{"name":"public","secret":"blocked","private":"blocked","visible":0},null],"byName":{"a":{"visible":7},"nil":null}}`)
	check := func(t *testing.T) {
		var value representationEnvelope
		if err := decoder.Unmarshal(raw, &value); err != nil {
			t.Error(err)
			return
		}
		row := value.Rows[0]
		if row.RepresentationEmbedded == nil || row.Name != "public" || row.Identity() != "linked" || row.Secret != "" || row.Private != "" {
			t.Errorf("canonical embedded value lost: %+v", row)
		}
		if row.Has == nil || !row.Has.Value || row.Has.Private {
			t.Errorf("canonical presence lost: %+v", row.Has)
		}
		if value.Rows[1] != nil || value.ByName["nil"] != nil || value.ByName["a"].Value != 7 {
			t.Error("container representation lost")
		}
	}
	check(t)
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() { defer group.Done(); check(t) }()
	}
	group.Wait()
	// Effective names do not contaminate ordinary type-plan caches.
	var ordinary representationRecord
	if err := representationDecoder().Unmarshal([]byte(`{"value":3,"private":"original"}`), &ordinary); err != nil {
		t.Fatal(err)
	}
	if ordinary.Value != 3 || ordinary.Private != "original" {
		t.Fatal("base plan contaminated")
	}
}
func TestDecoderRepresentationFastRoot(t *testing.T) {
	plan, _ := shape.CompileRewrite(reflect.TypeFor[representationRecord]())
	effective, err := plan.RewriteRepresentation(func(_ string, _ reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Value" {
			return &shape.FieldMetadata{Name: "Renamed", Tag: `json:"visible"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	decoder := representationDecoder().WithRepresentation(effective)
	var value representationRecord
	if err := decoder.Unmarshal([]byte(`{"name":"embedded","visible":0,"value":99}`), &value); err != nil {
		t.Fatal(err)
	}
	if value.Name != "embedded" || value.Value != 0 || value.Has == nil || !value.Has.Value {
		t.Fatalf("fast root lost metadata/presence: %+v", value)
	}
}
