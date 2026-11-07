package unmarshal

import (
	"reflect"
	"strings"
	"time"

	"github.com/viant/tagly/format"
	"github.com/viant/x/shape"
)

func declaredTimePolicy(field reflect.StructField) (*format.Tag, error) {
	if !isTimeTypeOrPtr(field.Type) {
		return nil, nil
	}
	raw := strings.ToLower(field.Tag.Get("format"))
	if !strings.Contains(raw, "timeinputlayouts=") && !strings.Contains(raw, "timeepoch=") && !strings.Contains(raw, "timeempty=") {
		return nil, nil
	}
	policy, err := format.Parse(field.Tag)
	return policy, err
}

func assignDeclaredTime(target reflect.Value, policy *format.Tag, raw []byte) error {
	stamp, action, err := policy.ParseTimeJSON(raw)
	if err != nil {
		return err
	}
	if target.Kind() == reflect.Pointer {
		if action == format.TimeNull {
			target.SetZero()
			return nil
		}
		if target.IsNil() {
			target.Set(reflect.New(target.Type().Elem()))
		}
		target = target.Elem()
	}
	if action == format.TimeAssign {
		target.Set(reflect.ValueOf(stamp))
	}
	return nil
}

// Standard decoding stays on encoding/json for ordinary subtrees. Traverse only
// types that explicitly declare this native scalar policy, retaining lexical
// members in the existing standard decoder rather than collapsing a body map.
func hasDeclaredTimePolicy(typ reflect.Type, representations ...*shape.Representation) bool {
	return hasDeclaredFieldPolicy(typ, func(field reflect.StructField) bool {
		policy, err := declaredTimePolicy(field)
		return policy != nil || err != nil
	}, representations...)
}

func hasDeclaredFieldPolicy(typ reflect.Type, matches func(reflect.StructField) bool, representations ...*shape.Representation) bool {
	type key struct {
		typ      reflect.Type
		metadata *shape.Representation
	}
	seen := map[key]bool{}
	var visit func(reflect.Type, *shape.Representation) bool
	visit = func(typ reflect.Type, metadata *shape.Representation) bool {
		metadata = representationFor(metadata, typ)
		k := key{typ, metadata}
		if typ == nil || seen[k] {
			return false
		}
		seen[k] = true
		if typ == reflect.TypeFor[time.Time]() {
			return false
		}
		switch typ.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			return visit(typ.Elem(), representationElement(metadata, typ))
		case reflect.Struct:
			for i := 0; i < typ.NumField(); i++ {
				field := typ.Field(i)
				var child *shape.Representation
				if metadata != nil {
					field = metadata.Field(i)
					child = metadata.FieldShape(i)
				}
				if field.PkgPath != "" || field.Tag.Get("json") == "-" || field.Tag.Get("internal") == "true" {
					continue
				}
				if matches(field) {
					return true
				}
				if visit(field.Type, child) {
					return true
				}
			}
		}
		return false
	}
	var metadata *shape.Representation
	if len(representations) > 0 {
		metadata = representations[0]
	}
	return visit(typ, metadata)
}
