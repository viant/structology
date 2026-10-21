package structology

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

// Maps is a batch of keyed rows. Load replaces a destination slice; Store
// replaces the receiver only after every row has been converted successfully.
type Maps []map[string]any

// MapField binds an exact external key to a Go field. Name may be empty for
// generated types. Type is required for explicit dynamic schemas.
type MapField struct {
	Key, Name string
	Type      reflect.Type
	Tag       reflect.StructTag
	// Fields describes a nested object or repeated object. When set, Type is
	// optional; the generated field is *T or []*T according to Repeated.
	Fields   []MapField
	Repeated bool
}

// MapOption configures the Maps convenience methods. A compiled
// MapPlan should be reused directly when adapting many batches.
type MapOption func(*mapOptions)

type mapOptions struct {
	fields []MapField
	plan   *MapPlan
}

const maxMapFields = 1024

var inferredMapPlans = struct {
	sync.Mutex
	items map[string]*MapPlan
	order []string
}{items: make(map[string]*MapPlan)}

// WithMapFields specifies a stable dynamic schema, including empty batches.
func WithMapFields(fields ...MapField) MapOption {
	return func(o *mapOptions) { o.fields = append([]MapField(nil), fields...) }
}

// WithMapPlan uses a previously compiled, concurrency-safe plan.
func WithMapPlan(plan *MapPlan) MapOption {
	return func(o *mapOptions) { o.plan = plan }
}

type recordBinding struct {
	key         string
	index       int
	markerIndex int
	typeOf      reflect.Type
	child       *MapPlan
}

// MapPlan contains an immutable schema and cached field positions. It can
// be used concurrently; each Load allocates independent rows and markers.
type MapPlan struct {
	rowType     reflect.Type
	sliceType   reflect.Type
	fields      []recordBinding
	byKey       map[string]int
	markerIndex int
	markerPtr   bool
}

// Type returns the row struct type; SliceType returns its []*T batch type.
func (p *MapPlan) Type() reflect.Type      { return p.rowType }
func (p *MapPlan) SliceType() reflect.Type { return p.sliceType }

// NewMapPlan builds a runtime row type once. The generated Has pointer has
// one boolean per field and uses the established setMarker tag.
func NewMapPlan(fields []MapField) (*MapPlan, error) {
	return newMapPlan(fields, 0)
}

func newMapPlan(fields []MapField, depth int) (*MapPlan, error) {
	if depth > 32 {
		return nil, fmt.Errorf("nested map schema exceeds 32 levels")
	}
	if len(fields) > maxMapFields {
		return nil, fmt.Errorf("map schema exceeds %d fields at one level", maxMapFields)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("record schema is empty")
	}
	definitions := make([]reflect.StructField, 0, len(fields)+1)
	flags := make([]reflect.StructField, 0, len(fields))
	bindings := make([]recordBinding, 0, len(fields))
	keys := make(map[string]int, len(fields))
	names := map[string]bool{"Has": true}
	for i, f := range fields {
		var child *MapPlan
		if len(f.Fields) > 0 {
			var err error
			child, err = newMapPlan(f.Fields, depth+1)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", f.Key, err)
			}
			if f.Repeated {
				f.Type = reflect.SliceOf(reflect.PointerTo(child.rowType))
			} else {
				f.Type = reflect.PointerTo(child.rowType)
			}
		}
		if f.Key == "" {
			return nil, fmt.Errorf("field %d has an empty key", i)
		}
		if f.Type == nil {
			return nil, fmt.Errorf("field %q has no type", f.Key)
		}
		if _, exists := keys[f.Key]; exists {
			return nil, fmt.Errorf("duplicate record key %q", f.Key)
		}
		name := f.Name
		if name == "" {
			name = recordGoName(f.Key)
			base := name
			for suffix := 2; names[name]; suffix++ {
				name = fmt.Sprintf("%s%d", base, suffix)
			}
		}
		if !validRecordName(name) || names[name] {
			return nil, fmt.Errorf("invalid or duplicate Go field name %q", name)
		}
		names[name] = true
		keys[f.Key] = i
		tag := f.Tag
		if tag == "" {
			tag = reflect.StructTag("json:" + strconv.Quote(f.Key) + " sqlx:" + strconv.Quote(f.Key))
		}
		definitions = append(definitions, reflect.StructField{Name: name, Type: f.Type, Tag: tag})
		flags = append(flags, reflect.StructField{Name: name, Type: reflect.TypeOf(true)})
		bindings = append(bindings, recordBinding{key: f.Key, index: i, markerIndex: i, typeOf: f.Type, child: child})
	}
	markerType := reflect.StructOf(flags)
	definitions = append(definitions, reflect.StructField{Name: "Has", Type: reflect.PointerTo(markerType), Tag: `json:"-" sqlx:"-" setMarker:"true"`})
	rowType := reflect.StructOf(definitions)
	return &MapPlan{rowType: rowType, sliceType: reflect.SliceOf(reflect.PointerTo(rowType)), fields: bindings, byKey: keys, markerIndex: len(fields), markerPtr: true}, nil
}

