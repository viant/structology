package marshal

import (
	"encoding"
	stdjson "encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"
	"unsafe"

	"github.com/viant/structology/encoding/json/internal/tagutil"
	"github.com/viant/xunsafe"
)

// Engine marshals values with transform/exclusion hooks.
type Engine struct {
	NameTransform func(path []string, field string) string
	Exclude       func(path []string, field string) bool

	hasTransform bool
	hasExclude   bool
	omitEmpty    bool
	nilSliceNull bool
	timeLayout   string
	caseKey      string
	compileName  func(string) string

	planMu       sync.RWMutex
	staticPlans  map[reflect.Type]*structPlan
	dynamicPlans map[reflect.Type]*structPlan

	customMu    sync.RWMutex
	customTypes map[reflect.Type]bool
}

type structPlan struct {
	fields    []fieldPlan
	fastOnly  bool
	fastOps   []fastFieldOp
	staticOps []staticFieldOp
	inlineIdx int
}

type fieldPlan struct {
	fieldName  string
	name       string
	keyLit     []byte
	omitempty  bool
	nullable   bool
	anonymous  bool
	inline     bool
	ignore     bool
	index      int
	kind       reflect.Kind
	ptrElem    reflect.Kind
	rType      reflect.Type
	xField     *xunsafe.Field
	explicit   bool
	inlineRaw  bool
	timeLayout string
	fast       bool
	appendFn   func(*[]byte, unsafe.Pointer) error
	emptyFn    func(unsafe.Pointer) bool
}

type fastFieldOp struct {
	xField  *xunsafe.Field
	keyLit  []byte
	omit    bool
	emptyFn func(unsafe.Pointer) bool
	kind    reflect.Kind
	ptrElem reflect.Kind
}

type staticFieldOp func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error

type pathStack struct {
	segments []string
	cached   []string
	depth    int
}

type encoderSession struct {
	buf  []byte
	path pathStack
}

var (
	sessionPool = sync.Pool{New: func() interface{} { return &encoderSession{buf: make([]byte, 0, 256)} }}
	timeType    = reflect.TypeOf(time.Time{})
	rawMsgType  = reflect.TypeOf(stdjson.RawMessage{})

	jsonMarshalerType = reflect.TypeOf((*stdjson.Marshaler)(nil)).Elem()
	textMarshalerType = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
)

func New(nameTransform func(path []string, field string) string, exclude func(path []string, field string) bool, omitEmpty bool, nilSliceNull bool, timeLayout string, caseKey string, compileName func(string) string) *Engine {
	if timeLayout == "" {
		timeLayout = time.RFC3339
	}
	return &Engine{
		NameTransform: nameTransform,
		Exclude:       exclude,
		hasTransform:  nameTransform != nil,
		hasExclude:    exclude != nil,
		omitEmpty:     omitEmpty,
		nilSliceNull:  nilSliceNull,
		timeLayout:    timeLayout,
		caseKey:       caseKey,
		compileName:   compileName,
		staticPlans:   map[reflect.Type]*structPlan{},
		dynamicPlans:  map[reflect.Type]*structPlan{},
		customTypes:   map[reflect.Type]bool{},
	}
}

func (e *Engine) Marshal(value interface{}) ([]byte, error) {
	sess := acquireSession()
	defer releaseSession(sess)
	if value == nil {
		sess.buf = append(sess.buf, "null"...)
		out := append([]byte(nil), sess.buf...)
		return out, nil
	}
	rv := reflect.ValueOf(value)
	if err := e.appendValue(sess, rv); err != nil {
		return nil, err
	}
	out := append([]byte(nil), sess.buf...)
	return out, nil
}

// MarshalTo appends marshaled JSON to dst and returns the resulting slice.
func (e *Engine) MarshalTo(dst []byte, value interface{}) ([]byte, error) {
	dst = ensureSpare(dst, 128)
	sess := encoderSession{buf: dst}
	if value == nil {
		sess.buf = append(sess.buf, "null"...)
		return sess.buf, nil
	}
	rv := reflect.ValueOf(value)
	if err := e.appendValue(&sess, rv); err != nil {
		return nil, err
	}
	return sess.buf, nil
}

// MarshalPtr marshals pointer roots on the fast pointer path.
func (e *Engine) MarshalPtr(value interface{}) ([]byte, error) {
	rt := reflect.TypeOf(value)
	if rt == nil || rt.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("MarshalPtr expects pointer, got %v", rt)
	}
	out, err := e.marshalPtrTyped(nil, rt.Elem(), xunsafe.AsPointer(value))
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), out...), nil
}

// MarshalPtrTo appends pointer-root marshaled JSON to dst.
func (e *Engine) MarshalPtrTo(dst []byte, value interface{}) ([]byte, error) {
	rt := reflect.TypeOf(value)
	if rt == nil || rt.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("MarshalPtrTo expects pointer, got %v", rt)
	}
	return e.marshalPtrTyped(dst, rt.Elem(), xunsafe.AsPointer(value))
}

// MarshalTypedPtr appends a typed pointer-root value to dst.
// This is intended for internal fast paths where caller already has root type and pointer.
func (e *Engine) MarshalTypedPtr(dst []byte, elemType reflect.Type, ptr unsafe.Pointer) ([]byte, error) {
	return e.marshalPtrTyped(dst, elemType, ptr)
}

func (e *Engine) marshalPtrTyped(dst []byte, elemType reflect.Type, ptr unsafe.Pointer) ([]byte, error) {
	dst = ensureSpare(dst, 128)
	if ptr == nil {
		return append(dst, "null"...), nil
	}
	if e.hasCustomMarshalerType(elemType) {
		sess := encoderSession{buf: dst}
		root := reflect.NewAt(elemType, ptr).Elem()
		if handled, err := e.tryAppendCustomMarshaler(&sess, root); handled || err != nil {
			if err != nil {
				return nil, err
			}
			return sess.buf, nil
		}
	}
	if !e.hasTransform && !e.hasExclude {
		if elemType.Kind() == reflect.Struct {
			plan := e.getStaticPlan(elemType)
			if plan.fastOnly {
				return e.appendStructFastOnly(dst, plan, ptr)
			}
			sess := encoderSession{buf: dst}
			if err := e.appendStructStaticPtr(&sess, elemType, ptr); err != nil {
				return nil, err
			}
			return sess.buf, nil
		}
	}
	sess := encoderSession{buf: dst}
	root := reflect.NewAt(elemType, ptr).Elem()
	if err := e.appendValue(&sess, root); err != nil {
		return nil, err
	}
	return sess.buf, nil
}

