package unmarshal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/viant/x/shape"
	"reflect"
)

// decodeRawCollection shares lexical collection traversal between the standard
// and native typed decoders; element assignment stays with the selected owner.
func decodeRawCollection(raw []byte, value reflect.Value, metadata *shape.Representation, rejectDuplicates bool, decode func([]byte, reflect.Value, *shape.Representation) error) error {
	var child *shape.Representation
	if metadata != nil {
		child = metadata.Elem()
	}
	switch value.Kind() {
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
			if err := decode(member, value.Index(i), child); err != nil {
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
		seen := map[string]bool{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			if key := token.(string); rejectDuplicates && seen[key] {
				return fmt.Errorf("duplicate field %s", key)
			} else {
				seen[key] = true
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
			err = decode(member, element, child)
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
		return fmt.Errorf("expected collection, got %s", value.Type())
	}
}
