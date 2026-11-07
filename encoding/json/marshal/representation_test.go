package marshal

import (
	"encoding/json"
	"reflect"
	"sync"
	"testing"

	"github.com/viant/x/shape"
)

type RepresentationEmbedded struct {
	Name   string `json:"name"`
	Secret string `json:"secret"`
}

func (*RepresentationEmbedded) Identity() string { return "linked" }

type representationRecord struct {
	*RepresentationEmbedded
	Value   int    `json:"value"`
	Private string `json:"private"`
}
type representationEnvelope struct {
	Rows []*representationRecord       `json:"rows"`
	ByID map[int]*representationRecord `json:"byID"`
}

func TestStandardEncoderRepresentationKeepsLinkedEmbedding(t *testing.T) {
	typ := reflect.TypeFor[representationEnvelope]()
	plan, _ := shape.CompileRewrite(typ)
	effective, err := plan.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if f.Name == "Private" {
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
	record := &representationRecord{RepresentationEmbedded: &RepresentationEmbedded{Name: "public", Secret: "unchanged"}, Value: 7, Private: "hidden"}
	value := &representationEnvelope{Rows: []*representationRecord{record, nil, {Value: 2}}, ByID: map[int]*representationRecord{10: record, 2: nil}}
	encoder := NewStandardEncoder(nil, nil).WithRepresentation(effective)
	encoded, err := encoder.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rows":[{"name":"public","secret":"unchanged","visible":7},null,{"visible":2}],"byID":{"10":{"name":"public","secret":"unchanged","visible":7},"2":null}}`
	if string(encoded) != want {
		t.Fatalf("%s", encoded)
	}
	if record.Private != "hidden" || record.Identity() != "linked" {
		t.Fatal("canonical instance/method altered")
	}
	plain, err := NewStandardEncoder(nil, nil).Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	original, _ := json.Marshal(value)
	if string(plain) != string(original) {
		t.Fatal("base encoder contaminated")
	}
	var group sync.WaitGroup
	for i := 0; i < 10; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			actual, e := encoder.Marshal(value)
			if e != nil || string(actual) != want {
				t.Errorf("concurrent variant: %s %v", actual, e)
			}
		}()
	}
	group.Wait()
}
func TestStandardEncoderRepresentationRenamesPromotedField(t *testing.T) {
	plan, _ := shape.CompileRewrite(reflect.TypeFor[representationRecord]())
	effective, err := plan.RewriteRepresentation(func(_ string, parent reflect.Type, f *shape.FieldMetadata) (*shape.FieldMetadata, error) {
		if parent == reflect.TypeFor[RepresentationEmbedded]() && f.Name == "Secret" {
			return &shape.FieldMetadata{Name: f.Name, Tag: `json:"-"`}, nil
		}
		return nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	value := &representationRecord{RepresentationEmbedded: &RepresentationEmbedded{Name: "ok", Secret: "hidden"}}
	encoded, err := NewStandardEncoder(nil, nil).WithRepresentation(effective).Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `{"name":"ok","value":0,"private":""}` {
		t.Fatal(string(encoded))
	}
}
