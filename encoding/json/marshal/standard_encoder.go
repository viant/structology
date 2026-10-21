package marshal

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	xshape "github.com/viant/x/shape"
)

// StandardEncoder projects typed objects without a JSON roundtrip. Scalar,
// opaque custom and map values are delegated to encoding/json. Object field
// authority is x/shape.JSONFields, including JSON-specific promotion dominance.
type StandardEncoder struct {
	exclude      func([]string, string) bool
	excludeIndex func([]string, []int) bool
}

func NewStandardEncoder(exclude func([]string, string) bool, excludeIndex func([]string, []int) bool) *StandardEncoder {
	return &StandardEncoder{exclude: exclude, excludeIndex: excludeIndex}
}

type standardField struct {
	xshape.JSONField
	key []byte
}
type standardFields struct {
	fields []standardField
	err    error
}

var standardFieldCache sync.Map

func (e *StandardEncoder) Marshal(value any) ([]byte, error) {
	if e.exclude == nil {
		return json.Marshal(value)
	}
	state := standardEncoding{encoder: e, active: map[standardPointer]bool{}}
	return state.append(nil, reflect.ValueOf(value), make([]string, 0, 16), 0)
}

type standardPointer struct {
	typ    reflect.Type
	ptr    uintptr
	length int
}
type standardEncoding struct {
	encoder *StandardEncoder
	active  map[standardPointer]bool
}

func (s *standardEncoding) append(dst []byte, v reflect.Value, path []string, depth int) ([]byte, error) {
	if depth > 10000 {
		return nil, fmt.Errorf("JSON value exceeds maximum nesting depth")
	}
	if !v.IsValid() {
		return append(dst, "null"...), nil
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return append(dst, "null"...), nil
	}
	if custom, ok := s.custom(v); ok {
		return s.scalar(dst, custom)
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice {
		key := standardPointer{typ: v.Type(), ptr: v.Pointer()}
		if v.Kind() == reflect.Slice {
			key.length = v.Len()
		}
		if s.active[key] {
			return nil, &json.UnsupportedValueError{Value: v, Str: "encountered a cycle via " + v.Type().String()}
		}
		s.active[key] = true
		defer delete(s.active, key)
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		return s.append(dst, v.Elem(), path, depth+1)
	case reflect.Struct:
		entry, ok := standardFieldCache.Load(v.Type())
		if !ok {
			fields, err := xshape.Linked(v.Type()).JSONFields()
			compiled := make([]standardField, len(fields))
			for i, field := range fields {
				key, _ := json.Marshal(field.Name)
				compiled[i] = standardField{field, key}
			}
			entry, _ = standardFieldCache.LoadOrStore(v.Type(), standardFields{compiled, err})
		}
		fields := entry.(standardFields)
		if fields.err != nil {
			return nil, fields.err
		}
		dst = append(dst, '{')
		count := 0
		for _, field := range fields.fields {
			excluded := false
			if s.encoder.excludeIndex != nil {
				excluded = s.encoder.excludeIndex(path, field.Field.Index)
			} else {
				excluded = s.encoder.exclude(path, field.Field.Name)
			}
			if excluded {
				continue
			}
			fv, err := v.FieldByIndexErr(field.Field.Index)
			if err != nil {
				continue
			} // nil promoted pointer holder
			if field.OmitEmpty && s.empty(fv) || field.OmitZero && s.zero(fv) {
				continue
			}
			if count > 0 {
				dst = append(dst, ',')
			}
			count++
			dst = append(dst, field.key...)
			dst = append(dst, ':')
			quoted := field.Quoted
			if quoted {
				_, opaque := s.custom(fv)
				quoted = !opaque
			}
			if quoted {
				encoded, err := json.Marshal(fv.Interface())
				if err != nil {
					return nil, err
				}
				if string(encoded) == "null" {
					dst = append(dst, encoded...)
				} else {
					quoted, _ := json.Marshal(string(encoded))
					dst = append(dst, quoted...)
				}
			} else {
				next := append(path, field.Name)
				dst, err = s.append(dst, fv, next, depth+1)
				if err != nil {
					return nil, err
				}
			}
		}
		return append(dst, '}'), nil
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && (v.IsNil() || v.Type().Elem().Kind() == reflect.Uint8) {
			return s.scalar(dst, v)
		}
		dst = append(dst, '[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = s.append(dst, v.Index(i), path, depth+1)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	default:
		return s.scalar(dst, v)
	}
}
func (*standardEncoding) scalar(dst []byte, v reflect.Value) ([]byte, error) {
	encoded, err := json.Marshal(v.Interface())
	if err != nil {
		return nil, err
	}
	return append(dst, encoded...), nil
}
func (*standardEncoding) empty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64, reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}
	return false
}
func (*standardEncoding) zero(v reflect.Value) bool {
	contract := reflect.TypeFor[interface{ IsZero() bool }]()
	if v.Type().Implements(contract) {
		if (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) && v.IsNil() {
			return true
		}
		if v.Kind() == reflect.Interface && v.Elem().Kind() == reflect.Pointer && v.Elem().IsNil() {
			return true
		}
		return v.Interface().(interface{ IsZero() bool }).IsZero()
	}
	if v.Kind() != reflect.Pointer && reflect.PointerTo(v.Type()).Implements(contract) {
		if !v.CanAddr() {
			copy := reflect.New(v.Type()).Elem()
			copy.Set(v)
			v = copy
		}
		return v.Addr().Interface().(interface{ IsZero() bool }).IsZero()
	}
	return v.IsZero()
}

func (*standardEncoding) custom(v reflect.Value) (reflect.Value, bool) {
	// encoding/json checks the addressable pointer JSON method before a value
	// TextMarshaler. Type checks avoid boxing every ordinary scalar field.
	typ := v.Type()
	addressable := v.Kind() != reflect.Pointer && v.CanAddr() && v.Addr().CanInterface()
	if addressable && reflect.PointerTo(typ).Implements(jsonMarshalerType) {
		return v.Addr(), true
	}
	if v.CanInterface() && typ.Implements(jsonMarshalerType) {
		return v, true
	}
	if addressable && reflect.PointerTo(typ).Implements(textMarshalerType) {
		return v.Addr(), true
	}
	if v.CanInterface() && typ.Implements(textMarshalerType) {
		return v, true
	}
	return reflect.Value{}, false
}