func (e *Engine) appendStructStaticPtr(sess *encoderSession, rt reflect.Type, structPtr unsafe.Pointer) error {
	if e.hasCustomMarshalerType(rt) {
		if handled, err := e.tryAppendCustomMarshaler(sess, reflect.NewAt(rt, structPtr).Elem()); handled || err != nil {
			return err
		}
	}
	plan := e.getStaticPlan(rt)
	if plan.inlineIdx >= 0 {
		p := &plan.fields[plan.inlineIdx]
		if p.inlineRaw {
			raw := inlineRaw(p.xField.Pointer(structPtr), p.kind)
			if len(raw) == 0 {
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			sess.buf = append(sess.buf, raw...)
			return nil
		}
		inlineVal := reflect.NewAt(p.rType, p.xField.Pointer(structPtr)).Elem()
		for inlineVal.Kind() == reflect.Interface || inlineVal.Kind() == reflect.Ptr {
			if inlineVal.IsNil() {
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			inlineVal = inlineVal.Elem()
		}
		return e.appendValue(sess, inlineVal)
	}
	if handled, err := e.tryInlineRawMessage(sess, plan, structPtr); handled || err != nil {
		return err
	}
	sess.buf = append(sess.buf, '{')
	fieldCounter := 0
	for i := range plan.staticOps {
		if err := plan.staticOps[i](e, sess, structPtr, &fieldCounter); err != nil {
			return err
		}
	}
	sess.buf = append(sess.buf, '}')
	return nil
}

func (e *Engine) appendStructFastOnly(dst []byte, plan *structPlan, structPtr unsafe.Pointer) ([]byte, error) {
	dst = append(dst, '{')
	fieldCounter := 0
	for i := range plan.fastOps {
		op := &plan.fastOps[i]
		fieldPtr := op.xField.Pointer(structPtr)
		if (op.omit || e.omitEmpty) && op.emptyFn(fieldPtr) {
			continue
		}
		if fieldCounter > 0 {
			dst = append(dst, ',')
		}
		fieldCounter++
		dst = append(dst, op.keyLit...)
		var err error
		dst, err = appendPrimitiveFast(dst, fieldPtr, op.kind, op.ptrElem)
		if err != nil {
			return nil, err
		}
	}
	dst = append(dst, '}')
	return dst, nil
}

func ensureSpare(dst []byte, minSpare int) []byte {
	if cap(dst)-len(dst) >= minSpare {
		return dst
	}
	if cap(dst) > 0 {
		return dst
	}
	grown := make([]byte, len(dst), len(dst)+minSpare)
	copy(grown, dst)
	return grown
}

func acquireSession() *encoderSession {
	s := sessionPool.Get().(*encoderSession)
	s.buf = s.buf[:0]
	s.path.reset()
	return s
}

func releaseSession(s *encoderSession) {
	const maxPooledCap = 64 << 10
	if cap(s.buf) > maxPooledCap {
		s.buf = make([]byte, 0, 256)
	}
	s.buf = s.buf[:0]
	s.path.reset()
	sessionPool.Put(s)
}

func (p *pathStack) reset() {
	p.depth = 0
}

func (p *pathStack) push(seg string) {
	if p.depth < len(p.segments) {
		p.segments[p.depth] = seg
		p.cached[p.depth] = ""
	} else {
		p.segments = append(p.segments, seg)
		p.cached = append(p.cached, "")
	}
	p.depth++
}

func (p *pathStack) pop() {
	if p.depth > 0 {
		p.depth--
	}
}

func (e *Engine) currentPath(sess *encoderSession) []string {
	if !e.hasTransform && !e.hasExclude {
		return nil
	}
	if sess.path.depth == 0 {
		return nil
	}
	return sess.path.segments[:sess.path.depth]
}

func (e *Engine) appendValue(sess *encoderSession, rv reflect.Value) error {
	if !rv.IsValid() {
		sess.buf = append(sess.buf, "null"...)
		return nil
	}
	if e.hasCustomMarshalerType(rv.Type()) {
		if handled, err := e.tryAppendCustomMarshaler(sess, rv); handled || err != nil {
			return err
		}
	}
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			sess.buf = append(sess.buf, "null"...)
			return nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		if rv.Type() == timeType {
			sess.buf = strconv.AppendQuote(sess.buf, rv.Interface().(time.Time).Format(e.timeLayout))
			return nil
		}
		if !e.hasTransform && !e.hasExclude {
			return e.appendStructStatic(sess, rv)
		}
		return e.appendStructDynamic(sess, rv)
	case reflect.Slice, reflect.Array:
		if rv.Kind() == reflect.Slice && rv.IsNil() {
			if e.nilSliceNull {
				sess.buf = append(sess.buf, "null"...)
			} else {
				sess.buf = append(sess.buf, '[', ']')
			}
			return nil
		}
		sess.buf = append(sess.buf, '[')
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				sess.buf = append(sess.buf, ',')
			}
			if err := e.appendValue(sess, rv.Index(i)); err != nil {
				return err
			}
		}
		sess.buf = append(sess.buf, ']')
		return nil
	case reflect.Map:
		if rv.IsNil() {
			sess.buf = append(sess.buf, "null"...)
			return nil
		}
		if !e.hasTransform && !e.hasExclude {
			return e.appendMapStatic(sess, rv)
		}
		return e.appendMapDynamic(sess, rv)
	case reflect.String:
		sess.buf = strconv.AppendQuote(sess.buf, rv.String())
		return nil
	case reflect.Bool:
		if rv.Bool() {
			sess.buf = append(sess.buf, "true"...)
		} else {
			sess.buf = append(sess.buf, "false"...)
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		sess.buf = strconv.AppendInt(sess.buf, rv.Int(), 10)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		sess.buf = strconv.AppendUint(sess.buf, rv.Uint(), 10)
		return nil
	case reflect.Float32:
		sess.buf = strconv.AppendFloat(sess.buf, rv.Float(), 'g', -1, 32)
		return nil
	case reflect.Float64:
		sess.buf = strconv.AppendFloat(sess.buf, rv.Float(), 'g', -1, 64)
		return nil
	default:
		return fmt.Errorf("unsupported marshal kind: %s", rv.Kind())
	}
}

