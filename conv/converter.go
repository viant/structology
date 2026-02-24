package conv

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// DefaultDateLayout is the default layout used for time parsing when no layout is specified
const DefaultDateLayout = "2006-01-02 15:04:05.000"

// Options contains configuration for the converter
type Options struct {
	// DateLayout specifies the layout for time parsing
	DateLayout string
	// TagName is the struct tag name to look for mapping information
	TagName string
	// CaseSensitive controls whether field/key matching is case sensitive
	CaseSensitive bool
	// IgnoreUnmapped controls whether to ignore unmapped fields
	IgnoreUnmapped bool
	// ClonePointerData if true, creates a clone of data pointed by pointers
	ClonePointerData bool
	// AccessUnexported if true, allows accessing unexported fields
	AccessUnexported bool
}

// DefaultOptions returns default conversion options
func DefaultOptions() Options {
	return Options{
		DateLayout:    DefaultDateLayout,
		TagName:       "json", // Changed: Set "json" as default tag name
		CaseSensitive: false,
	}
}

// Converter provides type conversion functionality
type Converter struct {
	options       Options
	structCache   sync.Map // map[reflect.Type]*structInfo
	customConvMap sync.Map // map[typeKey]ConversionFunc
	structTypeMap sync.Map // map[string]bool
}

// ConversionFunc defines a custom conversion function
type ConversionFunc func(src interface{}, dest interface{}, opts Options) error

type typeKey struct {
	srcType  reflect.Type
	destType reflect.Type
}

// NewConverter creates a new type converter with the provided options
func NewConverter(options Options) *Converter {
	return &Converter{
		options: options,
	}
}

// RegisterConversion registers a custom conversion function between source and destination types
func (c *Converter) RegisterConversion(srcType, destType reflect.Type, fn ConversionFunc) {
	c.customConvMap.Store(typeKey{srcType, destType}, fn)
}

// Convert converts the source value to the destination value
// Changed signature to take destination as second parameter
func (c *Converter) Convert(src interface{}, dest interface{}) error {
	if dest == nil {
		return errors.New("destination cannot be nil")
	}

	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return errors.New("destination must be a pointer")
	}
	if destValue.Elem().Kind() == reflect.Ptr {
		destValue = destValue.Elem()
	}

	if destValue.IsNil() {
		return errors.New("destination pointer cannot be nil")
	}

	if src == nil {
		return nil // Nothing to convert
	}

	srcValue := reflect.ValueOf(src)
	srcType := srcValue.Type()
	destElemType := destValue.Elem().Type()

	// Try custom conversion first
	if v, ok := c.customConvMap.Load(typeKey{srcType, destElemType}); ok {
		return v.(ConversionFunc)(src, dest, c.options)
	}

	// Handle direct assignability
	if srcType.AssignableTo(destElemType) {
		if c.options.ClonePointerData && srcType.Kind() == reflect.Ptr {
			return c.clonePointerValue(destValue, srcValue)
		}
		destValue.Elem().Set(srcValue)
		return nil
	}

	// Check for primitive type conversions first, before using general ConvertibleTo
	destKind := destElemType.Kind()

	// Handle primitive conversions first
	switch destKind {
	case reflect.String:
		return c.convertToString(destValue, srcValue)
	case reflect.Bool:
		return c.convertToBool(destValue, srcValue)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return c.convertToInt(destValue, srcValue)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return c.convertToUint(destValue, srcValue)
	case reflect.Float32, reflect.Float64:
		return c.convertToFloat(destValue, srcValue)
	}

	// Then handle direct convertibility for non-primitive types
	if srcType.ConvertibleTo(destElemType) {
		destValue.Elem().Set(srcValue.Convert(destElemType))
		return nil
	}

	return c.convertComplex(destValue, srcValue)
}

func (c *Converter) clonePointerValue(destValue, srcValue reflect.Value) error {
	srcElem := srcValue.Elem()
	destElem := destValue.Elem()

	// If pointer to struct, we want to deep copy it
	if srcElem.Kind() == reflect.Struct {
		newValue := reflect.New(srcElem.Type())
		for i := 0; i < srcElem.NumField(); i++ {
			field := srcElem.Field(i)
			newField := newValue.Elem().Field(i)
			if field.CanInterface() {
				newField.Set(field)
			} else if c.options.AccessUnexported {
				unsafeField := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
				unsafeNewField := reflect.NewAt(newField.Type(), unsafe.Pointer(newField.UnsafeAddr())).Elem()

				unsafeNewField.Set(unsafeField)
			}
		}
		destElem.Set(newValue)
	} else {
		// For other pointer types, copy the value
		newValue := reflect.New(srcElem.Type())
		newValue.Elem().Set(srcElem)
		destElem.Set(newValue)
	}
	return nil
}

