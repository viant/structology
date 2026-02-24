package unmarshal

import (
	"bytes"
	"context"
	"encoding"
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/viant/structology/encoding/json/internal/lru"
	"github.com/viant/structology/encoding/json/internal/tagutil"
	"github.com/viant/xunsafe"
)

var (
	timeType            = reflect.TypeOf(time.Time{})
	stringMapType       = reflect.TypeOf(map[string]string(nil))
	stringSliceTy       = reflect.TypeOf([]string(nil))
	intSliceTy          = reflect.TypeOf([]int(nil))
	int64SliceTy        = reflect.TypeOf([]int64(nil))
	float64SliceTy      = reflect.TypeOf([]float64(nil))
	boolSliceTy         = reflect.TypeOf([]bool(nil))
	jsonUnmarshalerType = reflect.TypeOf((*stdjson.Unmarshaler)(nil)).Elem()
	textUnmarshalType   = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

type UnknownFieldPolicy int

const (
	IgnoreUnknown UnknownFieldPolicy = iota
	ErrorOnUnknown
)

type NumberPolicy int

const (
	CoerceNumbers NumberPolicy = iota
	ExactNumbers
)

type NullPolicy int

const (
	CompatNulls NullPolicy = iota
	StrictNulls
)

type DuplicateKeyPolicy int

const (
	LastWins DuplicateKeyPolicy = iota
	ErrorOnDuplicate
)

type MalformedPolicy int

const (
	Tolerant MalformedPolicy = iota
	FailFast
)

type ScannerHooks interface {
	SkipWhitespace(data []byte, pos int) int
	FindQuoteOrEscape(data []byte, pos int) (quotePos int, escapePos int)
	FindStructural(data []byte, pos int) int
}

type Engine struct {
	Ctx                context.Context
	Hooks              ScannerHooks
	UnknownFieldPolicy UnknownFieldPolicy
	NumberPolicy       NumberPolicy
	NullPolicy         NullPolicy
	DuplicateKeyPolicy DuplicateKeyPolicy
	MalformedPolicy    MalformedPolicy
	timeLayout         string
	caseKey            string
	compileName        func(string) string
	PathHook           func(ctx context.Context, holder unsafe.Pointer, path []string, field string, value interface{}) (interface{}, error)
}

func New(ctx context.Context, hooks ScannerHooks, unknown UnknownFieldPolicy, number NumberPolicy, nulls NullPolicy, duplicates DuplicateKeyPolicy, malformed MalformedPolicy, timeLayout string, caseKey string, compileName func(string) string, pathHook func(ctx context.Context, holder unsafe.Pointer, path []string, field string, value interface{}) (interface{}, error)) *Engine {
	if timeLayout == "" {
		timeLayout = time.RFC3339
	}
	return &Engine{
		Ctx:                ctx,
		Hooks:              hooks,
		UnknownFieldPolicy: unknown,
		NumberPolicy:       number,
		NullPolicy:         nulls,
		DuplicateKeyPolicy: duplicates,
		MalformedPolicy:    malformed,
		timeLayout:         timeLayout,
		caseKey:            caseKey,
		compileName:        compileName,
		PathHook:           pathHook,
	}
}

func (e *Engine) Unmarshal(data []byte, dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("nil destination")
	}
	if rt := reflect.TypeOf(dest); rt != nil && rt.Kind() == reflect.Ptr && rt.Elem() == timeType {
		parsed, err := decodeJSON(data, e.Hooks, e.DuplicateKeyPolicy, e.MalformedPolicy)
		if err != nil {
			return err
		}
		return assignParsed(dest, parsed, e)
	}
	if u, ok := dest.(stdjson.Unmarshaler); ok {
		return u.UnmarshalJSON(data)
	}
	if tu, ok := dest.(encoding.TextUnmarshaler); ok {
		parsed, err := decodeJSON(data, e.Hooks, e.DuplicateKeyPolicy, e.MalformedPolicy)
		if err != nil {
			return err
		}
		s, ok := parsed.(string)
		if !ok {
			return fmt.Errorf("expected string for TextUnmarshaler")
		}
		return tu.UnmarshalText([]byte(s))
	}
	rt := reflect.TypeOf(dest)
	if rt.Kind() == reflect.Ptr {
		target := rt.Elem()
		if target.Kind() == reflect.Struct {
			ptr := xunsafe.RefPointer(xunsafe.AsPointer(dest))
			return e.unmarshalStructFast(data, ptr, target)
		}
	}
	parsed, err := decodeJSON(data, e.Hooks, e.DuplicateKeyPolicy, e.MalformedPolicy)
	if err != nil {
		return err
	}
	return assignParsed(dest, parsed, e)
}

func (e *Engine) unmarshalStructFast(data []byte, ptr unsafe.Pointer, rType reflect.Type) error {
	d := &scalarDecoder{
		data:               data,
		hooks:              e.Hooks,
		duplicateKeyPolicy: e.DuplicateKeyPolicy,
		malformedPolicy:    e.MalformedPolicy,
	}
	d.skipWS()
	if d.pos >= len(d.data) || d.data[d.pos] != '{' {
		parsed, err := decodeJSON(data, e.Hooks, e.DuplicateKeyPolicy, e.MalformedPolicy)
		if err != nil {
			return err
		}
		return assignValue(ptr, reflect.PointerTo(rType), parsed, e)
	}
	structPtr := xunsafe.SafeDerefPointer(ptr, reflect.PointerTo(rType))
	if err := e.unmarshalStructFromDecoder(d, structPtr, rType); err != nil {
		return err
	}
	d.skipWS()
	if d.pos != len(d.data) {
		return fmt.Errorf("unexpected trailing data at %d", d.pos)
	}
	return nil
}

