package visitor

import (
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"unsafe"
)

var structCache = NewSyncMap[reflect.Type, *xunsafe.Struct]()

// StructVisitor implements Visitor[string, interface{}] for structs using reflection.
type StructVisitor struct {
	value   interface{}
	ptr     unsafe.Pointer
	xStruct *xunsafe.Struct
}

// StructVisitorOf creates a StructVisitor from any struct value.
func StructVisitorOf(value interface{}) (Visitor[string, interface{}], error) {
	valueType := reflect.TypeOf(value)
	isPtr := false
	var structType reflect.Type
	switch valueType.Kind() {
	case reflect.Ptr:

		isPtr = true
		structType = valueType.Elem()
	case reflect.Struct:
		structType = valueType
	default:
		return nil, fmt.Errorf("expected struct or pointer to struct, got %T", value)
	}

	if !isPtr {
		rPointer := reflect.New(structType)
		rPointer.Elem().Set(reflect.ValueOf(value))
		value = rPointer.Interface()
	}
	xStruct, ok := structCache.Get(structType)
	if !ok {
		xStruct = xunsafe.NewStruct(structType)
		structCache.Put(structType, xStruct)
	}
	visitor := &StructVisitor{
		value:   value,
		ptr:     xunsafe.AsPointer(value),
		xStruct: xStruct,
	}
	return visitor.Visit, nil
}

// Visit iterates over struct fields, calling the provided function with each field name and value.
func (w *StructVisitor) Visit(f func(key string, element interface{}) (bool, error)) error {
	for i := 0; i < len(w.xStruct.Fields); i++ {
		xField := w.xStruct.Fields[i]
		fieldValue := xField.Value(w.ptr)
		continueVisit, err := f(xField.Name, fieldValue)
		if err != nil {
			return err
		}
		if !continueVisit {
			break
		}
	}
	return nil
}
