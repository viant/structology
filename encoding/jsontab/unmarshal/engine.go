package unmarshal

import (
	"bytes"
	"encoding/csv"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/viant/structology/encoding/jsontab/internal/plan"
	"github.com/viant/xunsafe"
)

var timeType = reflect.TypeOf(time.Time{})

type UnknownHeaderPolicy int

const (
	IgnoreUnknownHeader UnknownHeaderPolicy = iota
	ErrorOnUnknownHeader
)

type ArityPolicy int

const (
	AllowArityMismatch ArityPolicy = iota
	ErrorOnArityMismatch
)

type MalformedPolicy int

const (
	TolerantMalformed MalformedPolicy = iota
	ErrorOnMalformed
)

type Engine struct {
	tagName             string
	caseKey             string
	compileName         func(string) string
	timeLayout          string
	unknownHeaderPolicy UnknownHeaderPolicy
	arityPolicy         ArityPolicy
	malformedPolicy     MalformedPolicy
	bindCache           sync.Map // map[bindCacheKey][]boundColumn
}

func New(tagName, caseKey string, compileName func(string) string, timeLayout string, unknown UnknownHeaderPolicy, arity ArityPolicy, malformed MalformedPolicy) *Engine {
	if tagName == "" {
		tagName = "csvName"
	}
	if timeLayout == "" {
		timeLayout = time.RFC3339
	}
	return &Engine{
		tagName:             tagName,
		caseKey:             caseKey,
		compileName:         compileName,
		timeLayout:          timeLayout,
		unknownHeaderPolicy: unknown,
		arityPolicy:         arity,
		malformedPolicy:     malformed,
	}
}

type decodeError struct {
	Path string
	Row  int
	Col  int
	Err  error
}

func (e *decodeError) Error() string {
	parts := make([]string, 0, 4)
	if e.Path != "" {
		parts = append(parts, "path="+e.Path)
	}
	if e.Row >= 0 {
		parts = append(parts, fmt.Sprintf("row=%d", e.Row))
	}
	if e.Col >= 0 {
		parts = append(parts, fmt.Sprintf("col=%d", e.Col))
	}
	if e.Err != nil {
		parts = append(parts, e.Err.Error())
	}
	return strings.Join(parts, " ")
}

func (e *decodeError) Unwrap() error { return e.Err }

func (e *Engine) derr(path string, row, col int, err error) error {
	return &decodeError{Path: path, Row: row, Col: col, Err: err}
}

func (e *Engine) Unmarshal(data []byte, dest interface{}) error {
	if dest == nil {
		return fmt.Errorf("nil destination")
	}
	rt := reflect.TypeOf(dest)
	if rt.Kind() != reflect.Ptr {
		return fmt.Errorf("destination must be pointer")
	}

	rootVal := reflect.ValueOf(dest).Elem()
	target := rootVal.Type()
	if target.Kind() == reflect.Ptr {
		target = target.Elem()
	}

	rootIsSlice := false
	var elemType reflect.Type
	if target.Kind() == reflect.Slice {
		rootIsSlice = true
		elemType = target.Elem()
		for elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
	} else {
		elemType = target
	}
	if elemType.Kind() != reflect.Struct {
		return fmt.Errorf("unsupported destination type: %s", target)
	}
	p := plan.For(elemType, e.tagName, e.caseKey, e.compileName)

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		if e.malformedPolicy == TolerantMalformed {
			return nil
		}
		return fmt.Errorf("empty input")
	}

	if trimmed[0] != '[' {
		return e.decodeCSVInto(trimmed, p, rootVal, rootIsSlice, "")
	}

	var parsed interface{}
	if err := stdjson.Unmarshal(trimmed, &parsed); err != nil {
		return err
	}
	table, ok := parsed.([]interface{})
	if !ok {
		return fmt.Errorf("expected table array")
	}
	rows, err := e.decodeTable(p, table, "", 0)
	if err != nil {
		return err
	}
	return setRootRows(rootVal, rootIsSlice, rows)
}

