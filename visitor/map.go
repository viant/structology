package visitor

import (
	"fmt"
	"reflect"
)

// MapVisitor holds a map of type map[K]E and implements the Visitor interface.
type MapVisitor[K comparable, E any] struct {
	data map[K]E
}

// MapVisitorOf creates a new MapVisitor from an interface{}.
// It must be map[K]E, or an error is returned.
func MapVisitorOf[K comparable, E any](aMap map[K]E) Visitor[K, E] {
	visitor := &MapVisitor[K, E]{data: aMap}
	return visitor.Visit
}

// Visit iterates over the map and calls f for each (key, element).
// - If f returns (true, nil), iteration continues.
// - If f returns (false, nil), iteration stops early.
// - If f returns an error, iteration stops with that error.
func (v *MapVisitor[K, E]) Visit(f func(key K, element E) (bool, error)) error {
	for k, e := range v.data {
		continueVisit, err := f(k, e)
		if err != nil {
			return err
		}
		if !continueVisit {
			break
		}
	}
	return nil
}

// AnyMapVisitorOf dynamically creates a MapVisitor from any map value.
func AnyMapVisitorOf(value interface{}) (Visitor[any, any], error) {
	switch actual := value.(type) {
	case map[string]bool:
		return AnyTypedMapVisitorOf[string, bool](actual), nil
	case map[string]interface{}:
		return AnyTypedMapVisitorOf[string, interface{}](actual), nil
	case map[string]int:
		return AnyTypedMapVisitorOf[string, int](actual), nil
	case map[string]string:
		return AnyTypedMapVisitorOf[string, string](actual), nil
	case map[int]string:
		return AnyTypedMapVisitorOf[int, string](actual), nil
	case map[int]interface{}:
		return AnyTypedMapVisitorOf[int, interface{}](actual), nil
	}
	val := reflect.ValueOf(value)
	if val.Kind() != reflect.Map {
		return nil, fmt.Errorf("expected map, got %T", value)
	}
	visitor := &AnyMapVisitor[any, any]{data: val}
	return visitor.Visit, nil
}

// AnyTypedMapVisitorOf return
func AnyTypedMapVisitorOf[K comparable, V any](aMap map[K]V) Visitor[any, any] {
	return func(f func(key any, element any) (bool, error)) error {
		for k, e := range aMap {
			continueVisit, err := f(k, e)
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

// AnyMapVisitor defins any map visitor
type AnyMapVisitor[K comparable, E any] struct {
	data reflect.Value
}

// Visit iterates over the map via reflection and calls f for each entry.
func (v *AnyMapVisitor[K, E]) Visit(f func(key K, element E) (bool, error)) error {
	for _, key := range v.data.MapKeys() {
		val := v.data.MapIndex(key)

		// Type assertion to concrete types
		k, ok1 := key.Interface().(K)
		e, ok2 := val.Interface().(E)

		if !ok1 || !ok2 {
			return fmt.Errorf("type assertion failed for key or element")
		}

		continueVisit, err := f(k, e)
		if err != nil {
			return err
		}
		if !continueVisit {
			break
		}
	}
	return nil
}