func (e *Engine) tryAppendCustomMarshaler(sess *encoderSession, rv reflect.Value) (bool, error) {
	if !rv.IsValid() {
		return false, nil
	}
	if rv.Kind() == reflect.Ptr && rv.IsNil() {
		return false, nil
	}
	if rv.CanInterface() {
		if m, ok := rv.Interface().(stdjson.Marshaler); ok {
			data, err := m.MarshalJSON()
			if err != nil {
				return true, err
			}
			sess.buf = append(sess.buf, data...)
			return true, nil
		}
		if tm, ok := rv.Interface().(encoding.TextMarshaler); ok {
			data, err := tm.MarshalText()
			if err != nil {
				return true, err
			}
			sess.buf = strconv.AppendQuote(sess.buf, string(data))
			return true, nil
		}
	}
	if rv.Kind() != reflect.Ptr && rv.CanAddr() {
		pv := rv.Addr()
		if pv.CanInterface() {
			if m, ok := pv.Interface().(stdjson.Marshaler); ok {
				data, err := m.MarshalJSON()
				if err != nil {
					return true, err
				}
				sess.buf = append(sess.buf, data...)
				return true, nil
			}
			if tm, ok := pv.Interface().(encoding.TextMarshaler); ok {
				data, err := tm.MarshalText()
				if err != nil {
					return true, err
				}
				sess.buf = strconv.AppendQuote(sess.buf, string(data))
				return true, nil
			}
		}
	}
	return false, nil
}

func (e *Engine) hasCustomMarshalerType(rt reflect.Type) bool {
	if isTimeTypeOrPtr(rt) {
		return false
	}
	e.customMu.RLock()
	if cached, ok := e.customTypes[rt]; ok {
		e.customMu.RUnlock()
		return cached
	}
	e.customMu.RUnlock()

	has := rt.Implements(jsonMarshalerType) || rt.Implements(textMarshalerType)
	if !has && rt.Kind() != reflect.Ptr {
		prt := reflect.PointerTo(rt)
		has = prt.Implements(jsonMarshalerType) || prt.Implements(textMarshalerType)
	}

	e.customMu.Lock()
	e.customTypes[rt] = has
	e.customMu.Unlock()
	return has
}

func isTimeTypeOrPtr(rt reflect.Type) bool {
	for rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}
	return rt == timeType
}

