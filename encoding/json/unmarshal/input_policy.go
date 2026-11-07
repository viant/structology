package unmarshal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/viant/tagly/format"
	"reflect"
	"strings"
)

func declaredInputPolicy(tag reflect.StructTag) (*format.Tag, error) {
	value := strings.ToLower(tag.Get("format"))
	if !strings.Contains(value, "inputnames") && !strings.Contains(value, "inputgroup") && !strings.Contains(value, "inputpriority") && !strings.Contains(value, "inputcase") && !strings.Contains(value, "inputunless") {
		return nil, nil
	}
	return format.Parse(tag)
}
func inputKind(raw []byte) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '{':
		return "object"
	case '[':
		return "array"
	case '"':
		return "string"
	case 'n':
		return "null"
	case 't', 'f':
		return "boolean"
	default:
		return "number"
	}
}
func selectStandardInput(raw []byte, names *format.InputNameMatcher, ancestors []format.InputScope, guards bool) ([]bool, []format.InputMember, error) {
	if !guards && !names.HasPolicies() {
		return nil, nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if _, err := d.Token(); err != nil {
		return nil, nil, err
	}
	var members []format.InputMember
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, nil, err
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, nil, err
		}
		members = append(members, format.InputMember{Name: key.(string), Kind: inputKind(value)})
	}
	if _, err := d.Token(); err != nil {
		return nil, nil, err
	}
	selected, err := names.SelectScoped(members, ancestors)
	return selected, members, err
}

// The native pre-scan uses the same parser and malformed policy as assignment.
// d is positioned immediately after the opening object delimiter.
func (e *Engine) selectNativeInput(d *scalarDecoder, plan *typePlan) ([]bool, []format.InputMember, error) {
	names := plan.inputNames
	if !e.guardEnabled && !names.HasPolicies() {
		return nil, nil, nil
	}
	scan := *d
	var members []format.InputMember
	for {
		scan.skipWS()
		if scan.pos >= len(scan.data) {
			return nil, nil, fmt.Errorf("unexpected EOF in object")
		}
		if scan.data[scan.pos] == '}' {
			selected, err := names.SelectScoped(members, e.inputScopes)
			return selected, members, err
		}
		key, err := scan.parseStringValue()
		if err != nil {
			return nil, nil, err
		}
		scan.skipWS()
		if scan.pos >= len(scan.data) || scan.data[scan.pos] != ':' {
			return nil, nil, fmt.Errorf("expected ':' at %d", scan.pos)
		}
		scan.pos++
		raw, err := scan.parseRawValue()
		if err != nil {
			return nil, nil, err
		}
		members = append(members, format.InputMember{Name: nativeInputName(plan, key), Kind: inputKind(raw)})
		scan.skipWS()
		if scan.pos >= len(scan.data) {
			return nil, nil, fmt.Errorf("unexpected EOF in object")
		}
		if scan.data[scan.pos] == '}' {
			selected, err := names.SelectScoped(members, e.inputScopes)
			return selected, members, err
		}
		if scan.data[scan.pos] != ',' {
			if e.MalformedPolicy == Tolerant && scan.data[scan.pos] == '"' {
				continue
			}
			return nil, nil, fmt.Errorf("expected ',' at %d", scan.pos)
		}
		scan.pos++
		scan.skipWS()
		if scan.pos < len(scan.data) && scan.data[scan.pos] == '}' && e.MalformedPolicy != Tolerant {
			return nil, nil, fmt.Errorf("unexpected trailing comma")
		}
	}
}
func parsedInputKind(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case map[string]interface{}:
		return "object"
	case []interface{}:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	default:
		return "number"
	}
}

func nativeInputName(plan *typePlan, key string) string {
	index, _, excluded := plan.inputNames.Classify(key)
	if index < 0 && !excluded {
		if field, ok := lookupField(plan, key); ok {
			return field.canonicalName
		}
	}
	return key
}
