package structology

import (
	"reflect"
	"strings"
)

const (
	//SetMarkerTag defines set marker tag
	SetMarkerTag = "setMarker"

	legacyMarkerTag = "presenceIndex"

	legacyTagFragment = "presence=true"
)

func IsSetMarker(tag reflect.StructTag) bool {
	if _, ok := tag.Lookup(SetMarkerTag); ok {
		return true
	}
	if _, ok := tag.Lookup(legacyMarkerTag); ok {
		return true
	}
	return strings.Contains(string(tag), legacyTagFragment)
}