// InferMapPlan discovers a deterministic union of keys. Each key must have
// one non-nil value type, except json.Number values, which retain their exact
// decimal representation. Keys with only nil values use interface{}.
func InferMapPlan(rows Maps) (*MapPlan, error) {
	return inferMapPlan(rows, 0)
}

func inferMapPlan(rows Maps, depth int) (*MapPlan, error) {
	if depth > 32 {
		return nil, fmt.Errorf("nested map data exceeds 32 levels")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("cannot infer a record schema from an empty batch; use WithMapFields or a typed destination")
	}
	types := map[string]reflect.Type{}
	values := map[string][]any{}
	for i, row := range rows {
		for key, value := range row {
			if key == "" {
				return nil, fmt.Errorf("row %d has an empty key", i)
			}
			values[key] = append(values[key], value)
			if value == nil {
				if _, ok := types[key]; !ok {
					types[key] = nil
				}
				continue
			}
			t := reflect.TypeOf(value)
			if old := types[key]; old != nil && old != t && !(isMapObject(old) && isMapObject(t)) && !(isMapSlice(old) && isMapSlice(t)) {
				return nil, fmt.Errorf("key %q has mixed value types %v and %v; supply an explicit schema", key, old, t)
			}
			types[key] = t
		}
	}
	keys := make([]string, 0, len(types))
	for key := range types {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]MapField, len(keys))
	for i, key := range keys {
		t := types[key]
		if t != nil && (isMapObject(t) || isMapSlice(t)) {
			var nested Maps
			for _, value := range values[key] {
				if value == nil {
					continue
				}
				if isMapObject(t) {
					m, err := asStringMap(value)
					if err != nil {
						return nil, fmt.Errorf("key %q: %w", key, err)
					}
					nested = append(nested, m)
				} else {
					items := reflect.ValueOf(value)
					for j := 0; j < items.Len(); j++ {
						item := items.Index(j).Interface()
						if item == nil {
							continue
						}
						m, err := asStringMap(item)
						if err != nil {
							return nil, fmt.Errorf("key %q element %d: %w", key, j, err)
						}
						nested = append(nested, m)
					}
				}
			}
			if len(nested) == 0 {
				return nil, fmt.Errorf("key %q has no nested shape; supply explicit MapField.Fields", key)
			}
			child, err := inferMapPlan(nested, depth+1)
			if err != nil {
				return nil, fmt.Errorf("key %q: %w", key, err)
			}
			childFields := child.schemaFields()
			fields[i] = MapField{Key: key, Fields: childFields, Repeated: isMapSlice(t)}
			continue
		}
		if t == nil {
			t = reflect.TypeOf((*any)(nil)).Elem()
		}
		// A scalar that can be explicitly null needs a nilable field.
		if !recordNilable(t) {
			for _, row := range rows {
				if value, ok := row[key]; ok && value == nil {
					t = reflect.PointerTo(t)
					break
				}
			}
		}
		fields[i] = MapField{Key: key, Type: t}
	}
	return cachedInferredMapPlan(fields)
}