func (e *Engine) unmarshalStructFromDecoder(d *scalarDecoder, structPtr unsafe.Pointer, rType reflect.Type) error {
	d.skipWS()
	if d.pos >= len(d.data) || d.data[d.pos] != '{' {
		return fmt.Errorf("expected '{' at %d", d.pos)
	}
	d.pos++
	plan := planFor(rType, e.caseKey, e.compileName)
	if plan.presence != nil {
		_ = ensurePresenceHolder(structPtr, plan.presence)
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == '}' {
		d.pos++
		return nil
	}
	var seen map[string]struct{}
	if e.DuplicateKeyPolicy == ErrorOnDuplicate {
		seen = make(map[string]struct{}, len(plan.fieldsByName))
	}
	for {
		key, err := d.parseStringValue()
		if err != nil {
			return err
		}
		if seen != nil {
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate field %s at %d", key, d.pos)
			}
			seen[key] = struct{}{}
		}
		d.skipWS()
		if d.pos >= len(d.data) || d.data[d.pos] != ':' {
			return fmt.Errorf("expected ':' at %d", d.pos)
		}
		d.pos++
		fp, ok := lookupField(plan, key)
		if !ok {
			if e.UnknownFieldPolicy == ErrorOnUnknown {
				return fmt.Errorf("unknown field %s", key)
			}
			if _, err = d.parseValue(); err != nil {
				return err
			}
		} else if fp.ignore {
			if _, err = d.parseValue(); err != nil {
				return err
			}
		} else {
			fieldPtr := fp.resolve(structPtr)
			pathPushed := false
			if e.PathHook != nil {
				d.path = append(d.path, fp.name)
				pathPushed = true
			}
			hooksEnabled := e.PathHook != nil
			customUnmarshal := fp.hasCustomUnmarshal && !isTimeTypeOrPtr(fp.rType)
			streamNested := fp.rType.Kind() == reflect.Struct || (fp.rType.Kind() == reflect.Ptr && fp.rType.Elem().Kind() == reflect.Struct)
			if hooksEnabled && streamNested && !customUnmarshal {
				if handled, decodeErr := e.tryDecodeTypedField(d, fp, fieldPtr); handled {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					if decodeErr != nil {
						return decodeErr
					}
				} else {
					val, parseErr := d.parseValue()
					if parseErr != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return parseErr
					}
					if err = e.assignPlannedField(fieldPtr, fp, val); err != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return err
					}
				}
			} else if hooksEnabled {
				val, parseErr := d.parseValue()
				if parseErr != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return parseErr
				}
				if e.PathHook != nil {
					parentPath := d.path
					if len(parentPath) > 0 {
						parentPath = parentPath[:len(parentPath)-1]
					}
					val, err = e.PathHook(e.Ctx, structPtr, parentPath, fp.name, val)
					if err != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return err
					}
				}
				if err = e.assignPlannedField(fieldPtr, fp, val); err != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return err
				}
			} else if customUnmarshal {
				raw, rawErr := d.parseRawValue()
				if rawErr != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return rawErr
				}
				if handled, customErr := assignCustomFromRaw(fieldPtr, fp.rType, raw); handled {
					if customErr != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return customErr
					}
				} else {
					val, parseErr := decodeJSON(raw, e.Hooks, e.DuplicateKeyPolicy, e.MalformedPolicy)
					if parseErr != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return parseErr
					}
					if err = e.assignPlannedField(fieldPtr, fp, val); err != nil {
						if pathPushed && len(d.path) > 0 {
							d.path = d.path[:len(d.path)-1]
						}
						return err
					}
				}
			} else if handled, decodeErr := e.tryDecodeTypedField(d, fp, fieldPtr); handled {
				if decodeErr != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return decodeErr
				}
			} else {
				val, parseErr := d.parseValue()
				if parseErr != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return parseErr
				}
				if err = e.assignPlannedField(fieldPtr, fp, val); err != nil {
					if pathPushed && len(d.path) > 0 {
						d.path = d.path[:len(d.path)-1]
					}
					return err
				}
			}
			if pathPushed && len(d.path) > 0 {
				d.path = d.path[:len(d.path)-1]
			}
			if plan.presence != nil && fp.presenceFlag != nil {
				h := ensurePresenceHolder(structPtr, plan.presence)
				if h != nil {
					fp.presenceFlag.SetBool(h, true)
				}
			}
		}
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in object")
		}
		if d.data[d.pos] == '}' {
			d.pos++
			return nil
		}
		if d.data[d.pos] != ',' {
			if e.MalformedPolicy == Tolerant && d.data[d.pos] == '"' {
				// Compat mode: tolerate missing comma between object members.
				continue
			}
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if e.MalformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == '}' {
				d.pos++
				return nil
			}
		}
	}
}

func (e *Engine) tryDecodeTypedField(d *scalarDecoder, fp *fieldPlan, fieldPtr unsafe.Pointer) (bool, error) {
	rt := fp.rType
	switch rt.Kind() {
	case reflect.Struct:
		if fp.hasCustomUnmarshal {
			return false, nil
		}
		if rt == timeType {
			return false, nil
		}
		d.skipWS()
		if d.pos < len(d.data) && d.data[d.pos] == 'n' {
			if d.match("null") {
				if e.NullPolicy == StrictNulls {
					return true, fmt.Errorf("null is not allowed for %s", rt.String())
				}
				return true, nil
			}
		}
		return true, e.unmarshalStructFromDecoder(d, fieldPtr, rt)
	case reflect.Ptr:
		if fp.hasCustomUnmarshal {
			return false, nil
		}
		elem := rt.Elem()
		if elem.Kind() == reflect.Struct && elem != timeType {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					if e.NullPolicy == StrictNulls {
						return true, fmt.Errorf("null is not allowed for %s", rt.String())
					}
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := reflect.New(elem)
				target = unsafe.Pointer(alloc.Pointer())
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, e.unmarshalStructFromDecoder(d, target, elem)
		}
		if elem == stringSliceTy {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new([]string)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseStringArrayInto((*[]string)(target))
		}
		if elem == intSliceTy {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new([]int)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseIntArrayInto((*[]int)(target), e.NumberPolicy)
		}
		if elem == int64SliceTy {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new([]int64)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseInt64ArrayInto((*[]int64)(target), e.NumberPolicy)
		}
		if elem == float64SliceTy {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new([]float64)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseFloat64ArrayInto((*[]float64)(target), e.NumberPolicy)
		}
		if elem == boolSliceTy {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new([]bool)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseBoolArrayInto((*[]bool)(target))
		}
		if elem == stringMapType {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == 'n' {
				if d.match("null") {
					return true, nil
				}
			}
			target := *(*unsafe.Pointer)(fieldPtr)
			if target == nil {
				alloc := new(map[string]string)
				target = unsafe.Pointer(alloc)
				*(*unsafe.Pointer)(fieldPtr) = target
			}
			return true, d.parseStringMapInto((*map[string]string)(target))
		}
	case reflect.Slice:
		if rt == stringSliceTy {
			return true, d.parseStringArrayInto((*[]string)(fieldPtr))
		}
		if rt == intSliceTy {
			return true, d.parseIntArrayInto((*[]int)(fieldPtr), e.NumberPolicy)
		}
		if rt == int64SliceTy {
			return true, d.parseInt64ArrayInto((*[]int64)(fieldPtr), e.NumberPolicy)
		}
		if rt == float64SliceTy {
			return true, d.parseFloat64ArrayInto((*[]float64)(fieldPtr), e.NumberPolicy)
		}
		if rt == boolSliceTy {
			return true, d.parseBoolArrayInto((*[]bool)(fieldPtr))
		}
	case reflect.Map:
		if rt == stringMapType {
			return true, d.parseStringMapInto((*map[string]string)(fieldPtr))
		}
	}
	return false, nil
}

