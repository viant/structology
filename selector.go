package structology

import (
	"github.com/viant/xunsafe"
	"reflect"
	"strings"
	"unsafe"
)

type (
	//Selector represents struct path selector
	Selector struct {
		paths paths
		Selectors
		useSlice bool
	}

	//Selectors indexed selectors
	Selectors struct {
		Map   map[string]int
		Items []*Selector
		Root  []*Selector
	}

	selectorOptions struct {
		markerOption Option
		getNames     func(name string, tag reflect.StructTag) []string
	}

	//SelectorOption represents selector option
	SelectorOption func(o *selectorOptions)
)

func (o *selectorOptions) apply(opts []SelectorOption) {
	for _, opt := range opts {
		opt(o)
	}
	if o.getNames == nil {
		o.getNames = func(name string, tag reflect.StructTag) []string {
			return []string{name}
		}
	}
}

func (s Selectors) Lookup(name string) *Selector {
	index, ok := s.Map[name]
	if !ok {
		return nil
	}
	return s.Items[index]
}

func (s *Selectors) Add(key string, selector *Selector) {
	index := len(s.Items)
	s.Map[key] = index
	s.Items = append(s.Items, selector)
	if strings.Index(key, ".") == -1 {
		s.Root = append(s.Root, selector)
	}
}

func (s *Selectors) Each(cb func(key string, selector *Selector)) {
	for k, index := range s.Map {
		cb(k, s.Items[index])
	}
}

func selectors() Selectors {
	return Selectors{Map: make(map[string]int)}
}

// Type returns selector result type
func (s *Selector) Type() reflect.Type {
	leaf := s.leaf()
	if leaf.field != nil {
		return leaf.field.Type
	}
	return leaf.slice.Type
}

func (s *Selector) IsAnonymous() bool {
	leaf := s.leaf()
	return leaf.field.Anonymous
}

func (s *Selector) Path() string {
	var result = []string{}
	for _, aPath := range s.paths {
		result = append(result, aPath.field.Name)
	}
	return strings.Join(result, ".")
}

func (s *Selector) leaf() *path {
	leaf := s.paths[len(s.paths)-1]
	return leaf
}

// Value returns selector value
func (s *Selector) Value(ptr unsafe.Pointer, opts ...PathOption) interface{} {
	var options *pathOptions
	if len(opts) > 0 {
		options = newPathOptions(opts)
	}
	holderPtr, leafField := s.paths.upstream(ptr, options)
	if leafField.slice != nil && options.hasIndex() {
		ptr = leafField.field.Pointer(holderPtr)
		return leafField.slice.ValueAt(ptr, options.index())
	}
	return leafField.field.Value(holderPtr)
}

// Value returns selector value
func (s *Selector) Values(ptr unsafe.Pointer, opts ...PathOption) []interface{} {
	value := s.Value(ptr, opts...)
	if value == nil {
		return nil
	}
	switch actual := value.(type) {
	case []interface{}:
		return actual
	default:
		valueType := reflect.TypeOf(value)
		if valueType.Kind() == reflect.Slice {
			return s.asValues(value, valueType)
		}
		return []interface{}{value}
	}
}

func (s *Selector) asValues(value interface{}, valueType reflect.Type) []interface{} {
	valuePtr := xunsafe.AsPointer(value)
	xSlice := xunsafe.NewSlice(valueType)
	sliceLen := xSlice.Len(valuePtr)
	ret := make([]interface{}, sliceLen)
	for i := 0; i < sliceLen; i++ {
		ret[i] = xSlice.ValueAt(valuePtr, i)
	}
	return ret
}

func (s *Selector) Bool(ptr unsafe.Pointer, opts ...PathOption) bool {
	if !s.useSlice {
		holderPtr, leafField := s.paths.upstream(ptr, nil)
		return leafField.field.Bool(holderPtr)
	}
	return s.asBoolValue(ptr, opts)
}

func (s *Selector) asBoolValue(ptr unsafe.Pointer, opts []PathOption) bool {
	value := s.Value(ptr, opts...)
	return value.(bool) //panic is value is not boolean
}

