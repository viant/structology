package marshal

import (
	"encoding"
	"encoding/json"
	"reflect"
)

// customWireShape describes only guarantees made by the encoder interface.
// JSON marshalers may emit any JSON value. Value text marshalers emit strings;
// pointer-only methods remain unconstrained because addressability affects dispatch.
// No user method is invoked and no underlying Go fields are inferred.
func customWireShape(t reflect.Type) *WireShape {
	jsonType, textType := reflect.TypeFor[json.Marshaler](), reflect.TypeFor[encoding.TextMarshaler]()
	pointer := reflect.PointerTo(t)
	kind := reflect.Interface
	if !t.Implements(jsonType) && !pointer.Implements(jsonType) && t.Implements(textType) {
		kind = reflect.String
	}
	return &WireShape{source: t, kind: kind, nullable: kind == reflect.Interface, length: -1}
}

func hasStandardWireMarshaler(t reflect.Type) bool {
	p := reflect.PointerTo(t)
	return t.Implements(jsonMarshalerType) || p.Implements(jsonMarshalerType) || t.Implements(textMarshalerType) || p.Implements(textMarshalerType)
}