func cachedInferredMapPlan(fields []MapField) (*MapPlan, error) {
	key := mapSchemaKey(fields)
	inferredMapPlans.Lock()
	if p := inferredMapPlans.items[key]; p != nil {
		inferredMapPlans.Unlock()
		return p, nil
	}
	inferredMapPlans.Unlock()
	p, err := NewMapPlan(fields)
	if err != nil {
		return nil, err
	}
	inferredMapPlans.Lock()
	defer inferredMapPlans.Unlock()
	if old := inferredMapPlans.items[key]; old != nil {
		return old, nil
	}
	if len(inferredMapPlans.order) == 64 {
		delete(inferredMapPlans.items, inferredMapPlans.order[0])
		inferredMapPlans.order = inferredMapPlans.order[1:]
	}
	inferredMapPlans.items[key] = p
	inferredMapPlans.order = append(inferredMapPlans.order, key)
	return p, nil
}

func mapSchemaKey(fields []MapField) string {
	var b strings.Builder
	for _, f := range fields {
		for _, part := range []string{f.Key, f.Name, string(f.Tag)} {
			b.WriteString(strconv.Itoa(len(part)))
			b.WriteByte(':')
			b.WriteString(part)
		}
		if f.Type != nil {
			b.WriteString(strconv.FormatUint(uint64(reflect.ValueOf(f.Type).Pointer()), 16))
		}
		if f.Repeated {
			b.WriteByte('R')
		} else {
			b.WriteByte('O')
		}
		b.WriteByte('[')
		b.WriteString(mapSchemaKey(f.Fields))
		b.WriteByte(']')
	}
	return b.String()
}

func (p *MapPlan) schemaFields() []MapField {
	result := make([]MapField, len(p.fields))
	for i, b := range p.fields {
		f := MapField{Key: b.key, Name: p.rowType.Field(b.index).Name, Type: b.typeOf, Tag: p.rowType.Field(b.index).Tag}
		if b.child != nil {
			f.Fields = b.child.schemaFields()
			f.Repeated = b.typeOf.Kind() == reflect.Slice
		}
		result[i] = f
	}
	return result
}

func isMapObject(t reflect.Type) bool {
	return t.Kind() == reflect.Map && t.Key().Kind() == reflect.String
}
func isMapSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && (isMapObject(t.Elem()) || t.Elem().Kind() == reflect.Interface)
}
func asStringMap(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	r := reflect.ValueOf(v)
	if !r.IsValid() || !isMapObject(r.Type()) {
		return nil, fmt.Errorf("expected string-keyed map, got %T", v)
	}
	m := make(map[string]any, r.Len())
	it := r.MapRange()
	for it.Next() {
		m[it.Key().String()] = it.Value().Interface()
	}
	return m, nil
}

// NewMapPlanForType compiles a supplied struct type. External names come
// from sqlx, then json, then the Go field name. A marker is used when present.
func NewMapPlanForType(t reflect.Type) (*MapPlan, error) {
	return newMapPlanForType(t, map[reflect.Type]bool{}, 0)
}

