package json

import "testing"

func TestNilSlicePolicy_DefaultIsNull(t *testing.T) {
	type sample struct {
		Items []int
	}
	data, err := Marshal(sample{})
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Items":null}`, string(data))
}

func TestNilSlicePolicy_EmptyArrayOverride(t *testing.T) {
	type sample struct {
		Items []int
	}
	data, err := Marshal(sample{}, WithNilSlicePolicy(NilSliceAsEmptyArray))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Items":[]}`, string(data))
}

func TestNilSlicePolicy_NonNilEmptyUnchanged(t *testing.T) {
	type sample struct {
		Items []int
	}
	in := sample{Items: []int{}}

	dataNullPolicy, err := Marshal(in, WithNilSlicePolicy(NilSliceAsNull))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Items":[]}`, string(dataNullPolicy))

	dataEmptyPolicy, err := Marshal(in, WithNilSlicePolicy(NilSliceAsEmptyArray))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `{"Items":[]}`, string(dataEmptyPolicy))
}

func TestNilSlicePolicy_TopLevelSlice(t *testing.T) {
	var in []int
	dataDefault, err := Marshal(in)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `null`, string(dataDefault))

	dataOverride, err := Marshal(in, WithNilSlicePolicy(NilSliceAsEmptyArray))
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}
	assertJSONEqual(t, `[]`, string(dataOverride))
}
