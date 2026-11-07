package unmarshal

import (
	"reflect"
	"strings"

	"github.com/viant/structology/encoding/json/internal/lru"
	"github.com/viant/x/shape"
)

type selectorEquivalenceKey struct {
	typ            reflect.Type
	representation *shape.Representation
}

var selectorEquivalenceCache = lru.New[selectorEquivalenceKey, bool](2048)

// StandardBoolSelector reports whether the complete destination is an ordinary
// bool selector with decode-only aliases. Options must be checked by the caller.
// Other graphs remain on the native engine; in particular presence holders and
// custom methods must not acquire different ownership through this fast path.
func StandardBoolSelector(typ reflect.Type, representation *shape.Representation) bool {
	if typ == nil || typ.Kind() != reflect.Pointer {
		return false
	}
	typ = typ.Elem()
	key := selectorEquivalenceKey{typ, representation}
	return selectorEquivalenceCache.GetOrCreate(key, func() bool {
		if typ.Kind() != reflect.Struct || hasCustomUnmarshalType(typ) {
			return false
		}
		if representation != nil && representation.Original() != typ {
			return false
		}
		aliased := false
		canonicalNames := map[string]bool{}
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if representation != nil {
				field = representation.Field(i)
			}
			if field.Tag.Get("setMarker") != "" {
				return false
			}
			if field.PkgPath != "" || field.Tag.Get("json") == "-" {
				continue
			}
			if field.Anonymous || field.Type != reflect.TypeFor[bool]() {
				return false
			}
			if field.Tag.Get("internal") != "" || field.Tag.Get("jsonx") != "" || field.Tag.Get("enum") != "" {
				return false
			}
			for _, key := range []string{"choice", "enumStyle", "enumAccept", "enumAlias", "enumCase", "enumInteger", "enumUnknownString"} {
				if _, declared := field.Tag.Lookup(key); declared {
					return false
				}
			}
			parts := strings.Split(field.Tag.Get("json"), ",")
			name := parts[0]
			if name == "" {
				name = field.Name
			}
			if canonicalNames[name] {
				return false
			}
			canonicalNames[name] = true
			for _, option := range parts[1:] {
				if option != "" && option != "omitempty" {
					return false
				}
			}
			policy := field.Tag.Get("format")
			if policy == "" {
				continue
			}
			// No field formatting, scalar policy, or name conversion can be lost.
			if !strings.HasPrefix(policy, "inputNames=") || strings.Contains(policy, ",") {
				return false
			}
			names, err := declaredInputNames(field.Tag)
			if err != nil || names == "" {
				return false
			}
			aliased = true
		}
		return aliased
	})
}
