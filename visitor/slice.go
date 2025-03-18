package visitor

import (
	"fmt"
	"reflect"
)

// SliceVisitor implements Visitor[int, interface{}] for []interface{}
type SliceVisitor[E any] struct {
	data []E
}

// SliceVisitorOf creates a Visitor for []interface{}
func SliceVisitorOf[E any](value interface{}) (Visitor[int, E], error) {
	slice, ok := value.([]E)
	if !ok {
		return nil, fmt.Errorf("expected []interface{}, got %T", value)
	}
	visitor := &SliceVisitor[E]{data: slice}
	return visitor.Visit, nil
}

// Visit iterates over the slice, calling the provided function for each element.
// The key is the slice index.
func (sw *SliceVisitor[E]) Visit(f func(key int, element E) (bool, error)) error {
	for i, elem := range sw.data {
		continueVisit, err := f(i, elem)
		if err != nil {
			return err
		}
		if !continueVisit {
			break
		}
	}
	return nil
}

// AnySliceVisitorOf dynamically creates a SliceVisitor from any slice value.
func AnySliceVisitorOf(value interface{}) (Visitor[int, any], error) {
	switch actual := value.(type) {
	case []string:
		return AnyTypedSliceVisitorOf[string](actual), nil
	case []bool:
		return AnyTypedSliceVisitorOf[bool](actual), nil
	case []int:
		return AnyTypedSliceVisitorOf[int](actual), nil
	case []int64:
		return AnyTypedSliceVisitorOf[int64](actual), nil
	case []uint64:
		return AnyTypedSliceVisitorOf[uint64](actual), nil
	case []byte:
		return AnyTypedSliceVisitorOf[byte](actual), nil
	case []interface{}:
		return AnyTypedSliceVisitorOf[interface{}](actual), nil
	case []float64:
		return AnyTypedSliceVisitorOf[float64](actual), nil
	case []float32:
		return AnyTypedSliceVisitorOf[float32](actual), nil
	}
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice, got %T", value)
	}
	visitor := &AnySliceVisitor[any]{data: val}
	return visitor.Visit, nil
}

// AnyTypedSliceVisitorOf return visitor
func AnyTypedSliceVisitorOf[E any](slice []E) Visitor[int, any] {
	return func(f func(key int, element any) (bool, error)) error {
		for i, e := range slice {
			continueVisit, err := f(i, e)
			if err != nil {
				return err
			}
			if !continueVisit {
				break
			}
		}
		return nil
	}
}

// AnySliceVisitor implements Visitor[int, interface{}] for slices of any type.
type AnySliceVisitor[E any] struct {
	data reflect.Value
}

// Visit iterates over any slice type via reflection.
func (v *AnySliceVisitor[E]) Visit(f func(key int, element E) (bool, error)) error {
	for i := 0; i < v.data.Len(); i++ {
		val := v.data.Index(i).Interface().(E)
		continueVisit, err := f(i, val)
		if err != nil {
			return err
		}
		if !continueVisit {
			break
		}
	}
	return nil
}
