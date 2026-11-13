package unmarshal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/viant/structology/encoding/json/internal/lru"
	"github.com/viant/tagly/choice"
	"github.com/viant/tagly/format"
	"github.com/viant/x/shape"
)

// StandardDecoder applies optional metadata to storage selection, delegating
// ordinary leaves and unchanged subtrees to encoding/json. It does not call the
// native formatted decoder or reinterpret the original receiver's method set.
type StandardDecoder struct {
	representation *shape.Representation
	presence       bool
	strictUnknown  bool
}

func (d *StandardDecoder) WithUnknownFieldPolicy(policy UnknownFieldPolicy) *StandardDecoder {
	copy := *d
	copy.strictUnknown = policy == ErrorOnUnknown
	return &copy
}

func (d *StandardDecoder) WithPresenceTracking(enabled bool) *StandardDecoder {
	copy := *d
	copy.presence = enabled
	return &copy
}

func NewStandardDecoder(metadata *shape.Representation) *StandardDecoder {
	return &StandardDecoder{representation: metadata}
}
func (d *StandardDecoder) Unmarshal(data []byte, destination any) error {
	typ := reflect.TypeOf(destination)
	if typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	guards, guardErr := ValidateInputGuards(typ, d.representation)
	if guardErr != nil {
		return guardErr
	}
	if !d.strictUnknown && !guards && !d.presence && (d.representation == nil || !d.representation.Changed()) && !choice.HasEnum(reflect.TypeOf(destination)) && !hasDeclaredTimePolicy(reflect.TypeOf(destination)) && !hasDeclaredInputNames(reflect.TypeOf(destination), d.representation) {
		return json.Unmarshal(data, destination)
	}
	value := reflect.ValueOf(destination)
	if !value.IsValid() || value.Kind() != reflect.Pointer || value.IsNil() {
		return &json.InvalidUnmarshalError{Type: reflect.TypeOf(destination)}
	}
	value = value.Elem()
	metadata := d.representation
	if metadata == nil {
		plan, err := shape.CompileRewrite(value.Type())
		if err != nil {
			return err
		}
		metadata, err = plan.RewriteRepresentation(nil)
		if err != nil {
			return err
		}
	}
	if value.Type() != metadata.Original() {
		return fmt.Errorf("representation expects destination *%s, got %T", metadata.Original(), destination)
	}
	// Match encoding/json's syntax-before-mutation behavior, including discarded fields.
	var valid json.RawMessage
	if err := json.Unmarshal(data, &valid); err != nil {
		return err
	}
	return decodeStandardRepresentation(data, value, metadata, d.presence, guards, nil, d.strictUnknown)
}
func decodeStandardRepresentation(data []byte, value reflect.Value, metadata *shape.Representation, presence bool, guards bool, ancestors []format.InputScope, strictUnknown bool) error {
	if hasCustomUnmarshalType(value.Type()) || (!strictUnknown && !guards && !presence && (metadata == nil || !metadata.Changed()) && !choice.HasEnum(value.Type()) && !hasDeclaredTimePolicy(value.Type()) && !hasDeclaredInputNames(value.Type(), metadata)) {
		return json.Unmarshal(data, value.Addr().Interface())
	}
	if metadata == nil {
		plan, err := shape.CompileRewrite(value.Type())
		if err != nil {
			return err
		}
		metadata, err = plan.RewriteRepresentation(nil)
		if err != nil {
			return err
		}
	}
	raw := bytes.TrimSpace(data)
	if bytes.Equal(raw, []byte("null")) {
		return json.Unmarshal(raw, value.Addr().Interface())
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		return decodeStandardRepresentation(raw, value.Elem(), metadata.Elem(), presence, guards, ancestors, strictUnknown)
	}
	switch value.Kind() {
	case reflect.Struct:
		if len(raw) == 0 || raw[0] != '{' {
			return json.Unmarshal(raw, value.Addr().Interface())
		}
		plan := standardDecodePlanFor(value.Type(), metadata)
		if plan.err != nil {
			return plan.err
		}
		fields := plan.fields
		admitted, members, err := selectStandardInput(raw, plan.names, ancestors, guards)
		if err != nil {
			return err
		}
		next := ancestors
		if guards {
			goNames := make([]string, len(fields))
			for i, f := range fields {
				if f.Field.Tag.Get("internal") != "true" && f.Field.Tag.Get("setMarker") != "true" {
					goNames[i] = f.Field.Name
				}
			}
			next = append([]format.InputScope{inputPresence(plan.names, members, admitted, goNames)}, ancestors...)
		}
		occurrence := 0
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
			keep := admitted == nil || admitted[occurrence]
			occurrence++
			if !keep {
				continue
			}
			index := plan.names.Match(key)
			if index < 0 {
				if strictUnknown && first == nil {
					first = fmt.Errorf("json: unknown field %q", key)
				}
				continue
			}
			field := fields[index]
			if field.Field.Tag.Get("setMarker") == "true" {
				continue
			}
			target, err := standardFieldValue(value, field.Field.Index)
			if err != nil {
				return err
			}
			if err := plan.enumErrors[index]; err != nil {
				return err
			}
			enumeration := plan.enums[index]
			if plan.timeErrors[index] != nil {
				return plan.timeErrors[index]
			}
			if policy := plan.timePolicies[index]; policy != nil {
				err = assignDeclaredTime(target, policy, member)
			} else if enumeration != nil {
				if field.Quoted {
					return fmt.Errorf("enumAccept conflicts with json string option")
				}
				converted, e := enumeration.DecodeJSON(member)
				if e != nil {
					return e
				}
				err = json.Unmarshal(converted, target.Addr().Interface())
			} else if field.Quoted {
				err = decodeStandardQuoted(member, target)
			} else {
				err = decodeStandardRepresentation(member, target, metadata.FieldShapeByIndex(field.Field.Index), presence, guards, next, strictUnknown)
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
			if presence {
				markStandardPresence(value, field.Field.Index)
			}
		}
		return first
	case reflect.Slice, reflect.Array, reflect.Map:
		return decodeRawCollection(raw, value, metadata, false, func(member []byte, target reflect.Value, child *shape.Representation) error {
			return decodeStandardRepresentation(member, target, child, presence, guards, ancestors, strictUnknown)
		})
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

func markStandardPresence(value reflect.Value, index []int) {
	leaf := value.Type().FieldByIndex(index).Name
	for depth, position := range index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			return
		}
		for i := 0; i < value.NumField(); i++ {
			field := value.Type().Field(i)
			if field.Tag.Get("setMarker") != "true" {
				continue
			}
			holder := value.Field(i)
			if !holder.CanSet() {
				continue
			}
			typ := holder.Type()
			if typ.Kind() == reflect.Pointer {
				typ = typ.Elem()
			}
			if typ.Kind() != reflect.Struct {
				continue
			}
			flag, ok := typ.FieldByName(leaf)
			if !ok || flag.Type.Kind() != reflect.Bool {
				continue
			}
			if holder.Kind() == reflect.Pointer {
				if holder.IsNil() {
					holder.Set(reflect.New(typ))
				}
				holder = holder.Elem()
			}
			selected := holder.FieldByIndex(flag.Index)
			if selected.CanSet() {
				selected.SetBool(true)
			}
		}
		if depth+1 < len(index) {
			value = value.Field(position)
		}
	}
}

