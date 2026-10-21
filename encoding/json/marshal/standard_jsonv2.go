//go:build goexperiment.jsonv2

package marshal

import (
	"reflect"
	"strings"
)

// encoding/json backed by json/v2 applies the declared ,string option to a
// named pointer to a primitive scalar. JSON v1 did not. Keep this distinction
// at the standard serializer boundary, not in downstream schema consumers.
func standardQuotedNamedPointer(t reflect.Type, tag reflect.StructTag) bool {
	if t.Kind() != reflect.Pointer || t.Name() == "" {
		return false
	}
	switch t.Elem().Kind() {
	case reflect.Bool, reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
	default:
		return false
	}
	_, options, _ := strings.Cut(tag.Get("json"), ",")
	for _, option := range strings.Split(options, ",") {
		if option == "string" {
			return true
		}
	}
	return false
}