func setRootRows(rootVal reflect.Value, rootIsSlice bool, rows []reflect.Value) error {
	if rootIsSlice {
		sliceType := rootVal.Type()
		out := reflect.MakeSlice(sliceType, 0, len(rows))
		for _, row := range rows {
			if sliceType.Elem().Kind() == reflect.Ptr {
				ptr := reflect.New(row.Type())
				ptr.Elem().Set(row)
				out = reflect.Append(out, ptr)
			} else {
				out = reflect.Append(out, row)
			}
		}
		rootVal.Set(out)
		return nil
	}
	if len(rows) == 0 {
		return nil
	}
	return setRootRow(rootVal, rows[0])
}

func setRootRow(rootVal reflect.Value, row reflect.Value) error {
	if rootVal.Kind() == reflect.Ptr {
		if rootVal.IsNil() {
			rootVal.Set(reflect.New(rootVal.Type().Elem()))
		}
		rootVal.Elem().Set(row)
		return nil
	}
	rootVal.Set(row)
	return nil
}

type boundColumn struct {
	field *plan.Field
	col   int
}

type bindCacheKey struct {
	rType      reflect.Type
	headersSig string
	unknown    UnknownHeaderPolicy
}

func (e *Engine) boundForHeaders(p *plan.Type, headers []string, path string) ([]boundColumn, error) {
	sig := strings.Join(headers, "\x1f")
	key := bindCacheKey{rType: p.Type, headersSig: sig, unknown: e.unknownHeaderPolicy}
	if v, ok := e.bindCache.Load(key); ok {
		return v.([]boundColumn), nil
	}
	bound := make([]boundColumn, 0, len(headers))
	for i, h := range headers {
		f, ok := p.HeaderToField[h]
		if !ok {
			if e.unknownHeaderPolicy == ErrorOnUnknownHeader {
				return nil, e.derr(path, 0, i, fmt.Errorf("unknown header %q", h))
			}
			continue
		}
		bound = append(bound, boundColumn{field: f, col: i})
	}
	e.bindCache.Store(key, bound)
	return bound, nil
}

func (e *Engine) decodeCSVInto(data []byte, p *plan.Type, rootVal reflect.Value, rootIsSlice bool, path string) error {
	if bytes.IndexByte(data, '"') < 0 {
		handled, err := e.decodeSimpleCSVInto(data, p, rootVal, rootIsSlice, path)
		if handled || err != nil {
			return err
		}
	}

	reader := csv.NewReader(bytes.NewReader(data))
	reader.TrimLeadingSpace = true
	reader.ReuseRecord = true

	headers, err := reader.Read()
	if err != nil {
		if err == io.EOF && e.malformedPolicy == TolerantMalformed {
			return nil
		}
		return e.derr(path, 0, -1, err)
	}
	bound, err := e.boundForHeaders(p, headers, path)
	if err != nil {
		return err
	}

	var out reflect.Value
	outLen := 0
	outCap := 0
	elemIsPtr := false
	if rootIsSlice {
		outCap = estimateCSVDataRows(data)
		if outCap < 1 {
			outCap = 1
		}
		elemIsPtr = rootVal.Type().Elem().Kind() == reflect.Ptr
		out = reflect.MakeSlice(rootVal.Type(), outCap, outCap)
	}
	rowNum := 1
	assignedOne := false
	for {
		record, readErr := reader.Read()
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			if e.malformedPolicy == TolerantMalformed {
				break
			}
			return e.derr(path, rowNum, -1, readErr)
		}
		if e.arityPolicy == ErrorOnArityMismatch && len(record) != len(headers) {
			return e.derr(path, rowNum, -1, fmt.Errorf("arity mismatch: got %d want %d", len(record), len(headers)))
		}

		var row reflect.Value
		var rowPtr unsafe.Pointer
		if rootIsSlice && !elemIsPtr && outLen < outCap {
			slot := out.Index(outLen)
			row = slot
			rowPtr = unsafe.Pointer(slot.Addr().Pointer())
		} else {
			row = reflect.New(p.Type).Elem()
			rowPtr = unsafe.Pointer(row.Addr().Pointer())
		}
		if p.Presence != nil {
			_ = plan.EnsurePresenceHolder(rowPtr, p.Presence)
		}
		if err = e.applyCSVRecord(p, rowPtr, record, bound, rowNum, path); err != nil {
			return err
		}

		if rootIsSlice {
			if elemIsPtr {
				ptr := reflect.New(row.Type())
				ptr.Elem().Set(row)
				if outLen < outCap {
					out.Index(outLen).Set(ptr)
				} else {
					out = reflect.Append(out, ptr)
				}
			}
			outLen++
		} else if !assignedOne {
			if err = setRootRow(rootVal, row); err != nil {
				return err
			}
			assignedOne = true
			break
		}
		rowNum++
	}
	if rootIsSlice {
		if outLen <= out.Len() {
			out = out.Slice(0, outLen)
		}
		rootVal.Set(out)
	}
	return nil
}

