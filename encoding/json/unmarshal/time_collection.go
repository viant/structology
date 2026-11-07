package unmarshal

import (
	"bytes"
	"encoding/json"
	"reflect"

	"github.com/viant/x/shape"
)

// Use the existing lexical collection traversal so each typed row reaches the
// native decoder before duplicate members or numeric spellings can be lost.
func (e *Engine) decodeTimePolicyCollection(raw []byte, target reflect.Value, metadata *shape.Representation) error {
	var valid json.RawMessage
	if err := json.Unmarshal(raw, &valid); err != nil {
		return err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return json.Unmarshal(raw, target.Addr().Interface())
	}
	for target.Kind() == reflect.Pointer {
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		metadata = representationElement(metadata, target.Type())
		target = target.Elem()
	}
	if target.Kind() == reflect.Struct {
		return e.WithRepresentation(representationFor(metadata, target.Type())).Unmarshal(raw, target.Addr().Interface())
	}
	return decodeRawCollection(bytes.TrimSpace(raw), target, metadata, e.DuplicateKeyPolicy == ErrorOnDuplicate, func(member []byte, value reflect.Value, child *shape.Representation) error {
		return e.WithRepresentation(child).Unmarshal(member, value.Addr().Interface())
	})
}
