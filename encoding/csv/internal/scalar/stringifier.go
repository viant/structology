package scalar

import (
	"fmt"
	format "github.com/viant/tagly/format/text"
	"github.com/viant/xunsafe"
	"reflect"
	"unsafe"
)

type (
	ObjectStringifier struct {
		fields   []*fieldStringifier
		index    map[string]int
		parallel bool
	}

	//ObjectStringifierFn returns stringified object properties values and information if value was string before
	ObjectStringifierFn func(val interface{}) ([]string, []bool)
	fieldStringifier    struct {
		stringify FieldStringifierFn
		fieldName string
	}

	FieldStringifierFn = func(pointer unsafe.Pointer) (string, bool)

	// StringifierConfig represents stringifier config
	StringifierConfig struct {
		Fields                   []string
		CaseFormat               format.CaseFormat
		StringifierFloat32Config StringifierFloat32Config
		StringifierFloat64Config StringifierFloat64Config
	}

	// StringifierFloat32Config represents stringifier float32 config
	StringifierFloat32Config struct {
		Precision string
	}

	// StringifierFloat64Config represents stringifier float64 config
	StringifierFloat64Config struct {
		Precision string
	}

	Parallel bool
)

func (s *ObjectStringifier) Has(fieldName string) bool {
	_, ok := s.index[fieldName]
	return ok
}

func (s *ObjectStringifier) Stringifier(options ...interface{}) (ObjectStringifierFn, error) {
	stringifierConfig := s.readOptions(options)

	if len(stringifierConfig.Fields) == 0 && stringifierConfig.CaseFormat == format.CaseFormatUpperCamel && !s.parallel {
		stringifiersLen := len(s.fields)
		strings := make([]string, stringifiersLen)
		wasStrings := make([]bool, stringifiersLen)

		return func(val interface{}) ([]string, []bool) {
			ptr := xunsafe.AsPointer(val)
			for i := 0; i < stringifiersLen; i++ {
				strings[i], wasStrings[i] = s.fields[i].stringify(ptr)
			}

			return strings, wasStrings
		}, nil
	}

	if len(stringifierConfig.Fields) == 0 {
		stringifiersLen := len(s.fields)
		return func(val interface{}) ([]string, []bool) {
			ptr := xunsafe.AsPointer(val)
			strings := make([]string, stringifiersLen)
			wasStrings := make([]bool, stringifiersLen)

			for i := 0; i < stringifiersLen; i++ {
				strings[i], wasStrings[i] = s.fields[i].stringify(ptr)
			}

			return strings, wasStrings
		}, nil
	}

	actualFieldsIndex := make([]int, 0, len(stringifierConfig.Fields))
	for _, field := range stringifierConfig.Fields {
		fieldIndex, ok := s.index[field]
		if !ok {
			return nil, fmt.Errorf("not found field %v at index %v", field, s.fields)
		}

		actualFieldsIndex = append(actualFieldsIndex, fieldIndex)
	}

	stringifiersLen := len(actualFieldsIndex)

	return func(val interface{}) ([]string, []bool) {
		strings := make([]string, stringifiersLen)
		wasStrings := make([]bool, stringifiersLen)
		ptr := xunsafe.AsPointer(val)

		for i := 0; i < stringifiersLen; i++ {
			strings[i], wasStrings[i] = s.fields[actualFieldsIndex[i]].stringify(ptr)
		}

		return strings, wasStrings
	}, nil
}

func (s *ObjectStringifier) readOptions(options []interface{}) *StringifierConfig {
	config := &StringifierConfig{
		CaseFormat: format.CaseFormatUpperCamel,
	}

	for _, anOption := range options {
		switch actual := anOption.(type) {
		case *StringifierConfig:
			config = actual
		case StringifierConfig:
			config = &actual
		}
	}

	return config
}

func (s *ObjectStringifier) FieldNames() []string {
	fieldNames := make([]string, 0, len(s.fields))
	for _, field := range s.fields {
		fieldNames = append(fieldNames, field.fieldName)
	}

	return fieldNames
}

// TypeStringifier returns ObjectStringifier for a given Type.
// It will replace nil values with nullValue for properties with tag: "nullifyEmpty" and omit (if specified) transient properties
// By default, results are shared, no new arrays are returned unless Parallel(true) is provided as an option.
func TypeStringifier(rType reflect.Type, nullValue string, omitTransient bool, options ...interface{}) *ObjectStringifier {
	fieldLen := rType.NumField()
	parallel := false
	for _, anOption := range options {
		switch actual := anOption.(type) {
		case Parallel:
			parallel = bool(actual)
		}
	}

	stringifiers := make([]*fieldStringifier, 0)

	for i := 0; i < fieldLen; i++ {
		field := rType.Field(i)
		if field.PkgPath != "" || field.Tag.Get("csvName") == "-" {
			continue
		}
		tag := ParseTag(field.Tag)
		if tag.Transient && omitTransient {
			continue
		}

		stringifiers = append(stringifiers, &fieldStringifier{
			stringify: stringifierEnclosured(xunsafe.NewField(field), tag.NullifyEmpty, nullValue, options...),
			fieldName: field.Name,
		})
	}

	fieldsIndex := map[string]int{}
	for i, aStringifier := range stringifiers {
		fieldsIndex[aStringifier.fieldName] = i
	}

	o := &ObjectStringifier{
		fields:   stringifiers,
		index:    fieldsIndex,
		parallel: parallel,
	}

	return o
}

func stringifierEnclosured(xField *xunsafe.Field, nullifyEmpty bool, nullValue string, options ...interface{}) FieldStringifierFn {
	preparedStringifier := Stringifier(xField, nullifyEmpty, nullValue, options...)
	return func(val unsafe.Pointer) (value string, wasString bool) {
		return preparedStringifier(val)
	}
}

func Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, options ...interface{}) FieldStringifierFn {
	wasPointer := field.Type.Kind() == reflect.Ptr
	var rType reflect.Type
	if wasPointer {
		rType = field.Elem()
	} else {
		rType = field.Type
	}

	switch rType.Kind() {
	case reflect.String:
		return stringStringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Int:
		return intStringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Int8:
		return int8Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Int16:
		return int16Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Int32:
		return int32Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Int64:
		return int64Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Uint:
		return uintStringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Uint8:
		return uint8Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Uint16:
		return uint16Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Uint32:
		return uint32Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Uint64:
		return uint64Stringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Bool:
		return boolStringifier(field, nullifyZeroValue, nullValue, wasPointer)
	case reflect.Float64:
		return float64Stringifier(field, nullifyZeroValue, nullValue, wasPointer, options...)
	case reflect.Float32:
		return float32Stringifier(field, nullifyZeroValue, nullValue, wasPointer, options...)
	case reflect.Interface:
		return interfaceStringifier(field, nullifyZeroValue, nullValue, wasPointer)
	default:
		return defaultStringifier(field, nullifyZeroValue, nullValue)
	}
}
