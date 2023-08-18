package structology

import (
	"encoding/json"
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type (
	converter struct {
		inputType reflect.Type
		setter    setter
	}
	setter func(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error
)

func timeToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(time.Time)
	field.SetString(structPtr, value.Format(time.RFC3339))
	return nil
}

func timePtrToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(*time.Time)
	if value == nil {
		field.SetString(structPtr, "")
		return nil
	}
	field.SetString(structPtr, value.Format(time.RFC3339))
	return nil
}

func stringToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(string)
	field.SetString(structPtr, value)
	return nil
}

func intToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int)(ptr)
	field.SetString(structPtr, strconv.Itoa(value))
	return nil
}

func float64ToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float64)
	field.SetString(structPtr, strconv.FormatFloat(value, 'f', -1, 64))
	return nil
}

func float32ToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float32)
	field.SetString(structPtr, strconv.FormatFloat(float64(value), 'f', -1, 32))
	return nil
}

func boolToString(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(bool)
	field.SetString(structPtr, strconv.FormatBool(value))
	return nil
}

func stringToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(string)
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return err
	}
	field.SetInt(structPtr, intValue)
	return nil
}

func intToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int)(ptr)
	field.SetInt(structPtr, value)
	return nil
}
func int8ToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int8)(ptr)
	field.SetInt(structPtr, int(value))
	return nil
}

func int16ToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int16)(ptr)
	field.SetInt(structPtr, int(value))
	return nil
}

func int32ToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int16)(ptr)
	field.SetInt(structPtr, int(value))
	return nil
}

func float64ToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float64)
	field.SetInt(structPtr, int(value))
	return nil
}

func float32ToInt(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float32)
	field.SetInt(structPtr, int(value))
	return nil
}

func stringToBool(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(string)
	parseBool, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	field.SetBool(structPtr, parseBool)
	return nil
}

func intToBool(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int)(ptr)
	field.SetBool(structPtr, value != 0)
	return nil
}

func boolToBool(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(bool)
	field.SetBool(structPtr, value)
	return nil
}

func float64ToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float64)
	field.SetFloat64(structPtr, value)
	return nil
}

func float32ToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float32)
	field.SetFloat64(structPtr, float64(value))
	return nil
}

func stringToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(string)
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return err
	}
	field.SetFloat64(structPtr, f)
	return nil
}

func intToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int)(ptr)
	field.SetFloat64(structPtr, float64(value))
	return nil
}

func int32ToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int32)(ptr)
	field.SetFloat64(structPtr, float64(value))
	return nil
}

func int16ToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int16)(ptr)
	field.SetFloat64(structPtr, float64(value))
	return nil
}

func int8ToFloat64(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int8)(ptr)
	field.SetFloat64(structPtr, float64(value))
	return nil
}

func float64ToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float64)
	field.SetFloat32(structPtr, float32(value))
	return nil
}

func float32ToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(float32)
	field.SetFloat32(structPtr, value)
	return nil
}

func stringToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	value := src.(string)
	f, err := strconv.ParseFloat(value, 32)
	if err != nil {
		return err
	}
	field.SetFloat32(structPtr, float32(f))
	return nil
}

func intToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int)(ptr)
	field.SetFloat32(structPtr, float32(value))
	return nil
}

func int32ToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int32)(ptr)
	field.SetFloat32(structPtr, float32(value))
	return nil
}

func int16ToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int16)(ptr)
	field.SetFloat32(structPtr, float32(value))
	return nil
}

func int8ToFloat32(src interface{}, field *xunsafe.Field, structPtr unsafe.Pointer) error {
	ptr := xunsafe.AsPointer(src)
	value := *(*int8)(ptr)
	field.SetFloat32(structPtr, float32(value))
	return nil
}

