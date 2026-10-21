// Package csv provides typed CSV encoding and decoding. Its default dialect uses
// standard double-quote escaping and preserves literal commas and backslashes.
// The initial typed relation traversal was ported from SQLX; SQLX remains unchanged.
package csv

import (
	"fmt"
	"reflect"
	"sync"
)

var defaultMarshallers sync.Map

func defaultMarshaller(t reflect.Type) (*Marshaller, error) {
	if t == nil {
		return nil, fmt.Errorf("CSV type is nil")
	}
	var err error
	t, err = rowType(t)
	if err != nil {
		return nil, err
	}
	if cached, ok := defaultMarshallers.Load(t); ok {
		return cached.(*Marshaller), nil
	}
	marshaller, err := NewMarshaller(t, nil)
	if err != nil {
		return nil, err
	}
	actual, _ := defaultMarshallers.LoadOrStore(t, marshaller)
	return actual.(*Marshaller), nil
}

// Marshal encodes a struct, pointer, array or slice with a header row. Compiled
// default shapes are cached and can be reused concurrently.
func Marshal(value any) ([]byte, error) {
	m, err := defaultMarshaller(reflect.TypeOf(value))
	if err != nil {
		return nil, err
	}
	return m.Marshal(value)
}

// Unmarshal appends decoded records to a nonnil pointer to a typed slice.
func Unmarshal(data []byte, destination any) error {
	m, err := defaultMarshaller(reflect.TypeOf(destination))
	if err != nil {
		return err
	}
	return m.Unmarshal(data, destination)
}