func hasCustomUnmarshalType(rt reflect.Type) bool {
	if isTimeTypeOrPtr(rt) {
		return false
	}
	if rt.Implements(jsonUnmarshalerType) || rt.Implements(textUnmarshalType) {
		return true
	}
	if rt.Kind() != reflect.Ptr {
		prt := reflect.PointerTo(rt)
		if prt.Implements(jsonUnmarshalerType) || prt.Implements(textUnmarshalType) {
			return true
		}
	}
	return false
}

func isTimeTypeOrPtr(rt reflect.Type) bool {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return rt == timeType
}

type scalarDecoder struct {
	data  []byte
	pos   int
	hooks ScannerHooks
	path  []string

	duplicateKeyPolicy DuplicateKeyPolicy
	malformedPolicy    MalformedPolicy
}

func decodeJSON(data []byte, hooks ScannerHooks, duplicates DuplicateKeyPolicy, malformed MalformedPolicy) (interface{}, error) {
	d := &scalarDecoder{data: data, hooks: hooks, duplicateKeyPolicy: duplicates, malformedPolicy: malformed}
	v, err := d.parseValue()
	if err != nil {
		return nil, err
	}
	d.skipWS()
	if d.pos != len(d.data) {
		return nil, fmt.Errorf("unexpected trailing data at %d", d.pos)
	}
	return v, nil
}

func (d *scalarDecoder) skipWS() { d.pos = d.hooks.SkipWhitespace(d.data, d.pos) }

func (d *scalarDecoder) parseValue() (interface{}, error) {
	d.skipWS()
	if d.pos >= len(d.data) {
		return nil, fmt.Errorf("unexpected EOF")
	}
	switch d.data[d.pos] {
	case '{':
		return d.parseObject()
	case '[':
		return d.parseArray()
	case '"':
		return d.parseString()
	case 't':
		if d.match("true") {
			return true, nil
		}
	case 'f':
		if d.match("false") {
			return false, nil
		}
	case 'n':
		if d.match("null") {
			return nil, nil
		}
	default:
		return d.parseNumber()
	}
	return nil, fmt.Errorf("invalid token at %d", d.pos)
}

func (d *scalarDecoder) parseRawValue() ([]byte, error) {
	d.skipWS()
	start := d.pos
	if err := d.skipRawValue(); err != nil {
		return nil, err
	}
	return d.data[start:d.pos], nil
}

func (d *scalarDecoder) skipRawValue() error {
	d.skipWS()
	if d.pos >= len(d.data) {
		return fmt.Errorf("unexpected EOF")
	}
	switch d.data[d.pos] {
	case '{':
		return d.skipRawObject()
	case '[':
		return d.skipRawArray()
	case '"':
		return d.skipRawString()
	case 't':
		if d.match("true") {
			return nil
		}
	case 'f':
		if d.match("false") {
			return nil
		}
	case 'n':
		if d.match("null") {
			return nil
		}
	default:
		return d.skipRawNumber()
	}
	return fmt.Errorf("invalid token at %d", d.pos)
}

func (d *scalarDecoder) skipRawObject() error {
	if d.pos >= len(d.data) || d.data[d.pos] != '{' {
		return fmt.Errorf("expected '{' at %d", d.pos)
	}
	d.pos++
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == '}' {
		d.pos++
		return nil
	}
	for {
		if err := d.skipRawString(); err != nil {
			return err
		}
		d.skipWS()
		if d.pos >= len(d.data) || d.data[d.pos] != ':' {
			return fmt.Errorf("expected ':' at %d", d.pos)
		}
		d.pos++
		if err := d.skipRawValue(); err != nil {
			return err
		}
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in object")
		}
		if d.data[d.pos] == '}' {
			d.pos++
			return nil
		}
		if d.data[d.pos] != ',' {
			if d.malformedPolicy == Tolerant && d.data[d.pos] == '"' {
				continue
			}
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == '}' {
				d.pos++
				return nil
			}
		}
	}
}

func (d *scalarDecoder) skipRawArray() error {
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		return nil
	}
	for {
		if err := d.skipRawValue(); err != nil {
			return err
		}
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				return nil
			}
		}
	}
}

func (d *scalarDecoder) skipRawString() error {
	if d.pos >= len(d.data) || d.data[d.pos] != '"' {
		return fmt.Errorf("expected string at %d", d.pos)
	}
	d.pos++
	escaped := false
	for d.pos < len(d.data) {
		c := d.data[d.pos]
		if c == '"' && !escaped {
			d.pos++
			return nil
		}
		if c == '\\' {
			escaped = !escaped
		} else {
			if c < 0x20 {
				return fmt.Errorf("invalid control character in string at %d", d.pos)
			}
			escaped = false
		}
		d.pos++
	}
	return fmt.Errorf("unterminated string")
}

func (d *scalarDecoder) skipRawNumber() error {
	start := d.pos
	for d.pos < len(d.data) {
		c := d.data[d.pos]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			d.pos++
			continue
		}
		break
	}
	if d.pos == start {
		return fmt.Errorf("invalid number at %d", d.pos)
	}
	return nil
}

func (d *scalarDecoder) match(token string) bool {
	end := d.pos + len(token)
	if end > len(d.data) {
		return false
	}
	if string(d.data[d.pos:end]) != token {
		return false
	}
	d.pos = end
	return true
}