func newMapPlanForType(t reflect.Type, seen map[reflect.Type]bool, depth int) (*MapPlan, error) {
	if depth > 32 {
		return nil, fmt.Errorf("nested struct schema exceeds 32 levels")
	}
	if t == nil {
		return nil, fmt.Errorf("nil record type")
	}
	for t.Kind() == reflect.Ptr || t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("record type %v is not a struct", t)
	}
	if seen[t] {
		return nil, fmt.Errorf("recursive struct type %v is unsupported", t)
	}
	seen[t] = true
	defer delete(seen, t)
	p := &MapPlan{rowType: t, sliceType: reflect.SliceOf(reflect.PointerTo(t)), byKey: map[string]int{}, markerIndex: -1}
	var markerType reflect.Type
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if IsSetMarker(f.Tag) {
			if p.markerIndex >= 0 {
				return nil, fmt.Errorf("multiple set markers in %v", t)
			}
			p.markerIndex = i
			markerType = f.Type
			if markerType.Kind() == reflect.Ptr {
				p.markerPtr = true
				markerType = markerType.Elem()
			}
			if markerType.Kind() != reflect.Struct {
				return nil, fmt.Errorf("set marker %s must be a struct or pointer to struct", f.Name)
			}
			continue
		}
		if f.PkgPath != "" {
			continue
		}
		key := recordTagName(f.Tag.Get("sqlx"))
		if key == "-" {
			continue
		}
		if key == "" {
			key = recordTagName(f.Tag.Get("json"))
		}
		if key == "-" {
			continue
		}
		if key == "" {
			key = f.Name
		}
		if _, ok := p.byKey[key]; ok {
			return nil, fmt.Errorf("duplicate record key %q", key)
		}
		p.byKey[key] = len(p.fields)
		b := recordBinding{key: key, index: i, markerIndex: -1, typeOf: f.Type}
		childType := f.Type
		for childType.Kind() == reflect.Ptr || childType.Kind() == reflect.Slice {
			childType = childType.Elem()
		}
		if childType.Kind() == reflect.Struct && childType != timeType {
			var err error
			b.child, err = newMapPlanForType(childType, seen, depth+1)
			if err != nil {
				return nil, fmt.Errorf("field %s: %w", f.Name, err)
			}
		}
		p.fields = append(p.fields, b)
	}
	if len(p.fields) == 0 {
		return nil, fmt.Errorf("record type %v has no mapped fields", t)
	}
	if p.markerIndex >= 0 {
		for i := range p.fields {
			f, ok := markerType.FieldByName(t.Field(p.fields[i].index).Name)
			if !ok {
				continue
			}
			if f.Type.Kind() != reflect.Bool || len(f.Index) != 1 {
				return nil, fmt.Errorf("marker lacks bool field %s", t.Field(p.fields[i].index).Name)
			}
			p.fields[i].markerIndex = f.Index[0]
		}
	}
	return p, nil
}

// Load converts maps into a real typed slice. dest must point to a slice or
// to an interface; *any receives a generated []*T when no type is supplied.
func (r Maps) Load(dest any, options ...MapOption) error {
	var o mapOptions
	for _, apply := range options {
		apply(&o)
	}
	if o.plan != nil && len(o.fields) > 0 {
		return fmt.Errorf("WithMapPlan and WithMapFields cannot be combined")
	}
	if o.plan != nil {
		return o.plan.Load(r, dest)
	}
	d := reflect.ValueOf(dest)
	if !d.IsValid() || d.Kind() != reflect.Ptr || d.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	var p *MapPlan
	var err error
	if d.Elem().Kind() == reflect.Slice {
		if len(o.fields) > 0 {
			return fmt.Errorf("WithMapFields requires an interface destination")
		}
		p, err = NewMapPlanForType(d.Elem().Type())
	} else if d.Elem().Kind() == reflect.Interface {
		if len(o.fields) > 0 {
			p, err = NewMapPlan(o.fields)
		} else {
			p, err = InferMapPlan(r)
		}
	} else {
		return fmt.Errorf("destination must point to a slice or interface")
	}
	if err != nil {
		return err
	}
	return p.Load(r, dest)
}

