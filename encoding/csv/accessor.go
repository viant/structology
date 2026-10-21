package csv

import (
	io2 "github.com/viant/structology/encoding/csv/internal/scalar"
	"github.com/viant/xunsafe"
	"unsafe"
)

type (
	Accessor struct {
		_parent             *Accessor
		cache               map[unsafe.Pointer]*stringified
		emitedFirst         bool
		parentAccessorIndex int
		fields              []*Field // requested fields
		children            []*Accessor
		config              *Config
		path                string

		currSliceIndex int
		slicePtr       unsafe.Pointer
		slice          *xunsafe.Slice

		ptr    unsafe.Pointer // refering to a single object
		field  *xunsafe.Field // used to get value from parent pointer
		holder string
		xType  *xunsafe.Type
	}

	stringified struct {
		values     []string
		wasStrings []bool
	}
)

func (a *Accessor) Reset() {
	a.emitedFirst = false
	a.cache = map[unsafe.Pointer]*stringified{}
	a.currSliceIndex = 0
}

func (a *Accessor) Has() bool {
	if a.emitedFirst {
		return a.prepare()
	}

	a.emitedFirst = true
	return a.ptr != nil
}

func (a *Accessor) prepare() bool {
	accessor, ok := a.next()
	if !ok {
		return false
	}

	// The rightmost advancing slice resets earlier siblings at each owner,
	// producing the typed relation product without treating parent memory as a child.
	for accessor._parent != nil {
		parent := accessor._parent
		for i := 0; i < accessor.parentAccessorIndex; i++ {
			child := parent.children[i]
			if parent.ptr == nil {
				child.Set(nil)
			} else {
				pointer, slice := parent.getChildValue(parent.ptr, child)
				child.slicePtr = slice
				child.Set(pointer)
			}
			child.currSliceIndex = 0
		}
		accessor = parent
	}
	return true
}

func (a *Accessor) Set(pointer unsafe.Pointer) {
	a.ptr = pointer

	for _, child := range a.children {
		if pointer == nil {
			child.Set(nil)
		} else {
			aPtr, slicePtr := a.getChildValue(pointer, child)
			child.slicePtr = slicePtr
			child.Set(aPtr)
		}

		child.currSliceIndex = 0
	}
}

func (a *Accessor) getChildValue(pointer unsafe.Pointer, child *Accessor) (valuePtr unsafe.Pointer, slicePtr unsafe.Pointer) {
	valuePointer := child.field.ValuePointer(pointer)
	if child.slice == nil {
		return valuePointer, nil
	}

	lenSlice := child.slice.Len(valuePointer)
	if lenSlice == 0 {
		return nil, valuePointer
	}

	at := child.slice.ValuePointerAt(valuePointer, 0)
	return xunsafe.AsPointer(at), valuePointer
}

func (a *Accessor) Stringify(writer *writer) {
	result, wasStrings := a.stringifyFields(writer)
	writer.writeObject(result, wasStrings)
	a.appendChildren(writer, a.children)
}

func (a *Accessor) appendChildren(writer *writer, children []*Accessor) {
	for _, child := range children {
		if child.config == nil {
			fields, bools := child.stringifyFields(writer)
			writer.appendObject(fields, bools)
			child.appendChildren(writer, child.children)
		} else {
			values, size := child.values()
			newWriter(child, child.config, writer.buffer, nil, values, size, a.config.FieldSeparator).writeObjects(child.Headers())
			child.Reset()
		}
	}
}

func (a *Accessor) Headers() []string {
	headers := make([]string, 0, len(a.fields))
	for _, field := range a.fields {
		headers = append(headers, field.header)
	}

	for _, child := range a.children {
		if child.config == nil {
			childHeaders := child.Headers()
			headers = append(headers, childHeaders...)
		} else {
			headers = append(headers, child.holder)
		}
	}

	return headers
}

func (a *Accessor) stringifyFields(writer *writer) ([]string, []bool) {
	if value, ok := a.cache[a.ptr]; ok {
		return value.values, value.wasStrings
	}

	if a.ptr == nil {
		strings := make([]string, len(a.fields))
		for i := range strings {
			strings[i] = writer.config.NullValue
		}

		return strings, make([]bool, len(a.fields))
	}

	result := make([]string, len(a.fields))
	wasStrings := make([]bool, len(a.fields))
	for i, field := range a.fields {
		result[i], wasStrings[i] = field.stringifier(a.ptr)
	}

	a.cache[a.ptr] = &stringified{
		values:     result,
		wasStrings: wasStrings,
	}

	return result, wasStrings
}

// next returns true if record was not exhausted and first not ehausted Accessor
func (a *Accessor) next() (*Accessor, bool) {
	for _, child := range a.children {
		if child.config != nil {
			continue
		}

		if accessor, ok := child.next(); ok {
			return accessor, true
		}
	}

	if a.slice != nil && a.slicePtr != nil {
		sliceLen := a.slice.Len(a.slicePtr)
		if a.currSliceIndex < sliceLen-1 {
			a.currSliceIndex++
			value := a.slice.ValuePointerAt(a.slicePtr, a.currSliceIndex)
			a.Set(xunsafe.AsPointer(value))
			return a, true
		}

		a.currSliceIndex = 0
	}

	return nil, false
}

func (a *Accessor) values() (io2.ValueAccessor, int) {
	if a.ptr == nil {
		return func(index int) interface{} {
			return nil
		}, 0
	}

	if a.slice != nil {
		if a.slicePtr == nil {
			return func(index int) interface{} {
				return nil
			}, 0
		}

		return func(index int) interface{} {
			if index >= a.slice.Len(a.slicePtr) {
				return nil
			}

			return a.slice.ValuePointerAt(a.slicePtr, index)
		}, 1
	}

	interfacer := a.Interfacer()
	return func(index int) interface{} {
		return interfacer.Interface(a.ptr)
	}, 1
}

func (a *Accessor) Interfacer() *xunsafe.Type {
	if a.xType == nil {
		a.xType = xunsafe.NewType(a.field.Type)
	}

	return a.xType
}
