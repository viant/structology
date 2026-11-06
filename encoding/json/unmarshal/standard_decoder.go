package unmarshal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/viant/x/shape"
)

// StandardDecoder applies optional metadata to storage selection, delegating
// ordinary leaves and unchanged subtrees to encoding/json. It does not call the
// native formatted decoder or reinterpret the original receiver's method set.
type StandardDecoder struct{ representation *shape.Representation }

func NewStandardDecoder(metadata *shape.Representation) *StandardDecoder {
	return &StandardDecoder{representation: metadata}
}
func (d *StandardDecoder) Unmarshal(data []byte, destination any) error {
	if d.representation == nil || !d.representation.Changed() {
		return json.Unmarshal(data, destination)
	}
	value := reflect.ValueOf(destination)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(destination)}
	}
	value = value.Elem()
	if value.Type() != d.representation.Original() {
		return fmt.Errorf("representation expects destination *%s, got %T", d.representation.Original(), destination)
	}
	// Match encoding/json's syntax-before-mutation behavior, including discarded fields.
	var valid json.RawMessage
	if err := json.Unmarshal(data, &valid); err != nil {
		return err
	}
	return decodeStandardRepresentation(data, value, d.representation)
}
func decodeStandardRepresentation(data []byte, value reflect.Value, metadata *shape.Representation) error {
	if metadata == nil || !metadata.Changed() {
		return json.Unmarshal(data, value.Addr().Interface())
	}
	raw := bytes.TrimSpace(data)
	if bytes.Equal(raw, []byte("null")) {
		return json.Unmarshal(raw, value.Addr().Interface())
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return decodeStandardRepresentation(raw, value.Elem(), metadata.Elem())
	}
	switch value.Kind() {
	case reflect.Struct:
		if len(raw) == 0 || raw[0] != '{' {
			return json.Unmarshal(raw, value.Addr().Interface())
		}
		fields, err := metadata.JSONFields()
		if err != nil {
			return err
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		if _, err = decoder.Token(); err != nil {
			return err
		}
		var first error
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			key := token.(string)
			var member json.RawMessage
			if err = decoder.Decode(&member); err != nil {
				return err
			}
			index := -1
			for i := range fields {
				if fields[i].Name == key {
					index = i
					break
				}
			}
			if index < 0 {
				for i := range fields {
					if strings.EqualFold(fields[i].Name, key) {
						index = i
						break
					}
				}
			}
			if index < 0 {
				continue
			}
			field := fields[index]
			target, err := standardFieldValue(value, field.Field.Index)
			if err != nil {
				return err
			}
			if field.Quoted {
				err = decodeStandardQuoted(member, target)
			} else {
				err = decodeStandardRepresentation(member, target, metadata.FieldShapeByIndex(field.Field.Index))
			}
			if err != nil {
				if hasCustomUnmarshalType(target.Type()) {
					return err
				}
				if typeErr, ok := err.(*json.UnmarshalTypeError); ok {
					copy := *typeErr
					copy.Struct = value.Type().Name()
					if copy.Field == "" {
						copy.Field = field.Name
					} else {
						copy.Field = field.Name + "." + copy.Field
					}
					if first == nil {
						first = &copy
					}
					continue
				}
				return err
			}
		}
		return first
	case reflect.Slice, reflect.Array:
		if len(raw) == 0 || raw[0] != '[' {
			return json.Unmarshal(raw, value.Addr().Interface())
		}
		var members []json.RawMessage
		if err := json.Unmarshal(raw, &members); err != nil {
			return err
		}
		var first error
		for i, member := range members {
			if value.Kind() == reflect.Array && i >= value.Len() {
				break
			}
			if value.Kind() == reflect.Slice && i >= value.Len() {
				value.Grow(i + 1 - value.Len())
				value.SetLen(i + 1)
			}
			if err := decodeStandardRepresentation(member, value.Index(i), metadata.Elem()); err != nil {
				if _, ok := err.(*json.UnmarshalTypeError); !ok {
					return err
				}
				if first == nil {
					first = err
				}
			}
		}
		if value.Kind() == reflect.Slice {
			if len(members) == 0 {
				value.Set(reflect.MakeSlice(value.Type(), 0, 0))
			} else {
				value.SetLen(len(members))
			}
		} else {
			for i := len(members); i < value.Len(); i++ {
				value.Index(i).SetZero()
			}
		}
		return first
	case reflect.Map:
		if len(raw) == 0 || raw[0] != '{' {
			return json.Unmarshal(raw, value.Addr().Interface())
		}
		keyMapType := reflect.MapOf(value.Type().Key(), reflect.TypeFor[json.RawMessage]())
		if err := json.Unmarshal([]byte("{}"), reflect.New(keyMapType).Interface()); err != nil {
			return err
		}
		if value.IsNil() {
			value.Set(reflect.MakeMap(value.Type()))
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		if _, err := decoder.Token(); err != nil {
			return err
		}
		var first error
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			var member json.RawMessage
			if err = decoder.Decode(&member); err != nil {
				return err
			}
			// Delegate key decoding, including TextUnmarshaler and integer boundaries,
			// to the selected standard decoder rather than inventing another key parser.
			encodedKey, _ := json.Marshal(token.(string))
			keyObject := append([]byte{'{'}, encodedKey...)
			keyObject = append(keyObject, []byte(":null}")...)
			keyMap := reflect.New(keyMapType)
			if err = json.Unmarshal(keyObject, keyMap.Interface()); err != nil {
				return err
			}
			key := keyMap.Elem().MapKeys()[0]
			element := reflect.New(value.Type().Elem()).Elem()
			err = decodeStandardRepresentation(member, element, metadata.Elem())
			if err != nil {
				if _, ok := err.(*json.UnmarshalTypeError); !ok {
					return err
				}
				if first == nil {
					first = err
				}
			}
			value.SetMapIndex(key, element)
		}
		return first
	default:
		return json.Unmarshal(raw, value.Addr().Interface())
	}
}
func standardFieldValue(value reflect.Value, index []int) (reflect.Value, error) {
	for _, i := range index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				if !value.CanSet() {
					return reflect.Value{}, fmt.Errorf("json: cannot set embedded pointer to unexported struct: %s", value.Type().Elem())
				}
				value.Set(reflect.New(value.Type().Elem()))
			}
			value = value.Elem()
		}
		value = value.Field(i)
	}
	if !value.CanAddr() || !value.CanInterface() {
		return reflect.Value{}, fmt.Errorf("JSON destination field is inaccessible")
	}
	return value, nil
}
func decodeStandardQuoted(raw []byte, value reflect.Value) error {
	// A one-field standard wrapper retains encoding/json's exact ,string rules
	// and custom-method precedence without rebuilding any embedded storage type.
	wrapper := reflect.StructOf([]reflect.StructField{{Name: "Value", Type: value.Type(), Tag: `json:"value,string"`}})
	target := reflect.New(wrapper)
	target.Elem().Field(0).Set(value)
	object := append([]byte(`{"value":`), raw...)
	object = append(object, '}')
	err := json.Unmarshal(object, target.Interface())
	value.Set(target.Elem().Field(0))
	return err
}