func (d *scalarDecoder) parseObject() (map[string]interface{}, error) {
	d.pos++
	obj := make(map[string]interface{})
	var seen map[string]struct{}
	if d.duplicateKeyPolicy == ErrorOnDuplicate {
		seen = make(map[string]struct{})
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == '}' {
		d.pos++
		return obj, nil
	}
	for {
		keyAny, err := d.parseString()
		if err != nil {
			return nil, err
		}
		key := keyAny.(string)
		if seen != nil {
			if _, exists := seen[key]; exists {
				return nil, fmt.Errorf("duplicate field %s at %d", key, d.pos)
			}
			seen[key] = struct{}{}
		}
		d.skipWS()
		if d.pos >= len(d.data) || d.data[d.pos] != ':' {
			return nil, fmt.Errorf("expected ':' at %d", d.pos)
		}
		d.pos++
		val, err := d.parseValue()
		if err != nil {
			return nil, err
		}
		obj[key] = val
		d.skipWS()
		if d.pos >= len(d.data) {
			return nil, fmt.Errorf("unexpected EOF in object")
		}
		if d.data[d.pos] == '}' {
			d.pos++
			return obj, nil
		}
		if d.data[d.pos] != ',' {
			if d.malformedPolicy == Tolerant && d.data[d.pos] == '"' {
				// Compat mode: tolerate missing comma between object members.
				continue
			}
			return nil, fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == '}' {
				d.pos++
				return obj, nil
			}
		}
	}
}

func (d *scalarDecoder) parseArray() ([]interface{}, error) {
	d.pos++
	arr := make([]interface{}, 0)
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		return arr, nil
	}
	for {
		v, err := d.parseValue()
		if err != nil {
			return nil, err
		}
		arr = append(arr, v)
		d.skipWS()
		if d.pos >= len(d.data) {
			return nil, fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			return arr, nil
		}
		if d.data[d.pos] != ',' {
			return nil, fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				return arr, nil
			}
		}
	}
}

func (d *scalarDecoder) parseString() (interface{}, error) {
	s, err := d.parseStringValue()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (d *scalarDecoder) parseStringValue() (string, error) {
	if d.pos >= len(d.data) || d.data[d.pos] != '"' {
		return "", fmt.Errorf("expected string at %d", d.pos)
	}
	d.pos++
	start := d.pos
	quote, escape := d.hooks.FindQuoteOrEscape(d.data, start)
	if quote >= 0 && escape < 0 {
		// Copy decoded strings to avoid aliasing caller input buffer.
		s := string(d.data[start:quote])
		d.pos = quote + 1
		return s, nil
	}
	i := start
	escaped := false
	for i < len(d.data) {
		c := d.data[i]
		if c == '"' && !escaped {
			s, err := unescapeJSONString(d.data[start:i])
			if err != nil {
				return "", err
			}
			d.pos = i + 1
			return s, nil
		}
		if c == '\\' {
			escaped = !escaped
		} else {
			if c < 0x20 {
				return "", fmt.Errorf("invalid control character in string at %d", i)
			}
			escaped = false
		}
		i++
	}
	return "", fmt.Errorf("unterminated string")
}

func unescapeJSONString(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	needsUnescape := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' {
			needsUnescape = true
			break
		}
	}
	if !needsUnescape {
		return string(raw), nil
	}
	out := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c != '\\' {
			out = append(out, c)
			continue
		}
		i++
		if i >= len(raw) {
			return "", fmt.Errorf("invalid escape sequence")
		}
		switch raw[i] {
		case '"', '\\', '/':
			out = append(out, raw[i])
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'u':
			if i+4 >= len(raw) {
				return "", fmt.Errorf("invalid unicode escape")
			}
			r, ok := parseHex4(raw[i+1 : i+5])
			if !ok {
				return "", fmt.Errorf("invalid unicode escape")
			}
			i += 4
			if utf16.IsSurrogate(r) {
				// Expect a surrogate pair for valid supplementary code points.
				if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
					return "", fmt.Errorf("invalid surrogate pair")
				}
				r2, ok := parseHex4(raw[i+3 : i+7])
				if !ok {
					return "", fmt.Errorf("invalid surrogate pair")
				}
				decoded := utf16.DecodeRune(r, r2)
				if decoded == utf8.RuneError {
					return "", fmt.Errorf("invalid surrogate pair")
				}
				out = utf8.AppendRune(out, decoded)
				i += 6
				continue
			}
			out = utf8.AppendRune(out, r)
		default:
			return "", fmt.Errorf("invalid escape character %q", raw[i])
		}
	}
	return string(out), nil
}

