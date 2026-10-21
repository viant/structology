package convert

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"time"
)

var timeType = reflect.TypeOf(time.Time{})

func Convert(raw string, t reflect.Type, layout string) (any, bool, error) {
	if t.Kind() == reflect.Pointer {
		value, _, err := Convert(raw, t.Elem(), layout)
		if err != nil {
			return nil, false, err
		}
		ptr := reflect.New(t.Elem())
		ptr.Elem().Set(reflect.ValueOf(value))
		return ptr.Interface(), false, nil
	}
	value := reflect.New(t).Elem()
	if t == timeType {
		if layout == "" {
			layout = time.RFC3339
		}
		parsed, err := time.Parse(layout, raw)
		return parsed, false, err
	}
	switch t.Kind() {
	case reflect.String:
		value.SetString(raw)
	case reflect.Bool:
		v, e := strconv.ParseBool(raw)
		if e != nil {
			return nil, false, e
		}
		value.SetBool(v)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if raw != "" {
			v, e := strconv.ParseInt(raw, 10, t.Bits())
			if e != nil {
				return nil, false, e
			}
			value.SetInt(v)
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if raw != "" {
			v, e := strconv.ParseUint(raw, 10, t.Bits())
			if e != nil {
				return nil, false, e
			}
			value.SetUint(v)
		}
	case reflect.Float32, reflect.Float64:
		if raw != "" {
			v, e := strconv.ParseFloat(raw, t.Bits())
			if e != nil {
				return nil, false, e
			}
			value.SetFloat(v)
		}
	default:
		if !value.CanAddr() {
			return nil, false, fmt.Errorf("unsupported CSV type %v", t)
		}
		if err := json.Unmarshal([]byte(raw), value.Addr().Interface()); err != nil {
			return nil, false, err
		}
	}
	return value.Interface(), false, nil
}