func (e *Engine) appendStructStatic(sess *encoderSession, rv reflect.Value) error {
	rt := rv.Type()
	plan := e.getStaticPlan(rt)
	if plan.inlineIdx >= 0 {
		p := &plan.fields[plan.inlineIdx]
		if p.inlineRaw {
			structPtr := structPointer(rv, rt)
			raw := inlineRaw(p.xField.Pointer(structPtr), p.kind)
			if len(raw) == 0 {
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			sess.buf = append(sess.buf, raw...)
			return nil
		}
		return e.appendValue(sess, rv.Field(plan.inlineIdx))
	}
	structPtr := structPointer(rv, rt)
	if handled, err := e.tryInlineRawMessage(sess, plan, structPtr); handled || err != nil {
		return err
	}
	sess.buf = append(sess.buf, '{')
	fieldCounter := 0
	for i := range plan.fields {
		p := &plan.fields[i]
		if p.ignore {
			continue
		}
		if p.anonymous || p.inline {
			if p.inlineRaw {
				fieldPtr := p.xField.Pointer(structPtr)
				raw := inlineRaw(fieldPtr, p.kind)
				if len(raw) == 0 {
					continue
				}
				if fieldCounter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				fieldCounter++
				sess.buf = append(sess.buf, raw...)
				continue
			}
			inlineVal := rv.Field(p.index)
			skipInline := false
			for inlineVal.Kind() == reflect.Interface || inlineVal.Kind() == reflect.Ptr {
				if inlineVal.IsNil() {
					skipInline = true
					break
				}
				inlineVal = inlineVal.Elem()
			}
			if !skipInline && inlineVal.Kind() == reflect.Struct {
				if err := e.appendStructFieldsStatic(sess, inlineVal, &fieldCounter); err != nil {
					return err
				}
			}
			continue
		}
		fieldPtr := p.xField.Pointer(structPtr)
		if p.fast {
			if (p.omitempty || e.omitEmpty) && p.emptyFn(fieldPtr) {
				continue
			}
		} else {
			fv := rv.Field(p.index)
			if (p.omitempty || e.omitEmpty) && isEmptyValue(fv) {
				continue
			}
		}
		if fieldCounter > 0 {
			sess.buf = append(sess.buf, ',')
		}
		fieldCounter++
		sess.buf = append(sess.buf, p.keyLit...)
		if p.fast {
			if err := p.appendFn(&sess.buf, fieldPtr); err != nil {
				return err
			}
		} else {
			fv := rv.Field(p.index)
			if err := e.appendValue(sess, fv); err != nil {
				return err
			}
		}
	}
	sess.buf = append(sess.buf, '}')
	return nil
}

func (e *Engine) appendStructFieldsStatic(sess *encoderSession, rv reflect.Value, counter *int) error {
	rt := rv.Type()
	plan := e.getStaticPlan(rt)
	if plan.inlineIdx >= 0 {
		if *counter > 0 {
			sess.buf = append(sess.buf, ',')
		}
		*counter++
		return e.appendValue(sess, rv.Field(plan.inlineIdx))
	}
	structPtr := structPointer(rv, rt)
	for i := range plan.fields {
		p := &plan.fields[i]
		if p.ignore {
			continue
		}
		if p.anonymous || p.inline {
			if p.inlineRaw {
				fieldPtr := p.xField.Pointer(structPtr)
				raw := inlineRaw(fieldPtr, p.kind)
				if len(raw) == 0 {
					continue
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, raw...)
				continue
			}
			inlineVal := rv.Field(p.index)
			skipInline := false
			for inlineVal.Kind() == reflect.Interface || inlineVal.Kind() == reflect.Ptr {
				if inlineVal.IsNil() {
					skipInline = true
					break
				}
				inlineVal = inlineVal.Elem()
			}
			if !skipInline && inlineVal.Kind() == reflect.Struct {
				if err := e.appendStructFieldsStatic(sess, inlineVal, counter); err != nil {
					return err
				}
			}
			continue
		}
		fieldPtr := p.xField.Pointer(structPtr)
		if p.fast {
			if (p.omitempty || e.omitEmpty) && p.emptyFn(fieldPtr) {
				continue
			}
		} else {
			fv := rv.Field(p.index)
			if (p.omitempty || e.omitEmpty) && isEmptyValue(fv) {
				continue
			}
		}
		if *counter > 0 {
			sess.buf = append(sess.buf, ',')
		}
		*counter++
		sess.buf = append(sess.buf, p.keyLit...)
		if p.fast {
			if err := p.appendFn(&sess.buf, fieldPtr); err != nil {
				return err
			}
		} else {
			fv := rv.Field(p.index)
			if err := e.appendValue(sess, fv); err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *Engine) appendStructDynamic(sess *encoderSession, rv reflect.Value) error {
	rv, structPtr := addressableStruct(rv)
	rt := rv.Type()
	plan := e.getDynamicPlan(rt)
	if plan.inlineIdx >= 0 {
		p := &plan.fields[plan.inlineIdx]
		if p.inlineRaw {
			raw := inlineRaw(p.xField.Pointer(structPtr), p.kind)
			if len(raw) == 0 {
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			sess.buf = append(sess.buf, raw...)
			return nil
		}
		return e.appendValue(sess, rv.Field(plan.inlineIdx))
	}
	if handled, err := e.tryInlineRawMessage(sess, plan, structPtr); handled || err != nil {
		return err
	}
	sess.buf = append(sess.buf, '{')
	fieldCounter := 0
	if err := e.appendStructFieldsDynamic(sess, rv, plan, structPtr, &fieldCounter); err != nil {
		return err
	}
	sess.buf = append(sess.buf, '}')
	return nil
}

func (e *Engine) appendStructFieldsDynamic(sess *encoderSession, rv reflect.Value, plan *structPlan, structPtr unsafe.Pointer, counter *int) error {
	path := e.currentPath(sess)
	for i := range plan.fields {
		p := &plan.fields[i]
		if p.ignore {
			continue
		}
		if p.anonymous || p.inline {
			if p.inlineRaw {
				fieldPtr := p.xField.Pointer(structPtr)
				raw := inlineRaw(fieldPtr, p.kind)
				if len(raw) == 0 {
					continue
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, raw...)
				continue
			}
			inlineVal := reflect.NewAt(p.rType, p.xField.Pointer(structPtr)).Elem()
			for inlineVal.Kind() == reflect.Interface || inlineVal.Kind() == reflect.Ptr {
				if inlineVal.IsNil() {
					inlineVal = reflect.Value{}
					break
				}
				inlineVal = inlineVal.Elem()
			}
			if inlineVal.IsValid() && inlineVal.Kind() == reflect.Struct {
				inlineVal, inlinePtr := addressableStruct(inlineVal)
				inlinePlan := e.getDynamicPlan(inlineVal.Type())
				if err := e.appendStructFieldsDynamic(sess, inlineVal, inlinePlan, inlinePtr, counter); err != nil {
					return err
				}
			}
			continue
		}
		if e.hasExclude && e.Exclude(path, p.fieldName) {
			continue
		}
		name := p.name
		if e.hasTransform && !p.explicit {
			name = e.NameTransform(path, p.name)
		}
		fieldPtr := p.xField.Pointer(structPtr)
		if p.fast {
			if (p.omitempty || e.omitEmpty) && p.emptyFn(fieldPtr) {
				continue
			}
		} else {
			fv := reflect.NewAt(p.rType, fieldPtr).Elem()
			if (p.omitempty || e.omitEmpty) && isEmptyValue(fv) {
				continue
			}
		}
		if *counter > 0 {
			sess.buf = append(sess.buf, ',')
		}
		*counter++
		sess.buf = strconv.AppendQuote(sess.buf, name)
		sess.buf = append(sess.buf, ':')
		if p.fast {
			if err := p.appendFn(&sess.buf, fieldPtr); err != nil {
				return err
			}
		} else {
			fv := reflect.NewAt(p.rType, fieldPtr).Elem()
			if p.rType == timeType {
				ts := xunsafe.AsTime(fieldPtr)
				if p.nullable && ts.IsZero() {
					sess.buf = append(sess.buf, "null"...)
					continue
				}
				layout := e.timeLayout
				if p.timeLayout != "" {
					layout = p.timeLayout
				}
				sess.buf = strconv.AppendQuote(sess.buf, ts.Format(layout))
				continue
			}
			if p.kind == reflect.Ptr && p.ptrElem == reflect.Struct && p.rType.Elem() == timeType {
				timePtr := *(*unsafe.Pointer)(fieldPtr)
				if timePtr == nil {
					sess.buf = append(sess.buf, "null"...)
					continue
				}
				layout := e.timeLayout
				if p.timeLayout != "" {
					layout = p.timeLayout
				}
				sess.buf = strconv.AppendQuote(sess.buf, xunsafe.AsTime(timePtr).Format(layout))
				continue
			}
			if p.nullable && isEmptyValue(fv) {
				sess.buf = append(sess.buf, "null"...)
				continue
			}
			if e.hasTransform || e.hasExclude {
				sess.path.push(name)
				err := e.appendValue(sess, fv)
				sess.path.pop()
				if err != nil {
					return err
				}
			} else {
				if err := e.appendValue(sess, fv); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func addressableStruct(rv reflect.Value) (reflect.Value, unsafe.Pointer) {
	rt := rv.Type()
	if rv.CanAddr() {
		return rv, unsafe.Pointer(rv.Addr().Pointer())
	}
	tmp := reflect.New(rt)
	tmp.Elem().Set(rv)
	return tmp.Elem(), unsafe.Pointer(tmp.Pointer())
}

func (e *Engine) appendMapStatic(sess *encoderSession, rv reflect.Value) error {
	sess.buf = append(sess.buf, '{')
	idx := 0
	iter := rv.MapRange()
	for iter.Next() {
		k := iter.Key()
		if k.Kind() != reflect.String {
			continue
		}
		if idx > 0 {
			sess.buf = append(sess.buf, ',')
		}
		idx++
		sess.buf = strconv.AppendQuote(sess.buf, k.String())
		sess.buf = append(sess.buf, ':')
		if err := e.appendValue(sess, iter.Value()); err != nil {
			return err
		}
	}
	sess.buf = append(sess.buf, '}')
	return nil
}

func (e *Engine) appendMapDynamic(sess *encoderSession, rv reflect.Value) error {
	sess.buf = append(sess.buf, '{')
	idx := 0
	path := e.currentPath(sess)
	iter := rv.MapRange()
	for iter.Next() {
		k := iter.Key()
		if k.Kind() != reflect.String {
			continue
		}
		name := k.String()
		if e.hasTransform {
			name = e.NameTransform(path, name)
		}
		if e.hasExclude && e.Exclude(path, name) {
			continue
		}
		if idx > 0 {
			sess.buf = append(sess.buf, ',')
		}
		idx++
		sess.buf = strconv.AppendQuote(sess.buf, name)
		sess.buf = append(sess.buf, ':')
		sess.path.push(name)
		err := e.appendValue(sess, iter.Value())
		sess.path.pop()
		if err != nil {
			return err
		}
	}
	sess.buf = append(sess.buf, '}')
	return nil
}

func structPointer(rv reflect.Value, rt reflect.Type) unsafe.Pointer {
	if rv.CanAddr() {
		return unsafe.Pointer(rv.Addr().Pointer())
	}
	tmp := reflect.New(rt)
	tmp.Elem().Set(rv)
	return unsafe.Pointer(tmp.Pointer())
}

func (e *Engine) getStaticPlan(rt reflect.Type) *structPlan {
	e.planMu.RLock()
	if p := e.staticPlans[rt]; p != nil {
		e.planMu.RUnlock()
		return p
	}
	e.planMu.RUnlock()
	plan := buildStructPlan(rt, e.compileName, e.hasCustomMarshalerType)
	e.planMu.Lock()
	if p := e.staticPlans[rt]; p != nil {
		e.planMu.Unlock()
		return p
	}
	e.staticPlans[rt] = plan
	e.planMu.Unlock()
	return plan
}

func (e *Engine) getDynamicPlan(rt reflect.Type) *structPlan {
	e.planMu.RLock()
	if p := e.dynamicPlans[rt]; p != nil {
		e.planMu.RUnlock()
		return p
	}
	e.planMu.RUnlock()
	plan := buildStructPlan(rt, nil, e.hasCustomMarshalerType)
	e.planMu.Lock()
	if p := e.dynamicPlans[rt]; p != nil {
		e.planMu.Unlock()
		return p
	}
	e.dynamicPlans[rt] = plan
	e.planMu.Unlock()
	return plan
}

func buildStructPlan(rt reflect.Type, compileName func(string) string, hasCustom func(reflect.Type) bool) *structPlan {
	result := &structPlan{
		fields:    make([]fieldPlan, 0, rt.NumField()),
		fastOnly:  true,
		fastOps:   make([]fastFieldOp, 0, rt.NumField()),
		staticOps: make([]staticFieldOp, 0, rt.NumField()),
		inlineIdx: -1,
	}
	inlineCandidates := make([]int, 0, 1)
	hasExplicitNonInline := false
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		if field.PkgPath != "" {
			continue
		}
		if field.Tag.Get("setMarker") == "true" {
			continue
		}
		resolved := tagutil.ResolveFieldTag(field)
		fTag := resolved.Format
		inline := resolved.Inline
		ignore := resolved.Ignore
		inlineRaw := inline && (field.Type == rawMsgType || (field.Type.Kind() == reflect.Ptr && field.Type.Elem() == rawMsgType))
		name := resolved.Name
		explicit := resolved.Explicit
		if compileName != nil && !explicit {
			name = compileName(name)
		}
		kind := field.Type.Kind()
		ptrElem := reflect.Invalid
		if kind == reflect.Ptr {
			ptrElem = field.Type.Elem().Kind()
		}
		fast := isFastPrimitiveKind(kind, ptrElem)
		if fTag.Nullable {
			fast = false
		}
		if hasCustom != nil && hasCustom(field.Type) {
			fast = false
		}
		fp := fieldPlan{
			fieldName:  field.Name,
			name:       name,
			keyLit:     appendQuotedName(nil, name),
			omitempty:  resolved.OmitEmpty,
			nullable:   fTag.Nullable,
			anonymous:  field.Anonymous,
			inline:     inline,
			ignore:     ignore,
			index:      i,
			kind:       kind,
			rType:      field.Type,
			xField:     xunsafe.NewField(field),
			explicit:   explicit,
			inlineRaw:  inlineRaw,
			timeLayout: fTag.TimeLayout,
			ptrElem:    ptrElem,
			fast:       fast,
			appendFn:   primitiveAppendFunc(kind, ptrElem),
			emptyFn:    primitiveEmptyFunc(kind, ptrElem),
		}
		result.fields = append(result.fields, fp)
		if !ignore {
			if fp.inline && !fp.anonymous {
				inlineCandidates = append(inlineCandidates, len(result.fields)-1)
			} else if explicit {
				hasExplicitNonInline = true
			}
			if !fast || inline || field.Anonymous {
				result.fastOnly = false
			} else {
				result.fastOps = append(result.fastOps, fastFieldOp{
					xField:  fp.xField,
					keyLit:  fp.keyLit,
					omit:    fp.omitempty,
					emptyFn: fp.emptyFn,
					kind:    fp.kind,
					ptrElem: fp.ptrElem,
				})
			}
			result.staticOps = append(result.staticOps, compileStaticFieldOp(fp))
		}
	}
	if len(inlineCandidates) == 1 && !hasExplicitNonInline {
		result.inlineIdx = inlineCandidates[0]
	}
	return result
}

func compileStaticFieldOp(fp fieldPlan) staticFieldOp {
	xField := fp.xField
	keyLit := fp.keyLit
	omit := fp.omitempty

	if fp.anonymous || fp.inline {
		if fp.inlineRaw {
			kind := fp.kind
			return func(_ *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				raw := inlineRaw(xField.Pointer(structPtr), kind)
				if len(raw) == 0 {
					return nil
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, raw...)
				return nil
			}
		}
		rType := fp.rType
		return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
			inlineVal := reflect.NewAt(rType, xField.Pointer(structPtr)).Elem()
			for inlineVal.Kind() == reflect.Interface || inlineVal.Kind() == reflect.Ptr {
				if inlineVal.IsNil() {
					return nil
				}
				inlineVal = inlineVal.Elem()
			}
			if inlineVal.Kind() == reflect.Struct {
				return e.appendStructFieldsStatic(sess, inlineVal, counter)
			}
			return nil
		}
	}

	if fp.fast {
		appendFn := fp.appendFn
		emptyFn := fp.emptyFn
		nullable := fp.nullable
		return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
			fieldPtr := xField.Pointer(structPtr)
			if (omit || e.omitEmpty) && emptyFn(fieldPtr) {
				return nil
			}
			if *counter > 0 {
				sess.buf = append(sess.buf, ',')
			}
			*counter++
			sess.buf = append(sess.buf, keyLit...)
			if nullable && emptyFn(fieldPtr) {
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			return appendFn(&sess.buf, fieldPtr)
		}
	}

	if fp.kind == reflect.Struct {
		rType := fp.rType
		if rType == timeType {
			fieldLayout := fp.timeLayout
			nullable := fp.nullable
			return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				fieldPtr := xField.Pointer(structPtr)
				ts := xunsafe.AsTime(fieldPtr)
				if (omit || e.omitEmpty) && ts.IsZero() {
					return nil
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				if nullable && ts.IsZero() {
					sess.buf = append(sess.buf, "null"...)
					return nil
				}
				layout := e.timeLayout
				if fieldLayout != "" {
					layout = fieldLayout
				}
				sess.buf = strconv.AppendQuote(sess.buf, ts.Format(layout))
				return nil
			}
		}
		return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
			fieldPtr := xField.Pointer(structPtr)
			if *counter > 0 {
				sess.buf = append(sess.buf, ',')
			}
			*counter++
			sess.buf = append(sess.buf, keyLit...)
			return e.appendStructStaticPtr(sess, rType, fieldPtr)
		}
	}

	if fp.kind == reflect.Ptr && fp.ptrElem == reflect.Struct {
		elemType := fp.rType.Elem()
		if elemType == timeType {
			fieldLayout := fp.timeLayout
			return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				fieldPtr := *(*unsafe.Pointer)(xField.Pointer(structPtr))
				if fieldPtr == nil {
					if omit || e.omitEmpty {
						return nil
					}
					if *counter > 0 {
						sess.buf = append(sess.buf, ',')
					}
					*counter++
					sess.buf = append(sess.buf, keyLit...)
					sess.buf = append(sess.buf, "null"...)
					return nil
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				layout := e.timeLayout
				if fieldLayout != "" {
					layout = fieldLayout
				}
				sess.buf = strconv.AppendQuote(sess.buf, xunsafe.AsTime(fieldPtr).Format(layout))
				return nil
			}
		}
		return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
			fieldPtr := *(*unsafe.Pointer)(xField.Pointer(structPtr))
			if fieldPtr == nil {
				if omit || e.omitEmpty {
					return nil
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				sess.buf = append(sess.buf, "null"...)
				return nil
			}
			if *counter > 0 {
				sess.buf = append(sess.buf, ',')
			}
			*counter++
			sess.buf = append(sess.buf, keyLit...)
			return e.appendStructStaticPtr(sess, elemType, fieldPtr)
		}
	}

	if fp.kind == reflect.Slice {
		if emit := primitiveSliceAppendFunc(fp.rType.Elem().Kind()); emit != nil {
			return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				fieldPtr := xField.Pointer(structPtr)
				fv := reflect.NewAt(fp.rType, fieldPtr).Elem()
				if (omit || e.omitEmpty) && fv.Len() == 0 {
					return nil
				}
				if fv.IsNil() {
					if *counter > 0 {
						sess.buf = append(sess.buf, ',')
					}
					*counter++
					sess.buf = append(sess.buf, keyLit...)
					if e.nilSliceNull {
						sess.buf = append(sess.buf, "null"...)
					} else {
						sess.buf = append(sess.buf, '[', ']')
					}
					return nil
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				return emit(&sess.buf, fieldPtr)
			}
		}
	}

	if fp.kind == reflect.Map && fp.rType.Key().Kind() == reflect.String {
		elemKind := fp.rType.Elem().Kind()
		if elemKind == reflect.String {
			return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				m := *(*map[string]string)(xField.Pointer(structPtr))
				if omit || e.omitEmpty {
					if len(m) == 0 {
						return nil
					}
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				if m == nil {
					sess.buf = append(sess.buf, "null"...)
					return nil
				}
				sess.buf = append(sess.buf, '{')
				idx := 0
				for k, v := range m {
					if idx > 0 {
						sess.buf = append(sess.buf, ',')
					}
					idx++
					sess.buf = strconv.AppendQuote(sess.buf, k)
					sess.buf = append(sess.buf, ':')
					sess.buf = strconv.AppendQuote(sess.buf, v)
				}
				sess.buf = append(sess.buf, '}')
				return nil
			}
		}
		if elemKind == reflect.Interface {
			return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
				m := *(*map[string]interface{})(xField.Pointer(structPtr))
				if omit || e.omitEmpty {
					if len(m) == 0 {
						return nil
					}
				}
				if *counter > 0 {
					sess.buf = append(sess.buf, ',')
				}
				*counter++
				sess.buf = append(sess.buf, keyLit...)
				if m == nil {
					sess.buf = append(sess.buf, "null"...)
					return nil
				}
				sess.buf = append(sess.buf, '{')
				idx := 0
				for k, v := range m {
					if idx > 0 {
						sess.buf = append(sess.buf, ',')
					}
					idx++
					sess.buf = strconv.AppendQuote(sess.buf, k)
					sess.buf = append(sess.buf, ':')
					if err := e.appendValue(sess, reflect.ValueOf(v)); err != nil {
						return err
					}
				}
				sess.buf = append(sess.buf, '}')
				return nil
			}
		}
	}

	rType := fp.rType
	nullable := fp.nullable
	return func(e *Engine, sess *encoderSession, structPtr unsafe.Pointer, counter *int) error {
		fieldPtr := xField.Pointer(structPtr)
		fv := reflect.NewAt(rType, fieldPtr).Elem()
		if (omit || e.omitEmpty) && isEmptyValue(fv) {
			return nil
		}
		if *counter > 0 {
			sess.buf = append(sess.buf, ',')
		}
		*counter++
		sess.buf = append(sess.buf, keyLit...)
		if nullable && isEmptyValue(fv) {
			sess.buf = append(sess.buf, "null"...)
			return nil
		}
		return e.appendValue(sess, fv)
	}
}

func primitiveSliceAppendFunc(elemKind reflect.Kind) func(*[]byte, unsafe.Pointer) error {
	switch elemKind {
	case reflect.String:
		return appendStringSlice
	case reflect.Bool:
		return appendBoolSlice
	case reflect.Int:
		return appendIntSlice
	case reflect.Int8:
		return appendInt8Slice
	case reflect.Int16:
		return appendInt16Slice
	case reflect.Int32:
		return appendInt32Slice
	case reflect.Int64:
		return appendInt64Slice
	case reflect.Uint:
		return appendUintSlice
	case reflect.Uint8:
		return appendUint8Slice
	case reflect.Uint16:
		return appendUint16Slice
	case reflect.Uint32:
		return appendUint32Slice
	case reflect.Uint64, reflect.Uintptr:
		return appendUint64Slice
	case reflect.Float32:
		return appendFloat32Slice
	case reflect.Float64:
		return appendFloat64Slice
	default:
		return nil
	}
}

func appendStringSlice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]string)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		appendQuotedStringFast(buf, items[i])
	}
	*buf = append(*buf, ']')
	return nil
}
func appendBoolSlice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]bool)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		if items[i] {
			*buf = append(*buf, "true"...)
		} else {
			*buf = append(*buf, "false"...)
		}
	}
	*buf = append(*buf, ']')
	return nil
}
func appendIntSlice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]int)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendInt(*buf, int64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendInt8Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]int8)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendInt(*buf, int64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendInt16Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]int16)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendInt(*buf, int64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendInt32Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]int32)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendInt(*buf, int64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendInt64Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]int64)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendInt(*buf, items[i], 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendUintSlice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]uint)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendUint(*buf, uint64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendUint8Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]uint8)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendUint(*buf, uint64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendUint16Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]uint16)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendUint(*buf, uint64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendUint32Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]uint32)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendUint(*buf, uint64(items[i]), 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendUint64Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]uint64)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendUint(*buf, items[i], 10)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendFloat32Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]float32)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendFloat(*buf, float64(items[i]), 'g', -1, 32)
	}
	*buf = append(*buf, ']')
	return nil
}
func appendFloat64Slice(buf *[]byte, fieldPtr unsafe.Pointer) error {
	items := *(*[]float64)(fieldPtr)
	*buf = append(*buf, '[')
	for i := 0; i < len(items); i++ {
		if i > 0 {
			*buf = append(*buf, ',')
		}
		*buf = strconv.AppendFloat(*buf, items[i], 'g', -1, 64)
	}
	*buf = append(*buf, ']')
	return nil
}

