package marshal

import (
	"encoding"
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	xshape "github.com/viant/x/shape"
)

// Standard describes encoding/json v1 output. It never encodes values and does
// not reuse the transformed engine's field, omission or inline policy.
type Standard struct {
	typeOf reflect.Type
	once   sync.Once
	shape  *WireShape
	err    error
}

func NewStandard(t reflect.Type) *Standard { return &Standard{typeOf: t} }
func (s *Standard) Wire() (*WireShape, error) {
	s.once.Do(func() {
		s.shape, s.err = (&standardCompiler{objects: map[reflect.Type]*WireShape{}, active: map[reflect.Type]bool{}}).value(s.typeOf)
	})
	return s.shape, s.err
}

type standardCompiler struct {
	objects map[reflect.Type]*WireShape
	active  map[reflect.Type]bool
}

func (c *standardCompiler) value(t reflect.Type) (*WireShape, error) {
	if t == nil {
		return nil, fmt.Errorf("standard JSON wire type is required")
	}
	// time.Time has a known standard JSON contract; arbitrary custom encoders do not.
	if t == reflect.TypeFor[time.Time]() {
		return &WireShape{source: t, kind: reflect.String, length: -1, format: "date-time"}, nil
	}
	if !isTimeTypeOrPtr(t) && (t.Implements(reflect.TypeFor[stdjson.Marshaler]()) || reflect.PointerTo(t).Implements(reflect.TypeFor[stdjson.Marshaler]()) || t.Implements(reflect.TypeFor[encoding.TextMarshaler]()) || reflect.PointerTo(t).Implements(reflect.TypeFor[encoding.TextMarshaler]())) {
		return nil, fmt.Errorf("opaque standard JSON encoder for %s has no wire contract", t)
	}
	if t.Kind() == reflect.Pointer {
		element, err := c.value(t.Elem())
		if err != nil {
			return nil, err
		}
		return &WireShape{source: t, kind: reflect.Pointer, nullable: true, length: -1, element: element}, nil
	}
	if prior := c.objects[t]; prior != nil {
		return prior, nil
	}
	if c.active[t] {
		return nil, fmt.Errorf("recursive non-object standard JSON type %s is not representable", t)
	}
	c.active[t] = true
	defer delete(c.active, t)
	result := &WireShape{source: t, kind: t.Kind(), length: -1}
	switch t.Kind() {
	case reflect.Struct:
		c.objects[t] = result
		fields, err := xshape.Linked(t).JSONFields()
		if err != nil {
			return nil, err
		}
		for _, field := range fields {
			if field.Field.Tag.Get("setMarker") == "true" {
				return nil, fmt.Errorf("output presence field %s.%s must be hidden by its JSON contract", t, field.Field.Name)
			}
			property, err := c.value(field.Field.ReflectedType)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", field.Name, err)
			}
			if field.Quoted {
				property = &WireShape{source: field.Field.ReflectedType, kind: reflect.String, nullable: field.Field.ReflectedType.Kind() == reflect.Pointer, length: -1}
			}
			result.properties = append(result.properties, WireProperty{field: field.Field.StructField(), name: field.Name, required: !field.MayOmit(), shape: property})
		}
	case reflect.Slice, reflect.Array:
		result.nullable = t.Kind() == reflect.Slice
		if t.Kind() == reflect.Array {
			result.length = t.Len()
		}
		if t.Kind() == reflect.Slice && t.Elem().Kind() == reflect.Uint8 {
			pointer := reflect.PointerTo(t.Elem())
			if !pointer.Implements(reflect.TypeFor[stdjson.Marshaler]()) && !pointer.Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
				result.kind = reflect.String
				result.format = "byte"
				return result, nil
			}
		}
		element, err := c.value(t.Elem())
		if err != nil {
			return nil, err
		}
		result.element = element
	case reflect.Map:
		switch t.Key().Kind() {
		case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		default:
			if !t.Key().Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
				return nil, fmt.Errorf("unsupported standard JSON map key %s", t.Key())
			}
		}
		element, err := c.value(t.Elem())
		if err != nil {
			return nil, err
		}
		result.element = element
		result.nullable = true
	case reflect.Interface:
		result.nullable = true
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64:
	default:
		return nil, fmt.Errorf("type %s cannot be represented as standard JSON", t)
	}
	return result, nil
}