func lookupSetter(src reflect.Type, dest reflect.Type) setter {
	switch dest.Kind() {
	case reflect.String:
		switch src.Kind() {
		case reflect.Struct:
			if dest.AssignableTo(timeType) {
				return timeToString
			}
			if dest.AssignableTo(timePtrType) {
				return timePtrToString
			}
		case reflect.String:
			return stringToString
		case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
			return intToString
		case reflect.Float64:
			return float64ToString
		case reflect.Float32:
			return float32ToString
		case reflect.Bool:
			return boolToString
		}
	case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
		switch src.Kind() {
		case reflect.String:
			return stringToInt
		case reflect.Int, reflect.Int64, reflect.Uint, reflect.Uint64:
			return intToInt
		case reflect.Int8, reflect.Uint8:
			return int8ToInt
		case reflect.Int16, reflect.Uint16:
			return int16ToInt
		case reflect.Int32, reflect.Uint32:
			return int32ToInt
		case reflect.Float64:
			return float64ToInt
		case reflect.Float32:
			return float32ToInt
		}
	case reflect.Bool:
		switch src.Kind() {
		case reflect.Bool:
			return boolToBool
		case reflect.String:
			return stringToBool
		case reflect.Int, reflect.Int64, reflect.Uint64, reflect.Uint:
			return intToBool
		}

	case reflect.Float64:
		switch src.Kind() {
		case reflect.Float64:
			return float64ToFloat64
		case reflect.Float32:
			return float32ToFloat64
		case reflect.String:
			return stringToFloat64
		case reflect.Int:
			return intToFloat64
		case reflect.Int8, reflect.Uint8:
			return int8ToFloat64
		case reflect.Int16, reflect.Uint16:
			return int16ToFloat64
		case reflect.Int32, reflect.Uint32:
			return int32ToFloat64
		}

	case reflect.Float32:
		switch src.Kind() {
		case reflect.Float64:
			return float64ToFloat32
		case reflect.Float32:
			return float32ToFloat32
		case reflect.String:
			return stringToFloat32
		case reflect.Int:
			return intToFloat32
		case reflect.Int8, reflect.Uint8:
			return int8ToFloat32
		case reflect.Int16, reflect.Uint16:
			return int16ToFloat32
		case reflect.Int32, reflect.Uint32:
			return int32ToFloat32
		}
	case reflect.Slice:
		switch src.Kind() {
		case reflect.String:
			switch dest.Elem().Kind() {
			case reflect.Int:
				return stringToInts
			case reflect.Uint64:
				return stringToUints
			case reflect.Int64:
				return stringToInt64s
			case reflect.Uint:
				return stringToUints
			case reflect.String:
				return stringToStrings
			case reflect.Float32:
				return stringToFloat32s
			case reflect.Float64:
				return stringToFloat64s
			}
		}
	}
	return anyToAny
}

func stringToInts(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n, err := r.AsInts()
	if err != nil {
		return err
	}
	field.SetValue(holder, n)
	return nil
}

func stringToInt64s(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n, err := r.AsInt64s()
	if err != nil {
		return err
	}
	field.SetValue(holder, n)
	return nil
}

func stringToUints(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n, err := r.AsUInts()
	if err != nil {
		return err
	}
	field.SetValue(holder, n)
	return nil
}

func stringToStrings(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n := []string(r)
	field.SetValue(holder, n)
	return nil
}

func stringToFloat64s(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n, err := r.AsFloats64()
	if err != nil {
		return err
	}
	field.SetValue(holder, n)
	return nil
}

func stringToFloat32s(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	value := src.(string)
	r := newRepeated(value, true)
	n, err := r.AsFloats32()
	if err != nil {
		return err
	}
	field.SetValue(holder, n)
	return nil
}

func anyToAny(src interface{}, field *xunsafe.Field, holder unsafe.Pointer) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	reflectValuePtr := reflect.New(field.Type)
	valuePtr := reflectValuePtr.Interface()
	if err = json.Unmarshal(data, valuePtr); err != nil {
		return err
	}
	value := reflectValuePtr.Elem().Interface()
	field.SetValue(holder, value)
	return nil
}

type repeated []string

func (r repeated) AsInts() ([]int, error) {
	var result = make([]int, 0, len(r))
	for _, item := range r {
		v, err := strconv.Atoi(item)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %v into %T, %w", r, result, err)
		}
		result = append(result, v)
	}
	return result, nil
}

func (r repeated) AsUInts() ([]uint, error) {
	v, err := r.AsInts()
	if err != nil {
		return nil, err
	}
	return *(*[]uint)(unsafe.Pointer(&v)), nil
}

func (r repeated) AsInt64s() ([]int64, error) {
	v, err := r.AsInts()
	if err != nil {
		return nil, err
	}
	return *(*[]int64)(unsafe.Pointer(&v)), nil
}

func (r repeated) AsUInt64s() ([]uint64, error) {
	v, err := r.AsInts()
	if err != nil {
		return nil, err
	}
	return *(*[]uint64)(unsafe.Pointer(&v)), nil
}

func (r repeated) AsFloats64() ([]float64, error) {
	var result = make([]float64, 0, len(r))
	for _, item := range r {
		v, err := strconv.ParseFloat(item, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %v into %T, %w", r, result, err)
		}
		result = append(result, v)
	}
	return result, nil
}

func (r repeated) AsFloats32() ([]float32, error) {
	var result = make([]float32, 0, len(r))
	for _, item := range r {
		v, err := strconv.ParseFloat(item, 32)
		if err != nil {
			return nil, fmt.Errorf("failed to convert %v into %T, %w", r, result, err)
		}
		result = append(result, float32(v))
	}
	return result, nil
}

func newRepeated(text string, isNumeric bool) repeated {
	if text == "" {
		return repeated{}
	}
	if text[0] == '[' && text[len(text)-1] == ']' { //remove enclosure if needed
		text = text[1 : len(text)-1]
	}
	elements := strings.Split(text, ",")
	if !isNumeric {
		return elements
	}
	var result = make(repeated, 0, len(elements))
	for _, elem := range elements {
		if isNumeric {
			if elem = strings.TrimSpace(elem); elem == "" {
				continue
			}
		}
		result = append(result, elem)
	}
	return result
}