func (s *Selector) Int(ptr unsafe.Pointer, opts ...PathOption) int {
	if !s.useSlice {
		holderPtr, leafField := s.paths.upstream(ptr, nil)
		return leafField.field.Int(holderPtr)
	}
	return s.asIntValue(ptr, opts)
}

func (s *Selector) asIntValue(ptr unsafe.Pointer, opts []PathOption) int {
	value := s.Value(ptr, opts...)
	return value.(int) //panic is value is not boolean
}

func (s *Selector) Float64(ptr unsafe.Pointer, opts ...PathOption) float64 {
	if !s.useSlice {
		holderPtr, leafField := s.paths.upstream(ptr, nil)
		return leafField.field.Float64(holderPtr)
	}
	return s.asFloat64(ptr, opts)
}

func (s *Selector) asFloat64(ptr unsafe.Pointer, opts []PathOption) float64 {
	value := s.Value(ptr, opts...)
	return value.(float64) //panic is value is not boolean
}

func (s *Selector) Float32(ptr unsafe.Pointer, opts ...PathOption) float32 {
	if !s.useSlice {
		holderPtr, leafField := s.paths.upstream(ptr, nil)
		return leafField.field.Float32(holderPtr)
	}
	return s.asFloat32(ptr, opts)
}

func (s *Selector) asFloat32(ptr unsafe.Pointer, opts []PathOption) float32 {
	value := s.Value(ptr, opts...)
	return value.(float32) //panic is value is not boolean
}

func (s *Selector) String(ptr unsafe.Pointer, opts ...PathOption) string {
	if !s.useSlice {
		holderPtr, leafField := s.paths.upstream(ptr, nil)
		return leafField.field.String(holderPtr)
	}
	return s.asStringValue(ptr, opts)
}

func (s *Selector) asStringValue(ptr unsafe.Pointer, opts []PathOption) string {
	value := s.Value(ptr, opts...)
	return value.(string) //panic is value is not boolean
}

func (s *Selector) Has(ptr unsafe.Pointer, opts ...PathOption) bool {
	_, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.marker == nil {
		return true
	}
	return aPath.marker.IsSet(holderPtr, int(aPath.field.Index))
}

// SetValue sets selector value
func (s *Selector) SetValue(ptr unsafe.Pointer, value interface{}, opts ...PathOption) (err error) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return nil
	}
	_ = aPath.setMarker(holderPtr)

	srcType := reflect.TypeOf(value)
	conv := aPath.converter
	if conv != nil && conv.inputType == srcType {
		return conv.setter(value, aPath.field, holderPtr)
	}
	if srcType == aPath.field.Type {
		aPath.field.SetValue(holderPtr, value)
		return nil
	}
	conv = &converter{inputType: srcType, setter: LookupSetter(srcType, aPath.field.Type)}
	aPath.converter = conv
	return conv.setter(value, aPath.field, holderPtr)
}

// Set sets selector value
func (s *Selector) Set(ptr unsafe.Pointer, value interface{}, opts ...PathOption) error {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return nil
	}
	_ = aPath.setMarker(holderPtr)

	srcType := reflect.TypeOf(value)
	conv := aPath.converter
	if conv != nil && conv.inputType == srcType {
		return conv.setter(value, aPath.field, holderPtr)
	}
	if srcType == aPath.field.Type {
		aPath.field.Set(holderPtr, value)
		return nil
	}
	conv = &converter{inputType: srcType, setter: LookupSetter(srcType, aPath.field.Type)}
	aPath.converter = conv
	return conv.setter(value, aPath.field, holderPtr)
}

// SetInt sets selector int value
func (s *Selector) SetInt(ptr unsafe.Pointer, value int, opts ...PathOption) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return
	}
	_ = aPath.setMarker(holderPtr)
	aPath.field.SetInt(holderPtr, value)
}

// SetBool sets selector bool value
func (s *Selector) SetBool(ptr unsafe.Pointer, value bool, opts ...PathOption) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return
	}
	_ = aPath.setMarker(holderPtr)
	aPath.field.SetBool(holderPtr, value)
}