// Load converts a batch using the compiled schema. Unknown map keys are errors.
// Destination mutation is atomic: it is assigned only after all rows succeed.
func (p *MapPlan) Load(rows Maps, dest any) error {
	if p == nil {
		return fmt.Errorf("nil record plan")
	}
	d := reflect.ValueOf(dest)
	if !d.IsValid() || d.Kind() != reflect.Ptr || d.IsNil() {
		return fmt.Errorf("destination must be a non-nil pointer")
	}
	out := reflect.MakeSlice(p.sliceType, 0, len(rows))
	for i, row := range rows {
		v, err := p.loadOne(row)
		if err != nil {
			return fmt.Errorf("row %d: %w", i, err)
		}
		out = reflect.Append(out, v)
	}
	target := d.Elem()
	if target.Kind() == reflect.Interface {
		target.Set(out)
		return nil
	}
	if target.Kind() != reflect.Slice {
		return fmt.Errorf("destination must point to a slice or interface")
	}
	if target.Type() == out.Type() {
		target.Set(out)
		return nil
	}
	if target.Type().Elem() == p.rowType {
		values := reflect.MakeSlice(target.Type(), out.Len(), out.Len())
		for i := 0; i < out.Len(); i++ {
			values.Index(i).Set(out.Index(i).Elem())
		}
		target.Set(values)
		return nil
	}
	return fmt.Errorf("destination slice %v is incompatible with %v", target.Type(), p.sliceType)
}

func (p *MapPlan) loadOne(row map[string]any) (reflect.Value, error) {
	v := reflect.New(p.rowType)
	var flags reflect.Value
	if p.markerIndex >= 0 {
		flags = v.Elem().Field(p.markerIndex)
		if p.markerPtr {
			flags.Set(reflect.New(flags.Type().Elem()))
			flags = flags.Elem()
		}
	}
	for key, raw := range row {
		index, ok := p.byKey[key]
		if !ok {
			return reflect.Value{}, fmt.Errorf("unknown key %q", key)
		}
		b := p.fields[index]
		converted, err := recordFieldValue(raw, b)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("key %q: %w", key, err)
		}
		v.Elem().Field(b.index).Set(converted)
		if p.markerIndex >= 0 && b.markerIndex >= 0 {
			flags.Field(b.markerIndex).SetBool(true)
		}
	}
	return v, nil
}

func recordFieldValue(raw any, b recordBinding) (reflect.Value, error) {
	if b.child == nil || raw == nil {
		return recordValue(raw, b.typeOf)
	}
	if b.typeOf.Kind() == reflect.Slice {
		v := reflect.ValueOf(raw)
		if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
			return reflect.Value{}, fmt.Errorf("expected a slice of objects, got %T", raw)
		}
		out := reflect.MakeSlice(b.typeOf, 0, v.Len())
		for i := 0; i < v.Len(); i++ {
			item := v.Index(i).Interface()
			if item == nil {
				if b.typeOf.Elem().Kind() != reflect.Ptr {
					return reflect.Value{}, fmt.Errorf("element %d is nil but %v cannot hold nil", i, b.typeOf.Elem())
				}
				out = reflect.Append(out, reflect.Zero(b.typeOf.Elem()))
				continue
			}
			m, err := asStringMap(item)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
			}
			child, err := b.child.loadOne(m)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("element %d: %w", i, err)
			}
			if b.typeOf.Elem().Kind() != reflect.Ptr {
				child = child.Elem()
			}
			out = reflect.Append(out, child)
		}
		return out, nil
	}
	m, err := asStringMap(raw)
	if err != nil {
		return reflect.Value{}, err
	}
	v, err := b.child.loadOne(m)
	if err != nil {
		return reflect.Value{}, err
	}
	if b.typeOf.Kind() != reflect.Ptr {
		return v.Elem(), nil
	}
	return v, nil
}

// Store converts a struct slice into maps. A non-nil Has marker omits fields
// whose flags are false. A nil marker follows Marker semantics: all are set.
func (r *Maps) Store(src any, options ...MapOption) error {
	if r == nil {
		return fmt.Errorf("nil Maps receiver")
	}
	var o mapOptions
	for _, apply := range options {
		apply(&o)
	}
	if len(o.fields) > 0 {
		return fmt.Errorf("WithMapFields applies only to Load")
	}
	p := o.plan
	if p == nil {
		var err error
		p, err = NewMapPlanForType(reflect.TypeOf(src))
		if err != nil {
			return err
		}
	}
	rows, err := p.Store(src)
	if err != nil {
		return err
	}
	*r = rows
	return nil
}