func (e *Engine) decodeSimpleCSVInto(data []byte, p *plan.Type, rootVal reflect.Value, rootIsSlice bool, path string) (bool, error) {
	if len(data) == 0 {
		return true, nil
	}

	line, pos := nextCSVLine(data, 0)
	if len(line) == 0 {
		if e.malformedPolicy == TolerantMalformed {
			return true, nil
		}
		return true, e.derr(path, 0, -1, fmt.Errorf("empty input"))
	}
	headers := splitSimpleCSV(line)
	bound, err := e.boundForHeaders(p, headers, path)
	if err != nil {
		return true, err
	}

	var out reflect.Value
	outLen := 0
	outCap := 0
	elemIsPtr := false
	if rootIsSlice {
		outCap = estimateCSVDataRows(data)
		if outCap < 1 {
			outCap = 1
		}
		elemIsPtr = rootVal.Type().Elem().Kind() == reflect.Ptr
		out = reflect.MakeSlice(rootVal.Type(), outCap, outCap)
	}

	rowNum := 1
	assignedOne := false
	for pos < len(data) {
		line, pos = nextCSVLine(data, pos)
		if len(line) == 0 {
			continue
		}
		if e.arityPolicy == ErrorOnArityMismatch {
			fields := 1
			for i := 0; i < len(line); i++ {
				if line[i] == ',' {
					fields++
				}
			}
			if fields != len(headers) {
				return true, e.derr(path, rowNum, -1, fmt.Errorf("arity mismatch: got %d want %d", fields, len(headers)))
			}
		}

		var row reflect.Value
		var rowPtr unsafe.Pointer
		if rootIsSlice && !elemIsPtr && outLen < outCap {
			slot := out.Index(outLen)
			row = slot
			rowPtr = unsafe.Pointer(slot.Addr().Pointer())
		} else {
			row = reflect.New(p.Type).Elem()
			rowPtr = unsafe.Pointer(row.Addr().Pointer())
		}
		if p.Presence != nil {
			_ = plan.EnsurePresenceHolder(rowPtr, p.Presence)
		}
		if err = e.applySimpleCSVRecord(p, rowPtr, line, bound, rowNum, path); err != nil {
			return true, err
		}

		if rootIsSlice {
			if elemIsPtr {
				ptr := reflect.New(row.Type())
				ptr.Elem().Set(row)
				if outLen < outCap {
					out.Index(outLen).Set(ptr)
				} else {
					out = reflect.Append(out, ptr)
				}
			}
			outLen++
		} else if !assignedOne {
			if err = setRootRow(rootVal, row); err != nil {
				return true, err
			}
			assignedOne = true
			break
		}
		rowNum++
	}

	if rootIsSlice {
		if outLen <= out.Len() {
			out = out.Slice(0, outLen)
		}
		rootVal.Set(out)
	}
	return true, nil
}

