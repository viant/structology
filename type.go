package structology

import (
	"reflect"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

var timePtrType = reflect.PointerTo(timeType)

func isTimeType(candidate reflect.Type) bool {
	return EnsureStructType(candidate) == timeType
}

// EnsureStructType returns struct type of given value
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

// EnsureSliceType returns slice type of given value
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

// StructTypeOf returns struct type of given value or nil
func StructTypeOf(v interface{}) reflect.Type {
	return EnsureStructType(reflect.TypeOf(v))
}

// EnsureInterfaceType returns interface type of given value
func EnsureInterfaceType(t reflect.Type) reflect.Type {
	switch t.Kind() {
	case reflect.Interface:
		return t
	case reflect.Ptr:
		return EnsureInterfaceType(t.Elem())
	case reflect.Slice:
		return EnsureInterfaceType(t.Elem())
	}
	return nil
}

// InterfaceTypeOf returns interface type of given value or nil
func InterfaceTypeOf(v interface{}) reflect.Type {
	return EnsureInterfaceType(reflect.TypeOf(v))
}