type standardDecodeKey struct {
	typ            reflect.Type
	representation *shape.Representation
}
type standardDecodePlan struct {
	names        *format.InputNameMatcher
	timePolicies []*format.Tag
	timeErrors   []error
	fields       []shape.JSONField
	enums        []*choice.Enum
	enumErrors   []error
	err          error
}

var standardDecodeCache = lru.New[standardDecodeKey, *standardDecodePlan](2048)

func standardDecodePlanFor(typ reflect.Type, metadata *shape.Representation) *standardDecodePlan {
	if metadata != nil && !metadata.Changed() {
		metadata = nil
	}
	return standardDecodeCache.GetOrCreate(standardDecodeKey{typ, metadata}, func() *standardDecodePlan {
		result := &standardDecodePlan{}
		if metadata != nil {
			result.fields, result.err = metadata.JSONFields()
		} else {
			result.fields, result.err = shape.Linked(typ).JSONFields()
		}
		if result.err != nil {
			return result
		}
		result.timePolicies = make([]*format.Tag, len(result.fields))
		result.timeErrors = make([]error, len(result.fields))
		result.enums = make([]*choice.Enum, len(result.fields))
		result.enumErrors = make([]error, len(result.fields))
		names := make([]format.InputName, len(result.fields))
		policies := make([]*format.Tag, len(result.fields))
		for i, field := range result.fields {
			policy, err := declaredInputPolicy(field.Field.Tag)
			policies[i] = policy
			aliases := ""
			if policy != nil {
				aliases = policy.InputNames
			}
			if err != nil {
				result.err = err
				return result
			}
			names[i] = format.InputName{Name: field.Name, Aliases: aliases}
			result.enums[i], result.enumErrors[i] = choice.ParseEnum(field.Field.Tag, field.Field.ReflectedType)
			result.timePolicies[i], result.timeErrors[i] = declaredTimePolicy(reflect.StructField{Type: field.Field.ReflectedType, Tag: field.Field.Tag})
		}
		result.names, result.err = format.NewInputNameMatcher(names, policies...)
		return result
	})
}