// Store returns new outer and nested maps and copies byte slices.
func (p *MapPlan) Store(src any) (Maps, error) {
	if p == nil {
		return nil, fmt.Errorf("nil record plan")
	}
	v := reflect.ValueOf(src)
	if !v.IsValid() {
		return nil, fmt.Errorf("nil source")
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil, fmt.Errorf("nil source")
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Slice {
		return nil, fmt.Errorf("source must be a struct slice")
	}
	out := make(Maps, v.Len())
	for i := 0; i < v.Len(); i++ {
		row := v.Index(i)
		if row.Kind() == reflect.Ptr {
			if row.IsNil() {
				return nil, fmt.Errorf("row %d is nil", i)
			}
			row = row.Elem()
		}
		if row.Type() != p.rowType {
			return nil, fmt.Errorf("row %d has type %v, expected %v", i, row.Type(), p.rowType)
		}
		m, err := p.storeOne(row)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i, err)
		}
		out[i] = m
	}
	return out, nil
}

func (p *MapPlan) storeOne(row reflect.Value) (map[string]any, error) {
	m := make(map[string]any, len(p.fields))
	var flags reflect.Value
	if p.markerIndex >= 0 {
		flags = row.Field(p.markerIndex)
		if p.markerPtr && !flags.IsNil() {
			flags = flags.Elem()
		}
	}
	for _, b := range p.fields {
		if p.markerIndex >= 0 && b.markerIndex >= 0 && (!p.markerPtr || flags.Kind() == reflect.Struct) && !flags.Field(b.markerIndex).Bool() {
			continue
		}
		field := row.Field(b.index)
		if !field.CanInterface() {
			return nil, fmt.Errorf("key %q is inaccessible", b.key)
		}
		if recordIsNil(field) {
			m[b.key] = nil
			continue
		}
		if b.child != nil {
			if field.Kind() == reflect.Slice {
				items := make([]map[string]any, field.Len())
				for i := 0; i < field.Len(); i++ {
					item := field.Index(i)
					if item.Kind() == reflect.Ptr {
						if item.IsNil() {
							items[i] = nil
							continue
						}
						item = item.Elem()
					}
					nested, err := b.child.storeOne(item)
					if err != nil {
						return nil, fmt.Errorf("key %q element %d: %w", b.key, i, err)
					}
					items[i] = nested
				}
				m[b.key] = items
			} else {
				if field.Kind() == reflect.Ptr {
					field = field.Elem()
				}
				nested, err := b.child.storeOne(field)
				if err != nil {
					return nil, fmt.Errorf("key %q: %w", b.key, err)
				}
				m[b.key] = nested
			}
			continue
		}
		value := field.Interface()
		if field.Kind() == reflect.Ptr {
			value = field.Elem().Interface()
		}
		if content, ok := value.([]byte); ok {
			value = bytes.Clone(content)
		}
		m[b.key] = value
	}
	return m, nil
}

func recordTagName(s string) string { return strings.SplitN(s, ",", 2)[0] }
func recordNilable(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Interface, reflect.Map, reflect.Slice, reflect.Ptr, reflect.Func, reflect.Chan:
		return true
	}
	return false
}
func recordIsNil(v reflect.Value) bool { return recordNilable(v.Type()) && v.IsNil() }
func validRecordName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !unicode.IsUpper(r) {
				return false
			}
		} else if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {
			return false
		}
	}
	return true
}
func recordGoName(key string) string {
	var b strings.Builder
	upper := true
	for _, r := range key {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			upper = true
			continue
		}
		if b.Len() == 0 && unicode.IsDigit(r) {
			b.WriteByte('F')
		}
		if upper {
			b.WriteRune(unicode.ToUpper(r))
		} else {
			b.WriteRune(r)
		}
		upper = false
	}
	if b.Len() == 0 {
		return "Field"
	}
	return b.String()
}