func primitiveAppendFunc(kind, ptrElem reflect.Kind) func(*[]byte, unsafe.Pointer) error {
	switch kind {
	case reflect.String:
		return appendStringPointer
	case reflect.Bool:
		return appendBoolPointer
	case reflect.Int:
		return appendIntPointer
	case reflect.Int8:
		return appendInt8Pointer
	case reflect.Int16:
		return appendInt16Pointer
	case reflect.Int32:
		return appendInt32Pointer
	case reflect.Int64:
		return appendInt64Pointer
	case reflect.Uint:
		return appendUintPointer
	case reflect.Uint8:
		return appendUint8Pointer
	case reflect.Uint16:
		return appendUint16Pointer
	case reflect.Uint32:
		return appendUint32Pointer
	case reflect.Uint64, reflect.Uintptr:
		return appendUint64Pointer
	case reflect.Float32:
		return appendFloat32Pointer
	case reflect.Float64:
		return appendFloat64Pointer
	case reflect.Ptr:
		inner := primitiveAppendFunc(ptrElem, reflect.Invalid)
		if inner == nil {
			return nil
		}
		return func(buf *[]byte, ptr unsafe.Pointer) error {
			p := *(*unsafe.Pointer)(ptr)
			if p == nil {
				*buf = append(*buf, "null"...)
				return nil
			}
			return inner(buf, p)
		}
	default:
		return nil
	}
}

