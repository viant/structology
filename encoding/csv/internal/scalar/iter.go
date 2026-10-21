package scalar

import (
	"fmt"
	"reflect"
)

type ValueAccessor = func(int) interface{}

func Values(value interface{}) (ValueAccessor, int, error) {
	v := reflect.ValueOf(value)
	if !v.IsValid() {
		return nil, 0, fmt.Errorf("CSV value is nil")
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return func(int) interface{} { return nil }, 0, nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Struct:
		return func(int) interface{} { return v.Interface() }, 1, nil
	case reflect.Array, reflect.Slice:
		return func(i int) interface{} { return v.Index(i).Interface() }, v.Len(), nil
	}
	return nil, 0, fmt.Errorf("unsupported CSV value %T", value)
}
