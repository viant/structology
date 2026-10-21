package scalar

import (
	"reflect"
	"strings"
)

func EnsureDereference(value interface{}) reflect.Type {
	t := reflect.TypeOf(value)
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t
}

type fieldTag struct{ Transient, NullifyEmpty bool }

func ParseTag(tag reflect.StructTag) fieldTag {
	result := fieldTag{Transient: tag.Get("setMarker") == "true"}
	for _, part := range strings.Split(tag.Get("sqlx"), ",") {
		if part == "-" {
			result.Transient = true
		}
		if part == "nullifyEmpty" || part == "nullifyEmpty=true" {
			result.NullifyEmpty = true
		}
	}
	return result
}
