package marshal

import (
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"sync"

	xshape "github.com/viant/x/shape"
)

// StandardEncoder projects typed objects without a JSON roundtrip. Scalar,
// opaque custom and map values are delegated to encoding/json. Object field
// authority is x/shape.JSONFields, including JSON-specific promotion dominance.
type StandardEncoder struct {
	representation       *xshape.Representation
	representationFields sync.Map
	exclude              func([]string, string) bool
	excludeIndex         func([]string, []int) bool
}

func NewStandardEncoder(exclude func([]string, string) bool, excludeIndex func([]string, []int) bool) *StandardEncoder {
	return &StandardEncoder{exclude: exclude, excludeIndex: excludeIndex}
}

// WithRepresentation returns an independent encoder over canonical storage.
// The immutable metadata object is also the field-plan cache identity.
func (e *StandardEncoder) WithRepresentation(representation *xshape.Representation) *StandardEncoder {
	return &StandardEncoder{exclude: e.exclude, excludeIndex: e.excludeIndex, representation: representation}
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
	if e.exclude == nil && (e.representation == nil || !e.representation.Changed()) {
		return json.Marshal(value)
	}
	if e.representation != nil && value != nil {
		typ := reflect.TypeOf(value)
		original := e.representation.Original()
		if typ != original && typ != reflect.PointerTo(original) {
			return nil, fmt.Errorf("representation expects %s or its pointer, got %s", original, typ)
		}
	}
	state := standardEncoding{encoder: e, active: map[standardPointer]bool{}}
	return state.append(nil, reflect.ValueOf(value), make([]string, 0, 16), 0, e.representation)
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

func (s *standardEncoding) append(dst []byte, v reflect.Value, path []string, depth int, representation *xshape.Representation) ([]byte, error) {
	if depth > 10000 {
		return nil, fmt.Errorf("JSON value exceeds maximum nesting depth")
	}
	if !v.IsValid() {
		return append(dst, "null"...), nil
	}
	if (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) && v.IsNil() {
		return append(dst, "null"...), nil
	}
	if representation != nil && !representation.Changed() {
		representation = nil
	}
	if custom, ok := s.custom(v); ok {
		return s.scalar(dst, custom)
	}
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Slice || v.Kind() == reflect.Map {
		var pointer uintptr
		if v.Kind() == reflect.Map {
			pointer = uintptr(v.UnsafePointer())
		} else {
			pointer = v.Pointer()
		}
		key := standardPointer{typ: v.Type(), ptr: pointer}
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
		if representation != nil && representation.Original() == v.Type() {
			representation = representation.Elem()
		}
		return s.append(dst, v.Elem(), path, depth+1, representation)
	case reflect.Struct:
		cache := &standardFieldCache
		var key any = v.Type()
		if representation != nil {
			cache = &s.encoder.representationFields
			key = representation
		}
		entry, ok := cache.Load(key)
		if !ok {
			var fields []xshape.JSONField
			var err error
			if representation != nil {
				fields, err = representation.JSONFields()
			} else {
				fields, err = xshape.Linked(v.Type()).JSONFields()
			}
			compiled := make([]standardField, len(fields))
			for i, field := range fields {
				key, _ := json.Marshal(field.Name)
				compiled[i] = standardField{field, key}
			}
			entry, _ = cache.LoadOrStore(key, standardFields{compiled, err})
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
			} else if s.encoder.exclude != nil {
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
				var child *xshape.Representation
				if representation != nil {
					child = representation.FieldShapeByIndex(field.Field.Index)
				}
				dst, err = s.append(dst, fv, next, depth+1, child)
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
		var child *xshape.Representation
		if representation != nil {
			child = representation.Elem()
		}
		dst = append(dst, '[')
		for i := 0; i < v.Len(); i++ {
			if i > 0 {
				dst = append(dst, ',')
			}
			var err error
			dst, err = s.append(dst, v.Index(i), path, depth+1, child)
			if err != nil {
				return nil, err
			}
		}
		return append(dst, ']'), nil
	case reflect.Map:
		if representation == nil || v.IsNil() {
			return s.scalar(dst, v)
		}
		return s.appendMap(dst, v, path, depth, representation.Elem())
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

// Map keys retain encoding/json's string/TextMarshaler/integer precedence and
// sorting. Values alone consume the occurrence-specific metadata.
func (s *standardEncoding) appendMap(dst []byte, v reflect.Value, path []string, depth int, child *xshape.Representation) ([]byte, error) {
	keyType := v.Type().Key()
	switch keyType.Kind() {
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
	default:
		if !keyType.Implements(reflect.TypeFor[encoding.TextMarshaler]()) {
			return nil, &json.UnsupportedTypeError{Type: v.Type()}
		}
	}
	type entry struct {
		name  string
		value reflect.Value
	}
	entries := make([]entry, 0, v.Len())
	iter := v.MapRange()
	for iter.Next() {
		key := iter.Key()
		name := ""
		if key.Kind() == reflect.String {
			name = key.String()
		} else if text, ok := key.Interface().(encoding.TextMarshaler); ok {
			if key.Kind() != reflect.Pointer || !key.IsNil() {
				raw, err := text.MarshalText()
				if err != nil {
					return nil, err
				}
				name = string(raw)
			}
		} else {
			switch key.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				name = strconv.FormatInt(key.Int(), 10)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				name = strconv.FormatUint(key.Uint(), 10)
			default:
				return nil, &json.UnsupportedTypeError{Type: v.Type()}
			}
		}
		entries = append(entries, entry{name, iter.Value()})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })
	dst = append(dst, '{')
	for i, entry := range entries {
		if i > 0 {
			dst = append(dst, ',')
		}
		key, _ := json.Marshal(entry.name)
		dst = append(dst, key...)
		dst = append(dst, ':')
		var err error
		dst, err = s.append(dst, entry.value, path, depth+1, child)
		if err != nil {
			return nil, err
		}
	}
	return append(dst, '}'), nil
}