func (e *Engine) applySimpleCSVRecord(p *plan.Type, rowPtr unsafe.Pointer, line []byte, bound []boundColumn, rowNum int, path string) error {
	if len(bound) == 0 {
		return nil
	}
	bi := 0
	col := 0
	start := 0
	for i := 0; i <= len(line); i++ {
		if i != len(line) && line[i] != ',' {
			continue
		}
		if bi < len(bound) && bound[bi].col == col {
			field := bound[bi].field
			fieldPtr := field.XField.Pointer(rowPtr)
			raw := line[start:i]
			switch field.Kind {
			case plan.FieldScalar:
				if err := e.assignScalarBytes(fieldPtr, field.Type, raw); err != nil {
					return e.derr(joinPath(path, field.StructName), rowNum, col, err)
				}
				e.markPresence(p, rowPtr, field.StructName)
			case plan.FieldStruct, plan.FieldSliceStruct:
				if len(raw) == 0 || isNullBytes(raw) {
					bi++
					col++
					start = i + 1
					continue
				}
				if raw[0] != '[' {
					if e.malformedPolicy == TolerantMalformed {
						bi++
						col++
						start = i + 1
						continue
					}
					return e.derr(joinPath(path, field.StructName), rowNum, col, fmt.Errorf("expects nested table"))
				}
				var parsed interface{}
				if parseErr := stdjson.Unmarshal(raw, &parsed); parseErr != nil {
					if e.malformedPolicy == TolerantMalformed {
						bi++
						col++
						start = i + 1
						continue
					}
					return e.derr(joinPath(path, field.StructName), rowNum, col, parseErr)
				}
				childTable, ok := parsed.([]interface{})
				if !ok {
					if e.malformedPolicy == TolerantMalformed {
						bi++
						col++
						start = i + 1
						continue
					}
					return e.derr(joinPath(path, field.StructName), rowNum, col, fmt.Errorf("expects nested table"))
				}
				fieldPath := joinPath(path, field.StructName)
				if field.Kind == plan.FieldStruct {
					childRows, decErr := e.decodeTable(field.Child, childTable, fieldPath, rowNum)
					if decErr != nil {
						return decErr
					}
					if len(childRows) == 0 {
						bi++
						col++
						start = i + 1
						continue
					}
					if field.Type.Kind() == reflect.Ptr {
						target := xunsafe.SafeDerefPointer(fieldPtr, field.Type)
						reflect.NewAt(field.Type.Elem(), target).Elem().Set(childRows[0])
					} else {
						reflect.NewAt(field.Type, fieldPtr).Elem().Set(childRows[0])
					}
				} else {
					childRows, decErr := e.decodeTable(field.Child, childTable, fieldPath, rowNum)
					if decErr != nil {
						return decErr
					}
					sliceVal := reflect.MakeSlice(field.Type, 0, len(childRows))
					for _, child := range childRows {
						if field.Type.Elem().Kind() == reflect.Ptr {
							item := reflect.New(child.Type())
							item.Elem().Set(child)
							sliceVal = reflect.Append(sliceVal, item)
						} else {
							sliceVal = reflect.Append(sliceVal, child)
						}
					}
					reflect.NewAt(field.Type, fieldPtr).Elem().Set(sliceVal)
				}
				e.markPresence(p, rowPtr, field.StructName)
			}
			bi++
		}
		col++
		start = i + 1
	}
	return nil
}