func recordValue(src any, t reflect.Type) (reflect.Value, error) {
	if src == nil {
		if recordNilable(t) {
			return reflect.Zero(t), nil
		}
		return reflect.Value{}, fmt.Errorf("nil cannot be stored in %v", t)
	}
	if t.Kind() == reflect.Ptr {
		if incoming := reflect.ValueOf(src); incoming.Kind() == reflect.Ptr && incoming.Type().AssignableTo(t) {
			if incoming.IsNil() {
				return reflect.Zero(t), nil
			}
			src = incoming.Elem().Interface()
		}
		v, e := recordValue(src, t.Elem())
		if e != nil {
			return reflect.Value{}, e
		}
		p := reflect.New(t.Elem())
		p.Elem().Set(v)
		return p, nil
	}
	v := reflect.ValueOf(src)
	if v.Type().AssignableTo(t) {
		return v, nil
	}
	if t.Kind() == reflect.Interface {
		if v.Type().Implements(t) {
			return v, nil
		}
		return reflect.Value{}, fmt.Errorf("%v does not implement %v", v.Type(), t)
	}
	if number, ok := src.(json.Number); ok {
		return recordNumber(string(number), t)
	}
	if isRecordNumeric(v.Kind()) && isRecordNumeric(t.Kind()) {
		return recordNumber(fmt.Sprint(src), t)
	}
	return reflect.Value{}, fmt.Errorf("cannot convert %v to %v", v.Type(), t)
}
func isRecordNumeric(k reflect.Kind) bool { return k >= reflect.Int && k <= reflect.Float64 }
func recordNumber(s string, t reflect.Type) (reflect.Value, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return reflect.Value{}, fmt.Errorf("invalid number %q", s)
	}
	v := reflect.New(t).Elem()
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !r.IsInt() || !r.Num().IsInt64() {
			return v, fmt.Errorf("%q overflows %v", s, t)
		}
		n := r.Num().Int64()
		if v.OverflowInt(n) {
			return v, fmt.Errorf("%q overflows %v", s, t)
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if !r.IsInt() || !r.Num().IsUint64() {
			return v, fmt.Errorf("%q overflows %v", s, t)
		}
		n := r.Num().Uint64()
		if v.OverflowUint(n) {
			return v, fmt.Errorf("%q overflows %v", s, t)
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		precision := uint(53)
		if t.Kind() == reflect.Float32 {
			precision = 24
		}
		f := new(big.Float).SetPrec(precision).SetRat(r)
		n, _ := f.Float64()
		if math.IsInf(n, 0) || v.OverflowFloat(n) {
			return v, fmt.Errorf("%q overflows %v", s, t)
		}
		// Reject rounding, including integer values above exact float precision.
		back := new(big.Rat)
		back.SetFloat64(n)
		if back.Cmp(r) != 0 {
			return v, fmt.Errorf("%q loses precision in %v", s, t)
		}
		v.SetFloat(n)
	default:
		return v, fmt.Errorf("number cannot be stored in %v", t)
	}
	return v, nil
}

// Fields returns a defensive schema copy suitable for selecting a physical
// column projection. Nested descriptors retain recursive presence semantics.
func (p *MapPlan) Fields() []MapField {
	if p == nil {
		return nil
	}
	fields := make([]MapField, len(p.fields))
	for i, b := range p.fields {
		f := MapField{Key: b.key, Name: p.rowType.Field(b.index).Name, Type: b.typeOf, Tag: p.rowType.Field(b.index).Tag}
		if b.child != nil {
			f.Fields = b.child.Fields()
			f.Repeated = b.typeOf.Kind() == reflect.Slice
			f.Type = nil
		}
		fields[i] = f
	}
	return fields
}
