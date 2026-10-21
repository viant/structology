package scalar

import (
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"strconv"
	"time"
	"unsafe"
)

var timeType = reflect.TypeOf(time.Time{})
var timePtrType = reflect.TypeOf(&time.Time{})

func stringStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			strPtr := field.StringPtr(pointer)
			if strPtr == nil || (nullifyZeroValue && *strPtr == "") {
				return nullValue, false
			}
			return *strPtr, true
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		str := field.String(pointer)
		if str == "" && nullifyZeroValue {
			return nullValue, false
		}
		return str, true
	}
}

func intStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.IntPtr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatInt(int64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Int(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatInt(int64(intValue), 10), false
	}
}

func int8Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Int8Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatInt(int64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Int8(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatInt(int64(intValue), 10), false
	}
}

func int16Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Int16Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatInt(int64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Int16(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatInt(int64(intValue), 10), false
	}
}

func int32Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Int32Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatInt(int64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Int32(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatInt(int64(intValue), 10), false
	}
}

func int64Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Int64Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatInt(int64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Int64(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatInt(int64(intValue), 10), false
	}
}

func uintStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.UintPtr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatUint(uint64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Uint(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatUint(uint64(intValue), 10), false
	}
}

func uint8Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Uint8Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatUint(uint64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Uint8(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatUint(uint64(intValue), 10), false
	}
}

func uint16Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Uint16Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatUint(uint64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Uint16(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatUint(uint64(intValue), 10), false
	}
}

func uint32Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Uint32Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatUint(uint64(*intPtr), 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Uint32(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatUint(uint64(intValue), 10), false
	}
}

func uint64Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			intPtr := field.Uint64Ptr(pointer)
			if intPtr == nil || (nullifyZeroValue && *intPtr == 0) {
				return nullValue, false
			}
			return strconv.FormatUint(*intPtr, 10), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		intValue := field.Uint64(pointer)
		if intValue == 0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatUint(intValue, 10), false
	}
}

func boolStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool) FieldStringifierFn {
	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			valuePtr := field.BoolPtr(pointer)
			if valuePtr == nil || (nullifyZeroValue && *valuePtr == false) {
				return nullValue, false
			}
			return strconv.FormatBool(*valuePtr), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		value := field.Bool(pointer)
		if !value && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatBool(value), false
	}
}

func getStringifierConfig(options []interface{}) *StringifierConfig {
	if len(options) == 0 {
		return nil
	}

	for _, candidate := range options {
		switch candidate.(type) {
		case *StringifierConfig:
			return candidate.(*StringifierConfig)
		}
	}

	return nil
}

func float64Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool, options ...interface{}) FieldStringifierFn {
	prec := -1
	sConfig := getStringifierConfig(options)

	if sConfig != nil {
		aPrec, err := strconv.Atoi(sConfig.StringifierFloat64Config.Precision)
		if err == nil {
			prec = aPrec
		}
	}

	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			valuePtr := field.Float64Ptr(pointer)
			if valuePtr == nil || (nullifyZeroValue && *valuePtr == 0) {
				return nullValue, false
			}
			return strconv.FormatFloat(*valuePtr, 'f', prec, 64), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		value := field.Float64(pointer)
		if value == 0.0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatFloat(value, 'f', prec, 64), false
	}
}

func float32Stringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool, options ...interface{}) FieldStringifierFn {
	prec := -1
	sConfig := getStringifierConfig(options)

	if sConfig != nil {
		aPrec, err := strconv.Atoi(sConfig.StringifierFloat32Config.Precision)
		if err == nil {
			prec = aPrec
		}
	}

	if wasPointer {
		return func(pointer unsafe.Pointer) (string, bool) {
			valuePtr := field.Float32Ptr(pointer)
			if valuePtr == nil || (nullifyZeroValue && *valuePtr == 0) {
				return nullValue, false
			}
			return strconv.FormatFloat(float64(*valuePtr), 'f', prec, 64), false
		}
	}

	return func(pointer unsafe.Pointer) (string, bool) {
		value := field.Float32(pointer)
		if value == 0.0 && nullifyZeroValue {
			return nullValue, false
		}
		return strconv.FormatFloat(float64(value), 'f', prec, 64), false
	}
}

// TODO: Time format
func defaultStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string) FieldStringifierFn {

	timeLayout := field.Tag.Get("timeLayout")
	if timeLayout == "" {
		timeLayout = time.RFC3339
	}

	if field.Type == timeType {
		return func(pointer unsafe.Pointer) (string, bool) {
			value := field.Time(pointer)
			if value.IsZero() && nullifyZeroValue {
				return nullValue, false
			}
			return value.Format(timeLayout), true
		}
	}
	if field.Type == timePtrType {
		return func(pointer unsafe.Pointer) (string, bool) {
			ptr := field.TimePtr(pointer)
			if ptr == nil || (nullifyZeroValue && *ptr == time.Time{}) {
				return nullValue, false
			}
			return ptr.Format(timeLayout), true
		}
	}

	return func(pointer unsafe.Pointer) (value string, wasString bool) {
		i := field.Value(pointer)
		return fmt.Sprintf("%v", i), false
	}
}

func interfaceStringifier(field *xunsafe.Field, nullifyZeroValue bool, nullValue string, wasPointer bool, options ...interface{}) FieldStringifierFn {
	return func(pointer unsafe.Pointer) (string, bool) {

		var iface interface{}

		switch actual := field.Interface(pointer).(type) {
		case *interface{}:
			iface = *actual
		default:
			iface = actual
		}

		if iface == nil { // interface with no type, no value
			return nullifiedInterface(nullifyZeroValue, nullValue)
		}

		valUPtr := xunsafe.AsPointer(iface)

		if valUPtr == nil { // interface with type but no value
			return nullifiedInterface(nullifyZeroValue, nullValue)
		}

		rType := reflect.TypeOf(iface)
		if rType.Kind() == reflect.Ptr {
			rType = rType.Elem()
		}

		// important to set offset to 0 and pointer (struct upointer) to interface value upointer
		// because struct addr can be greater than value addr and offset can't be negative
		fakeField := reflect.StructField{
			Name:      field.Name,
			PkgPath:   field.PkgPath(),
			Type:      rType,
			Tag:       field.Tag,
			Offset:    0,
			Index:     []int{int(field.Index)},
			Anonymous: field.Anonymous,
		}

		fakeXField := xunsafe.NewField(fakeField)

		var internalFn FieldStringifierFn
		internalFn = Stringifier(fakeXField, nullifyZeroValue, nullValue, options...)

		return internalFn(valUPtr)
	}
}

func nullifiedInterface(_ bool, nullValue string) (string, bool) { return nullValue, false }