func (e *Engine) assignScalarBytes(ptr unsafe.Pointer, rType reflect.Type, raw []byte) error {
	if rType.Kind() == reflect.Ptr {
		if len(raw) == 0 || isNullBytes(raw) {
			return nil
		}
		target := xunsafe.SafeDerefPointer(ptr, rType)
		return e.assignScalarBytes(target, rType.Elem(), raw)
	}
	if len(raw) == 0 || isNullBytes(raw) {
		return nil
	}

	switch rType.Kind() {
	case reflect.String:
		*xunsafe.AsStringPtr(ptr) = string(raw)
		return nil
	case reflect.Bool:
		b, ok := parseBoolBytes(raw)
		if !ok {
			parsed, err := strconv.ParseBool(string(raw))
			if err != nil {
				return fmt.Errorf("expected bool")
			}
			b = parsed
		}
		*xunsafe.AsBoolPtr(ptr) = b
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, ok := parseInt64Bytes(raw)
		if !ok {
			parsed, err := strconv.ParseInt(string(raw), 10, 64)
			if err != nil {
				return fmt.Errorf("expected number")
			}
			i = parsed
		}
		return setIntKind(ptr, rType.Kind(), i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, ok := parseUint64Bytes(raw)
		if !ok {
			parsed, err := strconv.ParseUint(string(raw), 10, 64)
			if err != nil {
				return fmt.Errorf("expected number")
			}
			u = parsed
		}
		return setUintKind(ptr, rType.Kind(), u)
	case reflect.Float32, reflect.Float64:
		if i, ok := parseInt64Bytes(raw); ok {
			return setFloatKind(ptr, rType.Kind(), float64(i))
		}
		if u, ok := parseUint64Bytes(raw); ok {
			return setFloatKind(ptr, rType.Kind(), float64(u))
		}
		f, err := strconv.ParseFloat(string(raw), 64)
		if err != nil {
			return fmt.Errorf("expected number")
		}
		return setFloatKind(ptr, rType.Kind(), f)
	case reflect.Struct:
		if rType == timeType {
			tm, err := time.Parse(e.timeLayout, string(raw))
			if err != nil {
				return err
			}
			*xunsafe.AsTimePtr(ptr) = tm
			return nil
		}
	}
	return e.assignScalar(ptr, rType, string(raw))
}

func isNullBytes(raw []byte) bool {
	return len(raw) == 4 && raw[0] == 'n' && raw[1] == 'u' && raw[2] == 'l' && raw[3] == 'l'
}

func parseBoolBytes(raw []byte) (bool, bool) {
	switch len(raw) {
	case 1:
		if raw[0] == '1' {
			return true, true
		}
		if raw[0] == '0' {
			return false, true
		}
	case 4:
		if raw[0] == 't' && raw[1] == 'r' && raw[2] == 'u' && raw[3] == 'e' {
			return true, true
		}
	case 5:
		if raw[0] == 'f' && raw[1] == 'a' && raw[2] == 'l' && raw[3] == 's' && raw[4] == 'e' {
			return false, true
		}
	}
	return false, false
}

func parseInt64Bytes(raw []byte) (int64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	sign := int64(1)
	idx := 0
	if raw[0] == '-' {
		sign = -1
		idx = 1
		if idx == len(raw) {
			return 0, false
		}
	}
	var n int64
	for ; idx < len(raw); idx++ {
		c := raw[idx]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return sign * n, true
}

func parseUint64Bytes(raw []byte) (uint64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		d := uint64(c - '0')
		if n > (math.MaxUint64-d)/10 {
			return 0, false
		}
		n = n*10 + d
	}
	return n, true
}

func splitSimpleCSV(line []byte) []string {
	if len(line) == 0 {
		return nil
	}
	fields := 1
	for i := 0; i < len(line); i++ {
		if line[i] == ',' {
			fields++
		}
	}
	out := make([]string, 0, fields)
	start := 0
	for i := 0; i <= len(line); i++ {
		if i != len(line) && line[i] != ',' {
			continue
		}
		out = append(out, string(line[start:i]))
		start = i + 1
	}
	return out
}

func nextCSVLine(data []byte, pos int) ([]byte, int) {
	if pos >= len(data) {
		return nil, len(data)
	}
	start := pos
	for pos < len(data) && data[pos] != '\n' {
		pos++
	}
	line := data[start:pos]
	if len(line) > 0 && line[len(line)-1] == '\r' {
		line = line[:len(line)-1]
	}
	if pos < len(data) && data[pos] == '\n' {
		pos++
	}
	return line, pos
}

func estimateCSVDataRows(data []byte) int {
	if len(data) == 0 {
		return 0
	}
	lines := bytes.Count(data, []byte{'\n'})
	if data[len(data)-1] != '\n' {
		lines++
	}
	if lines <= 1 {
		return 0
	}
	return lines - 1
}

func (e *Engine) applyCSVRecord(p *plan.Type, rowPtr unsafe.Pointer, record []string, bound []boundColumn, rowNum int, path string) error {
	for _, b := range bound {
		if b.col >= len(record) {
			continue
		}
		field := b.field
		raw := record[b.col]
		fieldPtr := field.XField.Pointer(rowPtr)
		switch field.Kind {
		case plan.FieldScalar:
			if err := e.assignScalarString(fieldPtr, field.Type, raw); err != nil {
				return e.derr(joinPath(path, field.StructName), rowNum, b.col, err)
			}
			e.markPresence(p, rowPtr, field.StructName)
		case plan.FieldStruct, plan.FieldSliceStruct:
			if raw == "" || raw == "null" {
				continue
			}
			if raw[0] != '[' {
				if e.malformedPolicy == TolerantMalformed {
					continue
				}
				return e.derr(joinPath(path, field.StructName), rowNum, b.col, fmt.Errorf("expects nested table"))
			}
			var parsed interface{}
			if parseErr := stdjson.Unmarshal([]byte(raw), &parsed); parseErr != nil {
				if e.malformedPolicy == TolerantMalformed {
					continue
				}
				return e.derr(joinPath(path, field.StructName), rowNum, b.col, parseErr)
			}
			childTable, ok := parsed.([]interface{})
			if !ok {
				if e.malformedPolicy == TolerantMalformed {
					continue
				}
				return e.derr(joinPath(path, field.StructName), rowNum, b.col, fmt.Errorf("expects nested table"))
			}
			fieldPath := joinPath(path, field.StructName)
			if field.Kind == plan.FieldStruct {
				childRows, decErr := e.decodeTable(field.Child, childTable, fieldPath, rowNum)
				if decErr != nil {
					return decErr
				}
				if len(childRows) == 0 {
					continue
				}
				if field.Type.Kind() == reflect.Ptr {
					target := xunsafe.SafeDerefPointer(fieldPtr, field.Type)
					reflect.NewAt(field.Type.Elem(), target).Elem().Set(childRows[0])
				} else {
					reflect.NewAt(field.Type, fieldPtr).Elem().Set(childRows[0])
				}
			} else {
				childRows, decErr := e.decodeTable(field.Child, childTable, fieldPath, rowNum)
				if decErr != nil {
					return decErr
				}
				sliceVal := reflect.MakeSlice(field.Type, 0, len(childRows))
				for _, child := range childRows {
					if field.Type.Elem().Kind() == reflect.Ptr {
						item := reflect.New(child.Type())
						item.Elem().Set(child)
						sliceVal = reflect.Append(sliceVal, item)
					} else {
						sliceVal = reflect.Append(sliceVal, child)
					}
				}
				reflect.NewAt(field.Type, fieldPtr).Elem().Set(sliceVal)
			}
			e.markPresence(p, rowPtr, field.StructName)
		}
	}
	return nil
}

func (e *Engine) decodeTable(p *plan.Type, table []interface{}, path string, baseRow int) ([]reflect.Value, error) {
	if len(table) == 0 {
		return nil, nil
	}
	headers, err := toStringSlice(table[0])
	if err != nil {
		if e.malformedPolicy == TolerantMalformed {
			return nil, nil
		}
		return nil, e.derr(path, baseRow, 0, fmt.Errorf("invalid header row: %w", err))
	}
	indexByHeader := make(map[string]int, len(headers))
	for i, h := range headers {
		indexByHeader[h] = i
	}
	if e.unknownHeaderPolicy == ErrorOnUnknownHeader {
		for i, h := range headers {
			if _, ok := p.HeaderToField[h]; !ok {
				return nil, e.derr(path, baseRow, i, fmt.Errorf("unknown header %q", h))
			}
		}
	}

	ret := make([]reflect.Value, 0, len(table)-1)
	for i := 1; i < len(table); i++ {
		record, ok := table[i].([]interface{})
		if !ok {
			if e.malformedPolicy == TolerantMalformed {
				continue
			}
			return nil, e.derr(path, baseRow+i, -1, fmt.Errorf("record is not an array"))
		}
		if e.arityPolicy == ErrorOnArityMismatch && len(record) != len(headers) {
			return nil, e.derr(path, baseRow+i, -1, fmt.Errorf("arity mismatch: got %d want %d", len(record), len(headers)))
		}

		row := reflect.New(p.Type).Elem()
		rowPtr := unsafe.Pointer(row.Addr().Pointer())
		if p.Presence != nil {
			_ = plan.EnsurePresenceHolder(rowPtr, p.Presence)
		}

		for _, field := range p.Fields {
			idx, ok := indexByHeader[field.HeaderName]
			if !ok || idx >= len(record) {
				continue
			}
			val := record[idx]
			fieldPtr := field.XField.Pointer(rowPtr)
			switch field.Kind {
			case plan.FieldScalar:
				if err = e.assignScalar(fieldPtr, field.Type, val); err != nil {
					return nil, e.derr(joinPath(path, field.StructName), baseRow+i, idx, err)
				}
				e.markPresence(p, rowPtr, field.StructName)
			case plan.FieldStruct:
				if val == nil {
					continue
				}
				childTable, ok := val.([]interface{})
				if !ok {
					if e.malformedPolicy == TolerantMalformed {
						continue
					}
					return nil, e.derr(joinPath(path, field.StructName), baseRow+i, idx, fmt.Errorf("expects nested table"))
				}
				fieldPath := joinPath(path, field.StructName)
				rows, decErr := e.decodeTable(field.Child, childTable, fieldPath, baseRow+i)
				if decErr != nil {
					return nil, decErr
				}
				if len(rows) == 0 {
					continue
				}
				if field.Type.Kind() == reflect.Ptr {
					target := xunsafe.SafeDerefPointer(fieldPtr, field.Type)
					reflect.NewAt(field.Type.Elem(), target).Elem().Set(rows[0])
				} else {
					reflect.NewAt(field.Type, fieldPtr).Elem().Set(rows[0])
				}
				e.markPresence(p, rowPtr, field.StructName)
			case plan.FieldSliceStruct:
				if val == nil {
					continue
				}
				childTable, ok := val.([]interface{})
				if !ok {
					if e.malformedPolicy == TolerantMalformed {
						continue
					}
					return nil, e.derr(joinPath(path, field.StructName), baseRow+i, idx, fmt.Errorf("expects nested table"))
				}
				fieldPath := joinPath(path, field.StructName)
				rows, decErr := e.decodeTable(field.Child, childTable, fieldPath, baseRow+i)
				if decErr != nil {
					return nil, decErr
				}
				sliceVal := reflect.MakeSlice(field.Type, 0, len(rows))
				for _, child := range rows {
					if field.Type.Elem().Kind() == reflect.Ptr {
						item := reflect.New(child.Type())
						item.Elem().Set(child)
						sliceVal = reflect.Append(sliceVal, item)
					} else {
						sliceVal = reflect.Append(sliceVal, child)
					}
				}
				reflect.NewAt(field.Type, fieldPtr).Elem().Set(sliceVal)
				e.markPresence(p, rowPtr, field.StructName)
			}
		}
		ret = append(ret, row)
	}
	return ret, nil
}

func (e *Engine) markPresence(p *plan.Type, rowPtr unsafe.Pointer, fieldName string) {
	if p == nil || p.Presence == nil {
		return
	}
	flag := p.Presence.Flags[fieldName]
	if flag == nil {
		return
	}
	holder := plan.EnsurePresenceHolder(rowPtr, p.Presence)
	if holder == nil {
		return
	}
	flag.SetBool(holder, true)
}

func joinPath(path, field string) string {
	if path == "" {
		return field
	}
	return path + "." + field
}

func (e *Engine) assignScalarString(ptr unsafe.Pointer, rType reflect.Type, raw string) error {
	if rType.Kind() == reflect.Ptr {
		if raw == "" || raw == "null" {
			return nil
		}
		target := xunsafe.SafeDerefPointer(ptr, rType)
		return e.assignScalarString(target, rType.Elem(), raw)
	}
	if raw == "" || raw == "null" {
		return nil
	}

	switch rType.Kind() {
	case reflect.String:
		*xunsafe.AsStringPtr(ptr) = raw
		return nil
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("expected bool")
		}
		*xunsafe.AsBoolPtr(ptr) = b
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("expected number")
		}
		return setIntKind(ptr, rType.Kind(), i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return fmt.Errorf("expected number")
		}
		return setUintKind(ptr, rType.Kind(), u)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return fmt.Errorf("expected number")
		}
		return setFloatKind(ptr, rType.Kind(), f)
	case reflect.Struct:
		if rType == timeType {
			tm, err := time.Parse(e.timeLayout, raw)
			if err != nil {
				return err
			}
			*xunsafe.AsTimePtr(ptr) = tm
			return nil
		}
	}
	return e.assignScalar(ptr, rType, raw)
}