// generateStructSignature generates a signature for a struct type for quick comparison
func (c *Converter) generateStructSignature(rType reflect.Type) string {
	if rType.Kind() != reflect.Struct {
		return ""
	}

	var sb strings.Builder
	sb.WriteString(rType.PkgPath())
	sb.WriteRune(':')

	for i := 0; i < rType.NumField(); i++ {
		field := rType.Field(i)
		sb.WriteString(field.Name)
		sb.WriteRune(':')
		sb.WriteString(field.Type.String())
		sb.WriteRune(';')
	}

	return sb.String()
}

// areStructTypesCompatible checks if two struct types have compatible memory layouts
func (c *Converter) areStructTypesCompatible(srcType, destType reflect.Type) bool {
	if srcType.Kind() != reflect.Struct || destType.Kind() != reflect.Struct {
		return false
	}

	if srcType.NumField() != destType.NumField() {
		return false
	}

	// Use xreflect to check if types are the same
	srcSig := c.generateStructSignature(srcType)
	destSig := c.generateStructSignature(destType)

	// If we've already determined these are compatible, return quickly
	key := srcSig + "->" + destSig
	if v, ok := c.structTypeMap.Load(key); ok {
		return v.(bool)
	}

	// First time checking these types
	// We consider them compatible if fields match and types are the same or convertible
	compatible := true

	for i := 0; i < srcType.NumField(); i++ {
		srcField := srcType.Field(i)
		destField := destType.Field(i)

		if srcField.Name != destField.Name {
			compatible = false
			break
		}

		if srcField.Type != destField.Type && !srcField.Type.ConvertibleTo(destField.Type) {
			// If these are structs themselves, recursively check
			if srcField.Type.Kind() == reflect.Struct && destField.Type.Kind() == reflect.Struct {
				if !c.areStructTypesCompatible(srcField.Type, destField.Type) {
					compatible = false
					break
				}
			} else {
				compatible = false
				break
			}
		}
	}

	// Cache the result
	c.structTypeMap.Store(key, compatible)
	return compatible
}

func (c *Converter) convertComplex(destValue, srcValue reflect.Value) error {
	// Ensure we're working with the right value
	srcValue = indirect(srcValue)
	if !srcValue.IsValid() {
		return nil // Source is nil, nothing to do
	}

	destType := destValue.Type().Elem()
	destKind := destType.Kind()
	srcKind := srcValue.Kind()

	// Special optimization for struct to struct conversion when types have the same signature
	if srcKind == reflect.Struct && destKind == reflect.Struct {
		srcType := srcValue.Type()
		if c.areStructTypesCompatible(srcType, destType) {
			// Direct memory copy for same layout structs
			destValue.Elem().Set(srcValue.Convert(destType))
			return nil
		}
	}

	// Handle complex conversions
	switch destKind {
	case reflect.Slice:
		return c.convertToSlice(destValue, srcValue)
	case reflect.Map:
		return c.convertToMap(destValue, srcValue)
	case reflect.Struct:
		if destType == reflect.TypeOf(time.Time{}) {
			return c.convertToTime(destValue, srcValue)
		}
		return c.convertToStruct(destValue, srcValue)
	}

	return fmt.Errorf("unsupported conversion: %v to %v", srcValue.Type(), destType)
}