func primitiveEmptyFunc(kind, ptrElem reflect.Kind) func(unsafe.Pointer) bool {
	switch kind {
	case reflect.String:
		return isEmptyStringPointer
	case reflect.Bool:
		return isEmptyBoolPointer
	case reflect.Int:
		return isEmptyIntPointer
	case reflect.Int8:
		return isEmptyInt8Pointer
	case reflect.Int16:
		return isEmptyInt16Pointer
	case reflect.Int32:
		return isEmptyInt32Pointer
	case reflect.Int64:
		return isEmptyInt64Pointer
	case reflect.Uint:
		return isEmptyUintPointer
	case reflect.Uint8:
		return isEmptyUint8Pointer
	case reflect.Uint16:
		return isEmptyUint16Pointer
	case reflect.Uint32:
		return isEmptyUint32Pointer
	case reflect.Uint64, reflect.Uintptr:
		return isEmptyUint64Pointer
	case reflect.Float32:
		return isEmptyFloat32Pointer
	case reflect.Float64:
		return isEmptyFloat64Pointer
	case reflect.Ptr:
		inner := primitiveEmptyFunc(ptrElem, reflect.Invalid)
		if inner == nil {
			return nil
		}
		return func(ptr unsafe.Pointer) bool {
			p := *(*unsafe.Pointer)(ptr)
			if p == nil {
				return true
			}
			return inner(p)
		}
	default:
		return nil
	}
}

