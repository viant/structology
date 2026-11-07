package json

import (
	"testing"

	"github.com/viant/tagly/format/text"
)

func TestNativeInputNamesRejectFoldedTransformedCollision(t *testing.T) {
	var value struct {
		First      bool `format:"inputNames=OTHER_VALUE"`
		OtherValue bool
	}
	err := Unmarshal([]byte(`{"OTHER_VALUE":true}`), &value, WithCaseFormat(text.CaseFormatLowerUnderscore))
	if err == nil {
		t.Fatalf("ambiguous alias accepted: %+v", value)
	}
	if value.First || value.OtherValue {
		t.Fatalf("ambiguous metadata mutated destination: %+v", value)
	}
}