func (c *Converter) convertToString(destValue, srcValue reflect.Value) error {
	var result string

	switch srcValue.Kind() {
	case reflect.String:
		result = srcValue.String()
	case reflect.Bool:
		result = strconv.FormatBool(srcValue.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = strconv.FormatInt(srcValue.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result = strconv.FormatUint(srcValue.Uint(), 10)
	case reflect.Float32:
		result = strconv.FormatFloat(srcValue.Float(), 'f', -1, 32)
	case reflect.Float64:
		result = strconv.FormatFloat(srcValue.Float(), 'f', -1, 64)
	case reflect.Slice:
		if srcValue.Type().Elem().Kind() == reflect.Uint8 { // []byte
			result = string(srcValue.Bytes())
		} else {
			return fmt.Errorf("cannot convert %v to string", srcValue.Type())
		}
	default:
		return fmt.Errorf("cannot convert %v to string", srcValue.Type())
	}

	destValue.Elem().SetString(result)
	return nil
}

func (c *Converter) convertToBool(destValue, srcValue reflect.Value) error {
	var result bool

	switch srcValue.Kind() {
	case reflect.Bool:
		result = srcValue.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = srcValue.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result = srcValue.Uint() != 0
	case reflect.Float32, reflect.Float64:
		result = srcValue.Float() != 0
	case reflect.String:
		var err error
		result, err = strconv.ParseBool(srcValue.String())
		if err != nil {
			// Try numeric conversion if boolean parsing fails
			if f, err := strconv.ParseFloat(srcValue.String(), 64); err == nil {
				result = f != 0
				break
			}
			return err
		}
	default:
		return fmt.Errorf("cannot convert %v to bool", srcValue.Type())
	}

	destValue.Elem().SetBool(result)
	return nil
}

func (c *Converter) convertToInt(destValue, srcValue reflect.Value) error {
	var result int64

	switch srcValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = srcValue.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v := srcValue.Uint()
		result = int64(v)
	case reflect.Float32, reflect.Float64:
		result = int64(srcValue.Float())
	case reflect.Bool:
		if srcValue.Bool() {
			result = 1
		}
	case reflect.String:
		var err error
		if strings.Contains(srcValue.String(), ".") {
			var f float64
			f, err = strconv.ParseFloat(srcValue.String(), 64)
			result = int64(f)
		} else {
			result, err = strconv.ParseInt(srcValue.String(), 0, 64)
		}
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot convert %v to int", srcValue.Type())
	}

	destValue.Elem().SetInt(result)
	return nil
}

func (c *Converter) convertToUint(destValue, srcValue reflect.Value) error {
	var result uint64

	switch srcValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v := srcValue.Int()
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %d to unsigned int", v)
		}
		result = uint64(v)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result = srcValue.Uint()
	case reflect.Float32, reflect.Float64:
		v := srcValue.Float()
		if v < 0 {
			return fmt.Errorf("cannot convert negative value %f to unsigned int", v)
		}
		result = uint64(v)
	case reflect.Bool:
		if srcValue.Bool() {
			result = 1
		}
	case reflect.String:
		var err error
		if strings.Contains(srcValue.String(), ".") {
			var f float64
			f, err = strconv.ParseFloat(srcValue.String(), 64)
			if f < 0 {
				return fmt.Errorf("cannot convert negative value %f to unsigned int", f)
			}
			result = uint64(f)
		} else {
			result, err = strconv.ParseUint(srcValue.String(), 0, 64)
		}
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot convert %v to uint", srcValue.Type())
	}

	destValue.Elem().SetUint(result)
	return nil
}

func (c *Converter) convertToFloat(destValue, srcValue reflect.Value) error {
	var result float64

	switch srcValue.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		result = float64(srcValue.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		result = float64(srcValue.Uint())
	case reflect.Float32, reflect.Float64:
		result = srcValue.Float()
	case reflect.Bool:
		if srcValue.Bool() {
			result = 1
		}
	case reflect.String:
		var err error
		result, err = strconv.ParseFloat(srcValue.String(), 64)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("cannot convert %v to float", srcValue.Type())
	}

	destValue.Elem().SetFloat(result)
	return nil
}

