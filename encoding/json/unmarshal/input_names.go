package unmarshal

import (
	"github.com/viant/tagly/format"
	"github.com/viant/x/shape"
	"reflect"
	"strings"
)

func declaredInputNames(tag reflect.StructTag) (string, error) {
	if !strings.Contains(strings.ToLower(tag.Get("format")), "inputnames=") {
		return "", nil
	}
	policy, err := format.Parse(tag)
	if err != nil {
		return "", err
	}
	return policy.InputNames, nil
}

func hasDeclaredInputNames(typ reflect.Type, representations ...*shape.Representation) bool {
	return hasDeclaredFieldPolicy(typ, func(field reflect.StructField) bool {
		names, err := declaredInputNames(field.Tag)
		return names != "" || err != nil
	}, representations...)
}
