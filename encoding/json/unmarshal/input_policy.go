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
	if !strings.Contains(value, "inputnames") && !strings.Contains(value, "inputgroup") && !strings.Contains(value, "inputpriority") && !strings.Contains(value, "inputcase") {
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
func selectStandardInput(raw []byte, names *format.InputNameMatcher) ([]bool, error) {
	if !names.HasPolicies() {
		return nil, nil
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	var members []format.InputMember
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return nil, err
		}
		var value json.RawMessage
		if err = d.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, format.InputMember{Name: key.(string), Kind: inputKind(value)})
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	return names.Select(members), nil
}

// The native pre-scan uses the same parser and malformed policy as assignment.
// d is positioned immediately after the opening object delimiter.
func (e *Engine) selectNativeInput(d *scalarDecoder, plan *typePlan) ([]bool, error) {
	names := plan.inputNames
	if !names.HasPolicies() {
		return nil, nil
	}
	scan := *d
	var members []format.InputMember
	for {
		scan.skipWS()
		if scan.pos >= len(scan.data) {
			return nil, fmt.Errorf("unexpected EOF in object")
		}
		if scan.data[scan.pos] == '}' {
			return names.Select(members), nil
		}
		key, err := scan.parseStringValue()
		if err != nil {
			return nil, err
		}
		scan.skipWS()
		if scan.pos >= len(scan.data) || scan.data[scan.pos] != ':' {
			return nil, fmt.Errorf("expected ':' at %d", scan.pos)
		}
		scan.pos++
		raw, err := scan.parseRawValue()
		if err != nil {
			return nil, err
		}
		members = append(members, format.InputMember{Name: nativeInputName(plan, key), Kind: inputKind(raw)})
		scan.skipWS()
		if scan.pos >= len(scan.data) {
			return nil, fmt.Errorf("unexpected EOF in object")
		}
		if scan.data[scan.pos] == '}' {
			return names.Select(members), nil
		}
		if scan.data[scan.pos] != ',' {
			if e.MalformedPolicy == Tolerant && scan.data[scan.pos] == '"' {
				continue
			}
			return nil, fmt.Errorf("expected ',' at %d", scan.pos)
		}
		scan.pos++
		scan.skipWS()
		if scan.pos < len(scan.data) && scan.data[scan.pos] == '}' && e.MalformedPolicy != Tolerant {
			return nil, fmt.Errorf("unexpected trailing comma")
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