func (e *Engine) assignScalar(ptr unsafe.Pointer, rType reflect.Type, value interface{}) error {
	if rType.Kind() == reflect.Ptr {
		if value == nil {
			return nil
		}
		target := xunsafe.SafeDerefPointer(ptr, rType)
		return e.assignScalar(target, rType.Elem(), value)
	}
	if value == nil {
		return nil
	}

	switch rType.Kind() {
	case reflect.String:
		s, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string")
		}
		*xunsafe.AsStringPtr(ptr) = s
		return nil
	case reflect.Bool:
		switch a := value.(type) {
		case bool:
			*xunsafe.AsBoolPtr(ptr) = a
			return nil
		case string:
			b, err := strconv.ParseBool(a)
			if err != nil {
				return fmt.Errorf("expected bool")
			}
			*xunsafe.AsBoolPtr(ptr) = b
			return nil
		default:
			return fmt.Errorf("expected bool")
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := asInt64(value)
		if err != nil {
			return err
		}
		return setIntKind(ptr, rType.Kind(), i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		u, err := asUint64(value)
		if err != nil {
			return err
		}
		return setUintKind(ptr, rType.Kind(), u)
	case reflect.Float32, reflect.Float64:
		f, err := asFloat64(value)
		if err != nil {
			return err
		}
		return setFloatKind(ptr, rType.Kind(), f)
	case reflect.Struct:
		if rType == timeType {
			s, ok := value.(string)
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
	}

	b, err := stdjson.Marshal(value)
	if err != nil {
		return err
	}
	tmp := reflect.New(rType)
	if err = stdjson.Unmarshal(b, tmp.Interface()); err != nil {
		return err
	}
	reflect.NewAt(rType, ptr).Elem().Set(tmp.Elem())
	return nil
}

func toStringSlice(v interface{}) ([]string, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected []interface{}")
	}
	ret := make([]string, 0, len(arr))
	for _, item := range arr {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("expected string header")
		}
		ret = append(ret, s)
	}
	return ret, nil
}