func isFastPrimitiveKind(kind, ptrElem reflect.Kind) bool {
	switch kind {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	case reflect.Ptr:
		switch ptrElem {
		case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
			reflect.Float32, reflect.Float64:
			return true
		}
	}
	return false
}

func appendQuotedName(dst []byte, name string) []byte {
	dst = strconv.AppendQuote(dst, name)
	dst = append(dst, ':')
	return dst
}

func appendStringPointer(buf *[]byte, ptr unsafe.Pointer) error {
	appendQuotedStringFast(buf, xunsafe.AsString(ptr))
	return nil
}
func appendBoolPointer(buf *[]byte, ptr unsafe.Pointer) error {
	if xunsafe.AsBool(ptr) {
		*buf = append(*buf, "true"...)
	} else {
		*buf = append(*buf, "false"...)
	}
	return nil
}
func appendIntPointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendInt(*buf, int64(xunsafe.AsInt(ptr)), 10)
	return nil
}
func appendInt8Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendInt(*buf, int64(xunsafe.AsInt8(ptr)), 10)
	return nil
}
func appendInt16Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendInt(*buf, int64(xunsafe.AsInt16(ptr)), 10)
	return nil
}
func appendInt32Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendInt(*buf, int64(xunsafe.AsInt32(ptr)), 10)
	return nil
}
func appendInt64Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendInt(*buf, xunsafe.AsInt64(ptr), 10)
	return nil
}
func appendUintPointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendUint(*buf, uint64(xunsafe.AsUint(ptr)), 10)
	return nil
}
func appendUint8Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendUint(*buf, uint64(xunsafe.AsUint8(ptr)), 10)
	return nil
}
func appendUint16Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendUint(*buf, uint64(xunsafe.AsUint16(ptr)), 10)
	return nil
}
func appendUint32Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendUint(*buf, uint64(xunsafe.AsUint32(ptr)), 10)
	return nil
}
func appendUint64Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendUint(*buf, xunsafe.AsUint64(ptr), 10)
	return nil
}
func appendFloat32Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendFloat(*buf, float64(xunsafe.AsFloat32(ptr)), 'g', -1, 32)
	return nil
}
func appendFloat64Pointer(buf *[]byte, ptr unsafe.Pointer) error {
	*buf = strconv.AppendFloat(*buf, xunsafe.AsFloat64(ptr), 'g', -1, 64)
	return nil
}

