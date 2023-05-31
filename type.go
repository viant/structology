package structology

import "reflect"

func ensureStruct(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Struct:
		return t
	case reflect.Ptr:
		return t.Elem()
	case reflect.Slice:
		return t.Elem()
	}
	return nil
}
