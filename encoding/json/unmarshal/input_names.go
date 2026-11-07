package unmarshal

import (
	"github.com/viant/x/shape"
	"reflect"
)

func declaredInputNames(tag reflect.StructTag) (string, error) {
	policy, err := declaredInputPolicy(tag)
	if err != nil || policy == nil {
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