func parseHex4(b []byte) (rune, bool) {
	if len(b) != 4 {
		return 0, false
	}
	var v rune
	for i := 0; i < 4; i++ {
		c := b[i]
		var d rune
		switch {
		case c >= '0' && c <= '9':
			d = rune(c - '0')
		case c >= 'a' && c <= 'f':
			d = rune(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = rune(c-'A') + 10
		default:
			return 0, false
		}
		v = (v << 4) | d
	}
	return v, true
}

func (d *scalarDecoder) parseKey() (string, error) {
	if d.pos >= len(d.data) || d.data[d.pos] != '"' {
		return "", fmt.Errorf("expected string key at %d", d.pos)
	}
	d.pos++
	start := d.pos
	quote, escape := d.hooks.FindQuoteOrEscape(d.data, start)
	if quote >= 0 && escape < 0 {
		// Key is used only during object traversal and not stored in decoded output;
		// safe to avoid copy on no-escape path.
		key := bytesToStringNoCopy(d.data[start:quote])
		d.pos = quote + 1
		return key, nil
	}
	i := start
	escaped := false
	for i < len(d.data) {
		c := d.data[i]
		if c == '"' && !escaped {
			key, err := unescapeJSONString(d.data[start:i])
			if err != nil {
				return "", err
			}
			d.pos = i + 1
			return key, nil
		}
		if c == '\\' {
			escaped = !escaped
		} else {
			if c < 0x20 {
				return "", fmt.Errorf("invalid control character in string at %d", i)
			}
			escaped = false
		}
		i++
	}
	return "", fmt.Errorf("unterminated key")
}

func (d *scalarDecoder) parseNumber() (interface{}, error) {
	start := d.pos
	for d.pos < len(d.data) {
		c := d.data[d.pos]
		if (c >= '0' && c <= '9') || c == '-' || c == '+' || c == '.' || c == 'e' || c == 'E' {
			d.pos++
			continue
		}
		break
	}
	raw := string(d.data[start:d.pos])
	if strings.ContainsAny(raw, ".eE") {
		f, err := strconvParseFloat(raw)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	i, err := strconvParseInt(raw)
	if err == nil {
		return i, nil
	}
	u, err := strconvParseUint(raw)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// tiny wrappers to keep strconv use localized for easy swap.
func strconvParseFloat(raw string) (float64, error) { return strconv.ParseFloat(raw, 64) }
func strconvParseInt(raw string) (int64, error)     { return strconv.ParseInt(raw, 10, 64) }
func strconvParseUint(raw string) (uint64, error)   { return strconv.ParseUint(raw, 10, 64) }

func (d *scalarDecoder) parseStringArrayInto(dst *[]string) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make([]string, 0, 4)
	} else {
		out = out[:0]
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		d.skipWS()
		item, err := d.parseStringValue()
		if err != nil {
			return err
		}
		out = append(out, item)
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

func (d *scalarDecoder) parseStringMapInto(dst *map[string]string) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '{' {
		return fmt.Errorf("expected '{' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make(map[string]string, 4)
	} else {
		for k := range out {
			delete(out, k)
		}
	}
	var seen map[string]struct{}
	if d.duplicateKeyPolicy == ErrorOnDuplicate {
		seen = make(map[string]struct{})
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == '}' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		key, err := d.parseStringValue()
		if err != nil {
			return err
		}
		if seen != nil {
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate field %s at %d", key, d.pos)
			}
			seen[key] = struct{}{}
		}
		d.skipWS()
		if d.pos >= len(d.data) || d.data[d.pos] != ':' {
			return fmt.Errorf("expected ':' at %d", d.pos)
		}
		d.pos++
		d.skipWS()
		val, err := d.parseStringValue()
		if err != nil {
			return err
		}
		out[key] = val
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in object")
		}
		if d.data[d.pos] == '}' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			if d.malformedPolicy == Tolerant && d.data[d.pos] == '"' {
				continue
			}
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == '}' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

func (d *scalarDecoder) parseIntArrayInto(dst *[]int, policy NumberPolicy) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make([]int, 0, 4)
	} else {
		out = out[:0]
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		v, err := d.parseValue()
		if err != nil {
			return err
		}
		n, err := asInt(v, policy)
		if err != nil {
			return err
		}
		out = append(out, n)
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

func (d *scalarDecoder) parseInt64ArrayInto(dst *[]int64, policy NumberPolicy) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make([]int64, 0, 4)
	} else {
		out = out[:0]
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		v, err := d.parseValue()
		if err != nil {
			return err
		}
		n, err := asInt64(v, policy)
		if err != nil {
			return err
		}
		out = append(out, n)
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

func (d *scalarDecoder) parseFloat64ArrayInto(dst *[]float64, policy NumberPolicy) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make([]float64, 0, 4)
	} else {
		out = out[:0]
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		v, err := d.parseValue()
		if err != nil {
			return err
		}
		n, err := asFloat64(v, policy)
		if err != nil {
			return err
		}
		out = append(out, n)
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

func (d *scalarDecoder) parseBoolArrayInto(dst *[]bool) error {
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == 'n' {
		if d.match("null") {
			*dst = nil
			return nil
		}
	}
	if d.pos >= len(d.data) || d.data[d.pos] != '[' {
		return fmt.Errorf("expected '[' at %d", d.pos)
	}
	d.pos++
	out := *dst
	if out == nil {
		out = make([]bool, 0, 4)
	} else {
		out = out[:0]
	}
	d.skipWS()
	if d.pos < len(d.data) && d.data[d.pos] == ']' {
		d.pos++
		*dst = out
		return nil
	}
	for {
		v, err := d.parseValue()
		if err != nil {
			return err
		}
		b, ok := v.(bool)
		if !ok {
			return fmt.Errorf("expected bool")
		}
		out = append(out, b)
		d.skipWS()
		if d.pos >= len(d.data) {
			return fmt.Errorf("unexpected EOF in array")
		}
		if d.data[d.pos] == ']' {
			d.pos++
			*dst = out
			return nil
		}
		if d.data[d.pos] != ',' {
			return fmt.Errorf("expected ',' at %d", d.pos)
		}
		d.pos++
		if d.malformedPolicy == Tolerant {
			d.skipWS()
			if d.pos < len(d.data) && d.data[d.pos] == ']' {
				d.pos++
				*dst = out
				return nil
			}
		}
	}
}

type typePlan struct {
	rType        reflect.Type
	fieldsByName map[string]*fieldPlan
	fieldsByFold map[uint64][]foldField
	presence     *presencePlan
}

type foldField struct {
	key string
	fp  *fieldPlan
}

type fieldPlan struct {
	name               string
	xField             *xunsafe.Field
	rType              reflect.Type
	ignore             bool
	timeLayout         string
	hasCustomUnmarshal bool
	presenceFlag       *xunsafe.Field
	resolve            func(root unsafe.Pointer) unsafe.Pointer
}

type presencePlan struct {
	holder     *xunsafe.Field
	holderType reflect.Type
	flags      map[string]*xunsafe.Field
}

type planCacheKey struct {
	rType   reflect.Type
	caseKey string
}

var planCache = lru.New[planCacheKey, *typePlan](2048)

func planFor(rType reflect.Type, caseKey string, compileName func(string) string) *typePlan {
	if rType.Kind() == reflect.Ptr {
		rType = rType.Elem()
	}
	key := planCacheKey{rType: rType, caseKey: caseKey}
	if v, ok := planCache.Get(key); ok {
		return v
	}
	p := buildPlan(rType, compileName)
	planCache.Set(key, p)
	return p
}

func buildPlan(rType reflect.Type, compileName func(string) string) *typePlan {
	p := &typePlan{rType: rType, fieldsByName: map[string]*fieldPlan{}, fieldsByFold: map[uint64][]foldField{}}
	if rType.Kind() != reflect.Struct {
		return p
	}
	addField := func(name string, fp *fieldPlan) {
		if _, ok := p.fieldsByName[name]; !ok {
			p.fieldsByName[name] = fp
		}
		h := foldedHash(name)
		candidates := p.fieldsByFold[h]
		for _, candidate := range candidates {
			if candidate.fp == fp && candidate.key == name {
				return
			}
		}
		p.fieldsByFold[h] = append(candidates, foldField{key: name, fp: fp})
	}

	buildResolver := func(chain []*xunsafe.Field) func(unsafe.Pointer) unsafe.Pointer {
		return func(root unsafe.Pointer) unsafe.Pointer {
			current := root
			for i, f := range chain {
				ptr := f.Pointer(current)
				if i == len(chain)-1 {
					return ptr
				}
				if f.Type.Kind() == reflect.Ptr {
					next := (*unsafe.Pointer)(ptr)
					if *next == nil {
						alloc := reflect.New(f.Type.Elem())
						*next = unsafe.Pointer(alloc.Pointer())
					}
					current = *next
				} else {
					current = ptr
				}
			}
			return current
		}
	}

	var collect func(t reflect.Type, parent []*xunsafe.Field, topLevel bool)
	collect = func(t reflect.Type, parent []*xunsafe.Field, topLevel bool) {
		for i := 0; i < t.NumField(); i++ {
			sf := t.Field(i)
			if sf.PkgPath != "" {
				continue
			}
			if sf.Tag.Get("setMarker") == "true" && topLevel {
				if sf.Type.Kind() == reflect.Struct || (sf.Type.Kind() == reflect.Ptr && sf.Type.Elem().Kind() == reflect.Struct) {
					p.presence = &presencePlan{holder: xunsafe.NewField(sf), holderType: sf.Type, flags: map[string]*xunsafe.Field{}}
					pt := sf.Type
					if pt.Kind() == reflect.Ptr {
						pt = pt.Elem()
					}
					for j := 0; j < pt.NumField(); j++ {
						mf := pt.Field(j)
						if mf.Type.Kind() == reflect.Bool {
							p.presence.flags[mf.Name] = xunsafe.NewField(mf)
						}
					}
				}
				continue
			}

			resolved := tagutil.ResolveFieldTag(sf)
			fTag := resolved.Format
			ignore := resolved.Ignore
			inline := resolved.Inline
			xf := xunsafe.NewField(sf)
			chain := append(append([]*xunsafe.Field{}, parent...), xf)

			if inline && !ignore {
				inlineType := sf.Type
				if inlineType.Kind() == reflect.Ptr {
					inlineType = inlineType.Elem()
				}
				if inlineType.Kind() == reflect.Struct {
					collect(inlineType, chain, false)
					continue
				}
			}

			name := resolved.Name
			explicit := resolved.Explicit
			fp := &fieldPlan{
				name:               sf.Name,
				xField:             xf,
				rType:              sf.Type,
				ignore:             ignore,
				timeLayout:         fTag.TimeLayout,
				hasCustomUnmarshal: hasCustomUnmarshalType(sf.Type),
				resolve:            buildResolver(chain),
			}
			addField(name, fp)
			if compileName != nil && !explicit {
				alias := compileName(name)
				if alias != "" && alias != name {
					addField(alias, fp)
				}
			}
		}
	}

	collect(rType, nil, true)
	if p.presence != nil {
		seen := map[*fieldPlan]struct{}{}
		for _, fp := range p.fieldsByName {
			if _, ok := seen[fp]; ok {
				continue
			}
			seen[fp] = struct{}{}
			fp.presenceFlag = p.presence.flags[fp.name]
		}
	}
	return p
}

func lookupField(plan *typePlan, key string) (*fieldPlan, bool) {
	if fp, ok := plan.fieldsByName[key]; ok {
		return fp, true
	}
	candidates := plan.fieldsByFold[foldedHash(key)]
	for _, candidate := range candidates {
		if strings.EqualFold(candidate.key, key) {
			return candidate.fp, true
		}
	}
	return nil, false
}

func foldedHash(s string) uint64 {
	const (
		offset64 = 1469598103934665603
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		h ^= uint64(c)
		h *= prime64
	}
	return h
}

func assignParsed(dest interface{}, parsed interface{}, e *Engine) error {
	rt := reflect.TypeOf(dest)
	if rt.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be pointer")
	}
	rootPtr := xunsafe.AsPointer(dest)
	// assignValue expects **T when rt is *T.
	rootPtr = xunsafe.RefPointer(rootPtr)
	return assignValue(rootPtr, rt, parsed, e)
}

func (e *Engine) assignPlannedField(ptr unsafe.Pointer, fp *fieldPlan, parsed interface{}) error {
	if fp != nil && fp.timeLayout != "" {
		if handled, err := assignTimeWithLayout(ptr, fp.rType, parsed, fp.timeLayout, e.NullPolicy); handled {
			return err
		}
	}
	return assignValue(ptr, fp.rType, parsed, e)
}

func assignValue(ptr unsafe.Pointer, rt reflect.Type, parsed interface{}, e *Engine) error {
	if handled, err := tryAssignCustomUnmarshal(ptr, rt, parsed, e); handled || err != nil {
		return err
	}
	if rt.Kind() == reflect.Ptr {
		if parsed == nil {
			return nil
		}
		inner := xunsafe.SafeDerefPointer(ptr, rt)
		return assignValue(inner, rt.Elem(), parsed, e)
	}
	if parsed == nil {
		if e.NullPolicy == StrictNulls && !isNullableKind(rt.Kind()) {
			return fmt.Errorf("null is not allowed for %s", rt.String())
		}
		return nil
	}
	switch rt.Kind() {
	case reflect.Struct:
		if rt == reflect.TypeOf(time.Time{}) {
			s, ok := parsed.(string)
			if !ok {
				return fmt.Errorf("expected time string")
			}
			tm, err := time.Parse(e.timeLayout, s)
			if err != nil {
				return err
			}
			*xunsafe.AsTimePtr(ptr) = tm
			return nil
		}
		obj, ok := parsed.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected object")
		}
		plan := planFor(rt, e.caseKey, e.compileName)
		if plan.presence != nil {
			_ = ensurePresenceHolder(ptr, plan.presence)
		}
		for key, val := range obj {
			fp, ok := lookupField(plan, key)
			if !ok {
				if e.UnknownFieldPolicy == ErrorOnUnknown {
					return fmt.Errorf("unknown field %s", key)
				}
				continue
			}
			if fp.ignore {
				continue
			}
			if e.PathHook != nil {
				transformed, hookErr := e.PathHook(e.Ctx, ptr, nil, fp.name, val)
				if hookErr != nil {
					return hookErr
				}
				val = transformed
			}
			if err := e.assignPlannedField(fp.resolve(ptr), fp, val); err != nil {
				return err
			}
			if plan.presence != nil && fp.presenceFlag != nil {
				h := ensurePresenceHolder(ptr, plan.presence)
				if h != nil {
					fp.presenceFlag.SetBool(h, true)
				}
			}
		}
		return nil
	case reflect.String:
		s, ok := parsed.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		*xunsafe.AsStringPtr(ptr) = s
		return nil
	case reflect.Bool:
		b, ok := parsed.(bool)
		if !ok {
			return fmt.Errorf("expected bool")
		}
		*xunsafe.AsBoolPtr(ptr) = b
		return nil
	case reflect.Int:
		i, err := asInt(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsIntPtr(ptr) = i
		return nil
	case reflect.Int8:
		i, err := asInt(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && (i < -128 || i > 127) {
			return fmt.Errorf("number out of range for int8: %d", i)
		}
		*xunsafe.AsInt8Ptr(ptr) = int8(i)
		return nil
	case reflect.Int16:
		i, err := asInt(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && (i < -32768 || i > 32767) {
			return fmt.Errorf("number out of range for int16: %d", i)
		}
		*xunsafe.AsInt16Ptr(ptr) = int16(i)
		return nil
	case reflect.Int32:
		i, err := asInt(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && (i < -2147483648 || i > 2147483647) {
			return fmt.Errorf("number out of range for int32: %d", i)
		}
		*xunsafe.AsInt32Ptr(ptr) = int32(i)
		return nil
	case reflect.Int64:
		i, err := asInt64(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsInt64Ptr(ptr) = i
		return nil
	case reflect.Uint:
		u, err := asUint(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsUintPtr(ptr) = u
		return nil
	case reflect.Uint8:
		u, err := asUint(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && u > 255 {
			return fmt.Errorf("number out of range for uint8: %d", u)
		}
		*xunsafe.AsUint8Ptr(ptr) = uint8(u)
		return nil
	case reflect.Uint16:
		u, err := asUint(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && u > 65535 {
			return fmt.Errorf("number out of range for uint16: %d", u)
		}
		*xunsafe.AsUint16Ptr(ptr) = uint16(u)
		return nil
	case reflect.Uint32:
		u, err := asUint(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		if e.NumberPolicy == ExactNumbers && u > 4294967295 {
			return fmt.Errorf("number out of range for uint32: %d", u)
		}
		*xunsafe.AsUint32Ptr(ptr) = uint32(u)
		return nil
	case reflect.Uint64:
		u, err := asUint64(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsUint64Ptr(ptr) = u
		return nil
	case reflect.Float32:
		f, err := asFloat64(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsFloat32Ptr(ptr) = float32(f)
		return nil
	case reflect.Float64:
		f, err := asFloat64(parsed, e.NumberPolicy)
		if err != nil {
			return err
		}
		*xunsafe.AsFloat64Ptr(ptr) = f
		return nil
	case reflect.Interface:
		reflect.NewAt(rt, ptr).Elem().Set(reflect.ValueOf(parsed))
		return nil
	case reflect.Slice:
		if parsed == nil {
			reflect.NewAt(rt, ptr).Elem().Set(reflect.Zero(rt))
			return nil
		}
		items, ok := parsed.([]interface{})
		if !ok {
			return fmt.Errorf("expected array")
		}
		slice := reflect.MakeSlice(rt, len(items), len(items))
		elemType := rt.Elem()
		for i := 0; i < len(items); i++ {
			elem := reflect.New(elemType)
			if err := assignValue(xunsafe.AsPointer(elem.Interface()), elemType, items[i], e); err != nil {
				return err
			}
			slice.Index(i).Set(elem.Elem())
		}
		reflect.NewAt(rt, ptr).Elem().Set(slice)
		return nil
	case reflect.Array:
		if parsed == nil {
			return nil
		}
		items, ok := parsed.([]interface{})
		if !ok {
			return fmt.Errorf("expected array")
		}
		arr := reflect.NewAt(rt, ptr).Elem()
		elemType := rt.Elem()
		limit := len(items)
		if limit > arr.Len() {
			limit = arr.Len()
		}
		for i := 0; i < limit; i++ {
			elem := reflect.New(elemType)
			if err := assignValue(xunsafe.AsPointer(elem.Interface()), elemType, items[i], e); err != nil {
				return err
			}
			arr.Index(i).Set(elem.Elem())
		}
		return nil
	case reflect.Map:
		if parsed == nil {
			reflect.NewAt(rt, ptr).Elem().Set(reflect.Zero(rt))
			return nil
		}
		obj, ok := parsed.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected object")
		}
		if rt.Key().Kind() != reflect.String {
			return fmt.Errorf("unsupported map key kind: %s", rt.Key().Kind())
		}
		m := reflect.MakeMapWithSize(rt, len(obj))
		elemType := rt.Elem()
		for key, val := range obj {
			elem := reflect.New(elemType)
			if err := assignValue(xunsafe.AsPointer(elem.Interface()), elemType, val, e); err != nil {
				return err
			}
			mapKey := reflect.New(rt.Key()).Elem()
			mapKey.SetString(key)
			m.SetMapIndex(mapKey, elem.Elem())
		}
		reflect.NewAt(rt, ptr).Elem().Set(m)
		return nil
	}
	return nil
}

func tryAssignCustomUnmarshal(ptr unsafe.Pointer, rt reflect.Type, parsed interface{}, e *Engine) (bool, error) {
	if isTimeTypeOrPtr(rt) {
		return false, nil
	}
	if rt.Kind() == reflect.Ptr {
		implements := rt.Implements(reflect.TypeOf((*stdjson.Unmarshaler)(nil)).Elem()) || rt.Implements(reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem())
		if !implements {
			return false, nil
		}
		if parsed == nil {
			return false, nil
		}
		holder := reflect.NewAt(rt, ptr).Elem()
		if holder.IsNil() {
			holder.Set(reflect.New(rt.Elem()))
		}
		if holder.CanInterface() {
			if u, ok := holder.Interface().(stdjson.Unmarshaler); ok {
				data, err := stdjson.Marshal(parsed)
				if err != nil {
					return true, err
				}
				return true, u.UnmarshalJSON(data)
			}
			if tu, ok := holder.Interface().(encoding.TextUnmarshaler); ok {
				s, ok := parsed.(string)
				if !ok {
					return true, fmt.Errorf("expected string for TextUnmarshaler")
				}
				return true, tu.UnmarshalText([]byte(s))
			}
		}
		return false, nil
	}

	value := reflect.NewAt(rt, ptr).Elem()
	if value.CanAddr() && value.Addr().CanInterface() {
		if u, ok := value.Addr().Interface().(stdjson.Unmarshaler); ok {
			data, err := stdjson.Marshal(parsed)
			if err != nil {
				return true, err
			}
			return true, u.UnmarshalJSON(data)
		}
		if tu, ok := value.Addr().Interface().(encoding.TextUnmarshaler); ok {
			s, ok := parsed.(string)
			if !ok {
				return true, fmt.Errorf("expected string for TextUnmarshaler")
			}
			return true, tu.UnmarshalText([]byte(s))
		}
	}
	if value.CanInterface() {
		if u, ok := value.Interface().(stdjson.Unmarshaler); ok {
			data, err := stdjson.Marshal(parsed)
			if err != nil {
				return true, err
			}
			return true, u.UnmarshalJSON(data)
		}
		if tu, ok := value.Interface().(encoding.TextUnmarshaler); ok {
			s, ok := parsed.(string)
			if !ok {
				return true, fmt.Errorf("expected string for TextUnmarshaler")
			}
			return true, tu.UnmarshalText([]byte(s))
		}
	}
	return false, nil
}

func assignCustomFromRaw(ptr unsafe.Pointer, rt reflect.Type, raw []byte) (bool, error) {
	if isTimeTypeOrPtr(rt) {
		return false, nil
	}
	if rt.Kind() == reflect.Ptr {
		implements := rt.Implements(jsonUnmarshalerType) || rt.Implements(textUnmarshalType)
		if !implements {
			return false, nil
		}
		if bytes.Equal(raw, []byte("null")) {
			return true, nil
		}
		holder := reflect.NewAt(rt, ptr).Elem()
		if holder.IsNil() {
			holder.Set(reflect.New(rt.Elem()))
		}
		if holder.CanInterface() {
			if u, ok := holder.Interface().(stdjson.Unmarshaler); ok {
				return true, u.UnmarshalJSON(raw)
			}
			if tu, ok := holder.Interface().(encoding.TextUnmarshaler); ok {
				var s string
				if err := stdjson.Unmarshal(raw, &s); err != nil {
					return true, err
				}
				return true, tu.UnmarshalText([]byte(s))
			}
		}
		return false, nil
	}

	value := reflect.NewAt(rt, ptr).Elem()
	if value.CanAddr() && value.Addr().CanInterface() {
		if u, ok := value.Addr().Interface().(stdjson.Unmarshaler); ok {
			return true, u.UnmarshalJSON(raw)
		}
		if tu, ok := value.Addr().Interface().(encoding.TextUnmarshaler); ok {
			var s string
			if err := stdjson.Unmarshal(raw, &s); err != nil {
				return true, err
			}
			return true, tu.UnmarshalText([]byte(s))
		}
	}
	if value.CanInterface() {
		if u, ok := value.Interface().(stdjson.Unmarshaler); ok {
			return true, u.UnmarshalJSON(raw)
		}
		if tu, ok := value.Interface().(encoding.TextUnmarshaler); ok {
			var s string
			if err := stdjson.Unmarshal(raw, &s); err != nil {
				return true, err
			}
			return true, tu.UnmarshalText([]byte(s))
		}
	}
	return false, nil
}

func assignTimeWithLayout(ptr unsafe.Pointer, rt reflect.Type, parsed interface{}, layout string, nullPolicy NullPolicy) (bool, error) {
	if rt != timeType && !(rt.Kind() == reflect.Ptr && rt.Elem() == timeType) {
		return false, nil
	}
	if layout == "" {
		layout = time.RFC3339
	}
	if parsed == nil {
		if rt == timeType && nullPolicy == StrictNulls {
			return true, fmt.Errorf("null is not allowed for %s", rt.String())
		}
		return true, nil
	}
	s, ok := parsed.(string)
	if !ok {
		return true, fmt.Errorf("expected time string")
	}
	tm, err := time.Parse(layout, s)
	if err != nil {
		return true, err
	}
	if rt == timeType {
		*xunsafe.AsTimePtr(ptr) = tm
		return true, nil
	}
	inner := xunsafe.SafeDerefPointer(ptr, rt)
	*xunsafe.AsTimePtr(inner) = tm
	return true, nil
}

func asInt(v interface{}, policy NumberPolicy) (int, error) {
	i, err := asInt64(v, policy)
	return int(i), err
}
func asInt64(v interface{}, policy NumberPolicy) (int64, error) {
	switch a := v.(type) {
	case int64:
		return a, nil
	case int:
		return int64(a), nil
	case uint64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected integer")
		}
		return int64(a), nil
	case float64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected integer")
		}
		return int64(a), nil
	}
	return 0, fmt.Errorf("expected integer")
}
func asUint(v interface{}, policy NumberPolicy) (uint, error) {
	u, err := asUint64(v, policy)
	return uint(u), err
}
func asUint64(v interface{}, policy NumberPolicy) (uint64, error) {
	switch a := v.(type) {
	case uint64:
		return a, nil
	case int64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected unsigned integer")
		}
		return uint64(a), nil
	case int:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected unsigned integer")
		}
		return uint64(a), nil
	case float64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected unsigned integer")
		}
		return uint64(a), nil
	}
	return 0, fmt.Errorf("expected unsigned integer")
}
func asFloat64(v interface{}, policy NumberPolicy) (float64, error) {
	switch a := v.(type) {
	case float64:
		return a, nil
	case int64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected number")
		}
		return float64(a), nil
	case uint64:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected number")
		}
		return float64(a), nil
	case int:
		if policy == ExactNumbers {
			return 0, fmt.Errorf("expected number")
		}
		return float64(a), nil
	}
	return 0, fmt.Errorf("expected number")
}

func isNullableKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Ptr, reflect.Interface, reflect.Map, reflect.Slice:
		return true
	default:
		return false
	}
}

func bytesToStringNoCopy(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func ensurePresenceHolder(structPtr unsafe.Pointer, p *presencePlan) unsafe.Pointer {
	if p.holderType.Kind() != reflect.Ptr {
		return p.holder.Pointer(structPtr)
	}
	holderPtr := p.holder.ValuePointer(structPtr)
	if holderPtr != nil {
		return holderPtr
	}
	// Typical case: pointer holder. Allocate only when first marker update is needed.
	holderVal := reflect.New(p.holderType.Elem()).Interface()
	p.holder.SetValue(structPtr, holderVal)
	return p.holder.ValuePointer(structPtr)
}