func asInt64(v interface{}) (int64, error) {
	switch a := v.(type) {
	case float64:
		return int64(a), nil
	case string:
		return strconv.ParseInt(a, 10, 64)
	case int64:
		return a, nil
	case int:
		return int64(a), nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func asUint64(v interface{}) (uint64, error) {
	switch a := v.(type) {
	case float64:
		return uint64(a), nil
	case string:
		return strconv.ParseUint(a, 10, 64)
	case uint64:
		return a, nil
	case int:
		return uint64(a), nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func asFloat64(v interface{}) (float64, error) {
	switch a := v.(type) {
	case float64:
		return a, nil
	case string:
		return strconv.ParseFloat(a, 64)
	case int:
		return float64(a), nil
	default:
		return 0, fmt.Errorf("expected number")
	}
}

func setIntKind(ptr unsafe.Pointer, kind reflect.Kind, value int64) error {
	switch kind {
	case reflect.Int:
		if strconv.IntSize == 32 && (value < math.MinInt32 || value > math.MaxInt32) {
			return fmt.Errorf("integer overflow")
		}
		*(*int)(ptr) = int(value)
	case reflect.Int8:
		if value < math.MinInt8 || value > math.MaxInt8 {
			return fmt.Errorf("integer overflow")
		}
		*(*int8)(ptr) = int8(value)
	case reflect.Int16:
		if value < math.MinInt16 || value > math.MaxInt16 {
			return fmt.Errorf("integer overflow")
		}
		*(*int16)(ptr) = int16(value)
	case reflect.Int32:
		if value < math.MinInt32 || value > math.MaxInt32 {
			return fmt.Errorf("integer overflow")
		}
		*(*int32)(ptr) = int32(value)
	case reflect.Int64:
		*(*int64)(ptr) = value
	default:
		return fmt.Errorf("expected integer kind")
	}
	return nil
}

func setUintKind(ptr unsafe.Pointer, kind reflect.Kind, value uint64) error {
	switch kind {
	case reflect.Uint:
		if strconv.IntSize == 32 && value > math.MaxUint32 {
			return fmt.Errorf("unsigned overflow")
		}
		*(*uint)(ptr) = uint(value)
	case reflect.Uint8:
		if value > math.MaxUint8 {
			return fmt.Errorf("unsigned overflow")
		}
		*(*uint8)(ptr) = uint8(value)
	case reflect.Uint16:
		if value > math.MaxUint16 {
			return fmt.Errorf("unsigned overflow")
		}
		*(*uint16)(ptr) = uint16(value)
	case reflect.Uint32:
		if value > math.MaxUint32 {
			return fmt.Errorf("unsigned overflow")
		}
		*(*uint32)(ptr) = uint32(value)
	case reflect.Uint64:
		*(*uint64)(ptr) = value
	default:
		return fmt.Errorf("expected unsigned kind")
	}
	return nil
}

func setFloatKind(ptr unsafe.Pointer, kind reflect.Kind, value float64) error {
	switch kind {
	case reflect.Float32:
		*(*float32)(ptr) = float32(value)
	case reflect.Float64:
		*(*float64)(ptr) = value
	default:
		return fmt.Errorf("expected float kind")
	}
	return nil
}