// SetFloat64 sets selector float64 value
func (s *Selector) SetFloat64(ptr unsafe.Pointer, value float64, opts ...PathOption) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return
	}
	_ = aPath.setMarker(holderPtr)
	aPath.field.SetFloat64(holderPtr, value)
}

// SetFloat32 sets selector float32 value
func (s *Selector) SetFloat32(ptr unsafe.Pointer, value float32, opts ...PathOption) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return
	}
	_ = aPath.setMarker(holderPtr)
	aPath.field.SetFloat32(holderPtr, value)
}

// SetString sets selector string value
func (s *Selector) SetString(ptr unsafe.Pointer, value string, opts ...PathOption) {
	options, holderPtr, aPath := s.upstreamWithMarker(ptr, opts)
	if aPath.slice != nil && options.hasIndex() {
		aPath.setSliceItem(holderPtr, value, options)
		return
	}
	_ = aPath.setMarker(holderPtr)
	aPath.field.SetString(holderPtr, value)
}

var withMarkerPathOption = &pathOptions{withMarkerSet: true}

func (s *Selector) upstreamWithMarker(ptr unsafe.Pointer, opts []PathOption) (*pathOptions, unsafe.Pointer, *path) {
	options := withMarkerPathOption
	if len(opts) > 0 {
		options = newPathOptions(opts)
		options.withMarkerSet = true
	}
	holderPtr, aPath := s.paths.upstream(ptr, options)
	return options, holderPtr, aPath
}

// NewSelectors creates a selectors for supplied owner types
func NewSelectors(owner reflect.Type, opts ...SelectorOption) (Selectors, *Marker) {
	options := &selectorOptions{}
	options.apply(opts)
	return newSelectors(owner, nil, options)
}

func newSelectors(owner reflect.Type, ancestors paths, options *selectorOptions) (Selectors, *Marker) {
	aStruct := ensureStruct(owner)
	xStruct := xunsafe.NewStruct(aStruct)
	var marker *Marker
	if HasSetMarker(aStruct) {
		marker, _ = NewMarker(owner)
	}
	var result = selectors()
	for i, field := range xStruct.Fields {

		fieldPath := &path{field: &xStruct.Fields[i], kind: field.Kind(), isPtr: field.Kind() == reflect.Ptr, marker: marker}
		selector := &Selector{paths: append(ancestors, fieldPath)}
		if sliceType := ensureSlice(field.Type); sliceType != nil {
			fieldPath.slice = xunsafe.NewSlice(sliceType)
			if sliceType.Elem().Kind() == reflect.Ptr {
				fieldPath.isPtr = true
			}
		}
		if structType := ensureStruct(field.Type); structType != nil && !isTimeType(structType) && owner != structType {
			if structType == ensureStruct(owner) {
				continue
			}
			selector.Selectors, _ = newSelectors(field.Type, selector.paths, options)
		}
		for _, key := range options.getNames(field.Name, field.Tag) {
			result.Add(key, selector)
			for subKey, index := range selector.Selectors.Map {
				result.Add(key+"."+subKey, selector.Selectors.Items[index])
			}
		}
		selector.useSlice = selector.paths.useSlice()
	}
	return result, marker
}

// Name returns selector leaf name
func (s *Selector) Name() string {
	leaf := s.leaf()
	if leaf == nil || leaf.field == nil {
		return ""
	}
	return leaf.field.Name
}

// Tag returns selector leaf struct tag
func (s *Selector) Tag() reflect.StructTag {
	leaf := s.leaf()
	if leaf == nil || leaf.field == nil {
		return ""
	}
	return leaf.field.Tag
}

// WithCustomizedNames returns selector option with customized names use by selector indexer
func WithCustomizedNames(fn func(name string, tag reflect.StructTag) []string) SelectorOption {
	return func(o *selectorOptions) {
		o.getNames = fn
	}
}

// WithMarkerOption returns selector option with marker option
func WithMarkerOption(opt Option) SelectorOption {
	return func(o *selectorOptions) {
		o.markerOption = opt
	}
}