func (c *Converter) convertToTime(destValue, srcValue reflect.Value) error {
	var t time.Time
	var err error

	switch srcValue.Kind() {
	case reflect.String:
		layout := c.options.DateLayout
		if layout == "" {
			layout = DefaultDateLayout
		}

		// Try to parse with the specified layout
		t, err = time.Parse(layout, srcValue.String())
		if err != nil {
			// Try RFC3339 as a fallback
			t, err = time.Parse(time.RFC3339, srcValue.String())
			if err != nil {
				// Try other common formats
				formats := []string{
					time.RFC3339Nano,
					time.RFC3339,
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02",
				}

				for _, format := range formats {
					t, err = time.Parse(format, srcValue.String())
					if err == nil {
						break
					}
				}

				if err != nil {
					return fmt.Errorf("cannot parse time string '%s': %w", srcValue.String(), err)
				}
			}
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		unixTime := srcValue.Int()
		if unixTime > 1e10 { // Assuming nanoseconds if value is very large
			t = time.Unix(0, unixTime)
		} else {
			t = time.Unix(unixTime, 0)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		unixTime := int64(srcValue.Uint())
		if unixTime > 1e10 { // Assuming nanoseconds if value is very large
			t = time.Unix(0, unixTime)
		} else {
			t = time.Unix(unixTime, 0)
		}
	case reflect.Float32, reflect.Float64:
		unixTime := int64(srcValue.Float())
		fractional := srcValue.Float() - float64(unixTime)
		nanos := int64(fractional * 1e9)
		t = time.Unix(unixTime, nanos)
	case reflect.Struct:
		if srcValue.Type() == reflect.TypeOf(time.Time{}) {
			t = srcValue.Interface().(time.Time)
		} else {
			return fmt.Errorf("cannot convert struct %v to time.Time", srcValue.Type())
		}
	default:
		return fmt.Errorf("cannot convert %v to time.Time", srcValue.Type())
	}

	destValue.Elem().Set(reflect.ValueOf(t))
	return nil
}

func (c *Converter) convertToSlice(destValue, srcValue reflect.Value) error {
	destType := destValue.Type().Elem()
	destElemType := destType.Elem()

	// Special case: []byte to string conversion
	if destElemType.Kind() == reflect.Uint8 && srcValue.Kind() == reflect.String {
		destValue.Elem().SetBytes([]byte(srcValue.String()))
		return nil
	}

	// Handle special case: []interface{} containing maps to []struct
	if srcValue.Kind() == reflect.Slice &&
		srcValue.Type().Elem().Kind() == reflect.Interface &&
		destElemType.Kind() == reflect.Struct {
		length := srcValue.Len()
		sliceValue := reflect.MakeSlice(destType, length, length)

		for i := 0; i < length; i++ {
			elemValue := srcValue.Index(i)
			elemPtr := reflect.New(destElemType)

			// Convert map[string]interface{} to struct
			if err := c.Convert(elemValue.Interface(), elemPtr.Interface()); err != nil {
				return fmt.Errorf("error converting slice element %d: %w", i, err)
			}

			sliceValue.Index(i).Set(elemPtr.Elem())
		}

		destValue.Elem().Set(sliceValue)
		return nil
	}

	// Special case handling for []interface{} to []string conversion
	if srcValue.Kind() == reflect.Slice &&
		srcValue.Type().Elem().Kind() == reflect.Interface &&
		destType.Elem().Kind() == reflect.String {
		length := srcValue.Len()
		sliceValue := reflect.MakeSlice(destType, length, length)

		for i := 0; i < length; i++ {
			elemValue := srcValue.Index(i)
			elemPtr := reflect.New(destElemType)

			if err := c.Convert(elemValue.Interface(), elemPtr.Interface()); err != nil {
				return fmt.Errorf("error converting slice element %d: %w", i, err)
			}

			sliceValue.Index(i).Set(elemPtr.Elem())
		}

		destValue.Elem().Set(sliceValue)
		return nil
	}

	if srcValue.Kind() != reflect.Slice && srcValue.Kind() != reflect.Array {
		// Convert single value to slice with one element
		sliceValue := reflect.MakeSlice(destType, 1, 1)
		elemPtr := reflect.New(destElemType)

		if err := c.Convert(srcValue.Interface(), elemPtr.Interface()); err != nil {
			return err
		}

		sliceValue.Index(0).Set(elemPtr.Elem())
		destValue.Elem().Set(sliceValue)
		return nil
	}

	length := srcValue.Len()
	sliceValue := reflect.MakeSlice(destType, length, length)

	for i := 0; i < length; i++ {
		// Handle the case when destElemType is a pointer type (like *Basic)
		if destElemType.Kind() == reflect.Ptr {
			// Create a new instance of the pointed-to type
			elemValue := reflect.New(destElemType.Elem())
			// Convert source value to the pointed-to type
			if err := c.Convert(srcValue.Index(i).Interface(), elemValue.Interface()); err != nil {
				return fmt.Errorf("error converting slice element %d: %w", i, err)
			}
			// Set the pointer value in the slice
			sliceValue.Index(i).Set(elemValue)
		} else {
			// Original code for non-pointer element types
			elemPtr := reflect.New(destElemType)
			if err := c.Convert(srcValue.Index(i).Interface(), elemPtr.Interface()); err != nil {
				return fmt.Errorf("error converting slice element %d: %w", i, err)
			}
			sliceValue.Index(i).Set(elemPtr.Elem())
		}
	}

	destValue.Elem().Set(sliceValue)
	return nil
}

func (c *Converter) convertToMap(destValue, srcValue reflect.Value) error {
	destType := destValue.Type().Elem()
	destKeyType := destType.Key()
	destValType := destType.Elem()

	mapValue := reflect.MakeMap(destType)

	// Handle struct to map conversion
	if srcValue.Kind() == reflect.Struct {
		// Get all exported fields from the struct
		for i := 0; i < srcValue.NumField(); i++ {
			field := srcValue.Type().Field(i)
			fieldValue := srcValue.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Skip fields with json tag "-"
			if tag := field.Tag.Get(c.options.TagName); tag == "-" {
				continue
			}

			// Get the key name (field name or from tag)
			keyName := field.Name

			// Convert the field value to map value type
			valPtr := reflect.New(destValType)
			if err := c.Convert(fieldValue.Interface(), valPtr.Interface()); err != nil {
				return fmt.Errorf("error converting field %s: %w", field.Name, err)
			}

			// Convert field name to map key type
			keyPtr := reflect.New(destKeyType)
			if err := c.Convert(keyName, keyPtr.Interface()); err != nil {
				return fmt.Errorf("error converting field name %s to key type: %w", field.Name, err)
			}

			mapValue.SetMapIndex(keyPtr.Elem(), valPtr.Elem())
		}
	} else if srcValue.Kind() == reflect.Map {
		iter := srcValue.MapRange()
		for iter.Next() {
			keyPtr := reflect.New(destKeyType)
			if err := c.Convert(iter.Key().Interface(), keyPtr.Interface()); err != nil {
				return fmt.Errorf("error converting map key: %w", err)
			}

			valPtr := reflect.New(destValType)
			if err := c.Convert(iter.Value().Interface(), valPtr.Interface()); err != nil {
				return fmt.Errorf("error converting map value: %w", err)
			}

			mapValue.SetMapIndex(keyPtr.Elem(), valPtr.Elem())
		}
	} else {
		return fmt.Errorf("cannot convert %v to map", srcValue.Type())
	}

	destValue.Elem().Set(mapValue)
	return nil
}

func (c *Converter) convertToStruct(destValue, srcValue reflect.Value) error {
	destType := destValue.Type().Elem()
	destInfo := c.getStructInfo(destType)

	var srcMap map[string]interface{}

	switch srcValue.Kind() {
	case reflect.Map:
		srcMap = make(map[string]interface{})
		iter := srcValue.MapRange()
		for iter.Next() {
			key := fmt.Sprintf("%v", iter.Key().Interface())
			srcMap[key] = iter.Value().Interface()
		}
	case reflect.Struct:
		srcInfo := c.getStructInfo(srcValue.Type())
		srcMap = make(map[string]interface{})

		for _, field := range srcInfo.fields {
			fieldValue := srcValue.FieldByIndex(field.index)

			var fieldInterface interface{}
			if fieldValue.CanInterface() {
				fieldInterface = fieldValue.Interface()
			} else if c.options.AccessUnexported {
				// Use unsafe to access unexported fields
				fieldPtr := reflect.NewAt(fieldValue.Type(), unsafe.Pointer(fieldValue.UnsafeAddr()))
				fieldInterface = fieldPtr.Elem().Interface()
			} else {
				continue
			}

			srcMap[field.name] = fieldInterface
		}
	default:
		return fmt.Errorf("cannot convert %v to struct", srcValue.Type())
	}

	for fieldName, fieldInfo := range destInfo.fieldsByName {
		var srcFieldName string

		// Try by tag name first
		if fieldInfo.tagName != "" && fieldInfo.tagName != "-" {
			srcFieldName = fieldInfo.tagName
			value, exists := srcMap[srcFieldName]
			if exists {
				fieldValue := destValue.Elem().FieldByIndex(fieldInfo.index)
				if setStructField(c, fieldValue, value, fieldInfo.name, fieldName, c.options) {
					continue
				}
			}
		}

		// Then try by field name if case sensitive
		if c.options.CaseSensitive {
			srcFieldName = fieldName
			value, exists := srcMap[srcFieldName]
			if exists {
				fieldValue := destValue.Elem().FieldByIndex(fieldInfo.index)
				if setStructField(c, fieldValue, value, fieldInfo.name, fieldName, c.options) {
					continue
				}
			}
		} else {
			// Or try with case-insensitive field name
			lowerFieldName := strings.ToLower(fieldName)
			for key, value := range srcMap {
				if strings.ToLower(key) == lowerFieldName {
					fieldValue := destValue.Elem().FieldByIndex(fieldInfo.index)
					if setStructField(c, fieldValue, value, fieldInfo.name, fieldName, c.options) {
						break
					}
				}
			}
		}
	}

	return nil
}

func setStructField(c *Converter, fieldValue reflect.Value, value interface{}, fieldName string, structFieldName string, opts Options) bool {
	if !fieldValue.CanSet() {
		if opts.AccessUnexported {
			// For unexported fields using unsafe pointer
			unsafeFieldPtr := reflect.NewAt(fieldValue.Type(), unsafe.Pointer(fieldValue.UnsafeAddr())).Elem()
			tempValue := reflect.New(fieldValue.Type())

			if err := c.Convert(value, tempValue.Interface()); err == nil {
				unsafeFieldPtr.Set(tempValue.Elem())
				return true
			}
		}
		return false
	}

	// Handle nil pointer case for struct pointers
	if value == nil && fieldValue.Kind() == reflect.Ptr {
		// If the field is a pointer and the value is nil, set it to nil
		fieldValue.Set(reflect.Zero(fieldValue.Type()))
		return true
	}

	// Special handling for nested structs when field is a pointer to struct
	if fieldValue.Kind() == reflect.Ptr && fieldValue.Type().Elem().Kind() == reflect.Struct {
		// Check if value is a map that can be converted to the struct type
		if valueMap, ok := value.(map[string]interface{}); ok {
			// Create a new instance of the struct
			newStructPtr := reflect.New(fieldValue.Type().Elem())

			// Convert the map to the struct
			if err := c.Convert(valueMap, newStructPtr.Interface()); err == nil {
				// Set the pointer to the new struct
				fieldValue.Set(newStructPtr)
				return true
			}
		}
	}

	fieldPtr := reflect.New(fieldValue.Type())
	if err := c.Convert(value, fieldPtr.Interface()); err != nil {
		return false
	}

	fieldValue.Set(fieldPtr.Elem())
	return true
}

// struct reflection caching

type structField struct {
	name    string
	tagName string
	index   []int
}

type structInfo struct {
	fields       []structField
	fieldsByName map[string]structField
}

func (c *Converter) getStructInfo(t reflect.Type) *structInfo {
	if v, ok := c.structCache.Load(t); ok {
		return v.(*structInfo)
	}

	info := &structInfo{
		fields:       make([]structField, 0),
		fieldsByName: make(map[string]structField),
	}

	c.buildStructInfo(t, info, nil)

	c.structCache.Store(t, info)
	return info
}

func (c *Converter) buildStructInfo(t reflect.Type, info *structInfo, index []int) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		fieldIndex := make([]int, len(index)+1)
		copy(fieldIndex, index)
		fieldIndex[len(index)] = i

		if field.Anonymous {
			// Handle embedded fields
			ft := field.Type
			if ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				c.buildStructInfo(ft, info, fieldIndex)
				continue
			}
		}

		fieldName := field.Name
		tagName := ""

		// Check for json tag by default
		if tag := field.Tag.Get(c.options.TagName); tag != "" {
			parts := strings.Split(tag, ",")
			if parts[0] == "-" {
				tagName = "-" // Mark fields to be skipped
				// continue - Don't continue here, we need to record this field with tagName "-"
			} else if parts[0] != "" {
				tagName = parts[0]
			}
		}

		sf := structField{
			name:    fieldName,
			tagName: tagName,
			index:   fieldIndex,
		}

		info.fields = append(info.fields, sf)

		name := fieldName
		if !c.options.CaseSensitive {
			name = strings.ToLower(name)
		}
		info.fieldsByName[name] = sf

		if tagName != "" && tagName != "-" {
			if !c.options.CaseSensitive {
				tagName = strings.ToLower(tagName)
			}
			info.fieldsByName[tagName] = sf
		}
	}
}

// helper functions

func indirect(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}
	return v
}
