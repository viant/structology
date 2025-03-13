package structology

import (
	"reflect"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

var timePtrType = reflect.PtrTo(timeType)

func isTimeType(candidate reflect.Type) bool {
	return EnsureStructType(candidate) == timeType
}

func EnsureStructType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Struct:
		return t
	case reflect.Ptr:
		return EnsureStructType(t.Elem())
	case reflect.Slice:
		return EnsureStructType(t.Elem())
	}
	return nil
}

func EnsureSliceType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Slice:
		return t
	case reflect.Ptr:
		return EnsureSliceType(t.Elem())
	case reflect.Struct:
		return nil
	}
	return nil
}