func isEmptyStringPointer(ptr unsafe.Pointer) bool  { return xunsafe.AsString(ptr) == "" }
func isEmptyBoolPointer(ptr unsafe.Pointer) bool    { return !xunsafe.AsBool(ptr) }
func isEmptyIntPointer(ptr unsafe.Pointer) bool     { return xunsafe.AsInt(ptr) == 0 }
func isEmptyInt8Pointer(ptr unsafe.Pointer) bool    { return xunsafe.AsInt8(ptr) == 0 }
func isEmptyInt16Pointer(ptr unsafe.Pointer) bool   { return xunsafe.AsInt16(ptr) == 0 }
func isEmptyInt32Pointer(ptr unsafe.Pointer) bool   { return xunsafe.AsInt32(ptr) == 0 }
func isEmptyInt64Pointer(ptr unsafe.Pointer) bool   { return xunsafe.AsInt64(ptr) == 0 }
func isEmptyUintPointer(ptr unsafe.Pointer) bool    { return xunsafe.AsUint(ptr) == 0 }
func isEmptyUint8Pointer(ptr unsafe.Pointer) bool   { return xunsafe.AsUint8(ptr) == 0 }
func isEmptyUint16Pointer(ptr unsafe.Pointer) bool  { return xunsafe.AsUint16(ptr) == 0 }
func isEmptyUint32Pointer(ptr unsafe.Pointer) bool  { return xunsafe.AsUint32(ptr) == 0 }
func isEmptyUint64Pointer(ptr unsafe.Pointer) bool  { return xunsafe.AsUint64(ptr) == 0 }
func isEmptyFloat32Pointer(ptr unsafe.Pointer) bool { return xunsafe.AsFloat32(ptr) == 0 }
func isEmptyFloat64Pointer(ptr unsafe.Pointer) bool { return xunsafe.AsFloat64(ptr) == 0 }

func appendPrimitiveFast(dst []byte, ptr unsafe.Pointer, kind reflect.Kind, ptrElem reflect.Kind) ([]byte, error) {
	switch kind {
	case reflect.String:
		dst = appendQuotedStringFastTo(dst, xunsafe.AsString(ptr))
	case reflect.Bool:
		if xunsafe.AsBool(ptr) {
			dst = append(dst, "true"...)
		} else {
			dst = append(dst, "false"...)
		}
	case reflect.Int:
		dst = strconv.AppendInt(dst, int64(xunsafe.AsInt(ptr)), 10)
	case reflect.Int8:
		dst = strconv.AppendInt(dst, int64(xunsafe.AsInt8(ptr)), 10)
	case reflect.Int16:
		dst = strconv.AppendInt(dst, int64(xunsafe.AsInt16(ptr)), 10)
	case reflect.Int32:
		dst = strconv.AppendInt(dst, int64(xunsafe.AsInt32(ptr)), 10)
	case reflect.Int64:
		dst = strconv.AppendInt(dst, xunsafe.AsInt64(ptr), 10)
	case reflect.Uint:
		dst = strconv.AppendUint(dst, uint64(xunsafe.AsUint(ptr)), 10)
	case reflect.Uint8:
		dst = strconv.AppendUint(dst, uint64(xunsafe.AsUint8(ptr)), 10)
	case reflect.Uint16:
		dst = strconv.AppendUint(dst, uint64(xunsafe.AsUint16(ptr)), 10)
	case reflect.Uint32:
		dst = strconv.AppendUint(dst, uint64(xunsafe.AsUint32(ptr)), 10)
	case reflect.Uint64, reflect.Uintptr:
		dst = strconv.AppendUint(dst, xunsafe.AsUint64(ptr), 10)
	case reflect.Float32:
		dst = strconv.AppendFloat(dst, float64(xunsafe.AsFloat32(ptr)), 'g', -1, 32)
	case reflect.Float64:
		dst = strconv.AppendFloat(dst, xunsafe.AsFloat64(ptr), 'g', -1, 64)
	case reflect.Ptr:
		p := *(*unsafe.Pointer)(ptr)
		if p == nil {
			dst = append(dst, "null"...)
			return dst, nil
		}
		return appendPrimitiveFast(dst, p, ptrElem, reflect.Invalid)
	default:
		return nil, fmt.Errorf("unsupported primitive kind: %s", kind)
	}
	return dst, nil
}

func appendQuotedStringFast(buf *[]byte, s string) {
	*buf = appendQuotedStringFastTo(*buf, s)
}

func appendQuotedStringFastTo(dst []byte, s string) []byte {
	start := len(dst)
	dst = append(dst, '"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x20 || c == '"' || c == '\\' {
			dst = dst[:start]
			dst = strconv.AppendQuote(dst, s)
			return dst
		}
	}
	dst = append(dst, s...)
	dst = append(dst, '"')
	return dst
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Struct:
		return v.IsZero()
	}
	return false
}

func inlineRaw(fieldPtr unsafe.Pointer, kind reflect.Kind) []byte {
	switch kind {
	case reflect.Slice:
		return *(*[]byte)(fieldPtr)
	case reflect.Ptr:
		p := *(*unsafe.Pointer)(fieldPtr)
		if p == nil {
			return nil
		}
		return *(*[]byte)(p)
	}
	return nil
}

func (e *Engine) tryInlineRawMessage(sess *encoderSession, plan *structPlan, structPtr unsafe.Pointer) (bool, error) {
	for i := range plan.fields {
		p := &plan.fields[i]
		if !p.inlineRaw {
			continue
		}
		raw := inlineRaw(p.xField.Pointer(structPtr), p.kind)
		if len(raw) == 0 {
			continue
		}
		sess.buf = append(sess.buf, raw...)
		return true, nil
	}
	return false, nil
}
