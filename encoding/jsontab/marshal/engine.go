package marshal

import (
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"time"
	"unsafe"

	"github.com/viant/structology/encoding/jsontab/internal/plan"
)

var timeType = reflect.TypeOf(time.Time{})

type Engine struct {
	tagName     string
	caseKey     string
	compileName func(string) string
	timeLayout  string
}

func New(tagName, caseKey string, compileName func(string) string, timeLayout string) *Engine {
	if tagName == "" {
		tagName = "csvName"
	}
	if timeLayout == "" {
		timeLayout = time.RFC3339
	}
	return &Engine{tagName: tagName, caseKey: caseKey, compileName: compileName, timeLayout: timeLayout}
}

func (e *Engine) Marshal(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	rv := reflect.ValueOf(value)
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return []byte("null"), nil
		}
		rv = rv.Elem()
	}

	var rows []reflect.Value
	switch rv.Kind() {
	case reflect.Slice:
		rows = make([]reflect.Value, 0, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			item := deref(rv.Index(i))
			if !item.IsValid() || item.Kind() != reflect.Struct {
				continue
			}
			rows = append(rows, item)
		}
	case reflect.Struct:
		rows = []reflect.Value{rv}
	default:
		return nil, fmt.Errorf("unsupported root kind: %s", rv.Kind())
	}

	if len(rows) == 0 {
		return stdjson.Marshal([]interface{}{})
	}
	p := plan.For(rows[0].Type(), e.tagName, e.caseKey, e.compileName)
	table, err := e.marshalTable(p, rows)
	if err != nil {
		return nil, err
	}
	return stdjson.Marshal(table)
}

func (e *Engine) marshalTable(p *plan.Type, rows []reflect.Value) ([]interface{}, error) {
	out := make([]interface{}, 0, len(rows)+1)
	headers := make([]interface{}, len(p.Headers))
	for i := range p.Headers {
		headers[i] = p.Headers[i]
	}
	out = append(out, headers)
	for _, row := range rows {
		rec, err := e.marshalRecord(p, row)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, nil
}

func (e *Engine) marshalRecord(p *plan.Type, row reflect.Value) ([]interface{}, error) {
	row, ptr := addressableStruct(row)
	ret := make([]interface{}, len(p.Fields))
	for i, f := range p.Fields {
		fieldPtr := f.XField.Pointer(ptr)
		switch f.Kind {
		case plan.FieldScalar:
			val, err := e.scalarValue(fieldPtr, f.Type)
			if err != nil {
				return nil, err
			}
			ret[i] = val
		case plan.FieldStruct:
			childVals, err := e.childStructTable(fieldPtr, f)
			if err != nil {
				return nil, err
			}
			ret[i] = childVals
		case plan.FieldSliceStruct:
			childVals, err := e.childSliceTable(fieldPtr, f)
			if err != nil {
				return nil, err
			}
			ret[i] = childVals
		}
	}
	return ret, nil
}

func (e *Engine) scalarValue(fieldPtr unsafe.Pointer, rType reflect.Type) (interface{}, error) {
	if rType.Kind() == reflect.Ptr {
		p := *(*unsafe.Pointer)(fieldPtr)
		if p == nil {
			return nil, nil
		}
		return e.scalarValue(p, rType.Elem())
	}
	if rType == timeType {
		tm := *(*time.Time)(fieldPtr)
		if tm.IsZero() {
			return nil, nil
		}
		return tm.Format(e.timeLayout), nil
	}
	return reflect.NewAt(rType, fieldPtr).Elem().Interface(), nil
}

func (e *Engine) childStructTable(fieldPtr unsafe.Pointer, f *plan.Field) (interface{}, error) {
	vType := f.Type
	if vType.Kind() == reflect.Ptr {
		next := *(*unsafe.Pointer)(fieldPtr)
		if next == nil {
			return nil, nil
		}
		vType = vType.Elem()
		val := reflect.NewAt(vType, next).Elem()
		table, err := e.marshalTable(f.Child, []reflect.Value{val})
		if err != nil {
			return nil, err
		}
		return table, nil
	}
	val := reflect.NewAt(vType, fieldPtr).Elem()
	table, err := e.marshalTable(f.Child, []reflect.Value{val})
	if err != nil {
		return nil, err
	}
	return table, nil
}

func (e *Engine) childSliceTable(fieldPtr unsafe.Pointer, f *plan.Field) (interface{}, error) {
	rv := reflect.NewAt(f.Type, fieldPtr).Elem()
	if rv.IsNil() || rv.Len() == 0 {
		return nil, nil
	}
	rows := make([]reflect.Value, 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		rows = append(rows, deref(rv.Index(i)))
	}
	table, err := e.marshalTable(f.Child, rows)
	if err != nil {
		return nil, err
	}
	return table, nil
}

func deref(v reflect.Value) reflect.Value {
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return reflect.Value{}
		}
		v = v.Elem()
	}
	return v
}

func addressableStruct(rv reflect.Value) (reflect.Value, unsafe.Pointer) {
	if rv.CanAddr() {
		return rv, unsafe.Pointer(rv.Addr().Pointer())
	}
	tmp := reflect.New(rv.Type())
	tmp.Elem().Set(rv)
	return tmp.Elem(), unsafe.Pointer(tmp.Pointer())
}
