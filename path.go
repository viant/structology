package structology

import (
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"unsafe"
)

type (
	path struct {
		field     *xunsafe.Field
		kind      reflect.Kind
		slice     *xunsafe.Slice
		marker    *Marker
		isPtr     bool
		converter *converter
	}

	paths []*path

	pathOptions struct {
		indexes       []int
		indexPos      int
		withMarkerSet bool
		err           error
		//keys    []string once we add map support
	}

	PathOption func(o *pathOptions)
)

func (p paths) useSlice() bool {
	for _, aPath := range p {
		if aPath.slice != nil {
			return true
		}
	}
	return false
}

func (o *pathOptions) index() int {
	if o == nil {
		return 0
	}
	if o.hasIndex() {
		return o.nextIndex()
	}
	return 0
}

func (o *pathOptions) hasIndex() bool {
	if o == nil {
		return false
	}
	return o.indexPos < len(o.indexes)
}

func (o *pathOptions) nextIndex() int {
	ret := o.indexes[o.indexPos]
	o.indexPos++
	return ret
}

func (o *pathOptions) shallSetMarker() bool {
	if o == nil {
		return false
	}
	return o.withMarkerSet
}

func newPathOptions(opts []PathOption) *pathOptions {
	var result = &pathOptions{}
	for _, opt := range opts {
		opt(result)
	}
	return result
}

func (p *path) setSliceItem(holderPtr unsafe.Pointer, value interface{}, options *pathOptions) error {
	if p.field != nil {
		holderPtr = p.field.Pointer(holderPtr)
	}
	idx := options.index()
	length := p.slice.Len(holderPtr)
	if idx < 0 || idx >= length {
		if options != nil {
			options.err = fmt.Errorf("index out of range: %v, len: %v", idx, length)
		}
		return fmt.Errorf("index out of range: %v, len: %v", idx, length)
	}
	p.slice.SetValueAt(holderPtr, idx, value)
	return nil
}

func (p paths) upstream(ptr unsafe.Pointer, options *pathOptions) (unsafe.Pointer, *path) {
	count := len(p)
	if count == 1 {
		return ptr, p[0]
	}
	for i := 0; i < count-1; i++ {
		ptr = p[i].pointer(ptr, options)
		if options != nil && options.err != nil {
			break
		}
	}
	leaf := p[count-1]
	return ptr, leaf
}

func (p *path) setMarker(ptr unsafe.Pointer) error {
	if !p.ensureMarker(ptr) {
		return nil
	}
	return p.marker.Set(ptr, int(p.field.Index), true)
}

func (p *path) ensureMarker(ptr unsafe.Pointer) bool {
	if p.marker == nil {
		return false
	}
	p.marker.EnsureHolder(ptr)
	return true
}

func (p *path) pointer(parent unsafe.Pointer, options *pathOptions) unsafe.Pointer {
	ptr := parent
	if xField := p.field; xField != nil {
		ptr = xField.Pointer(ptr)
	}
	if p.slice != nil {
		ptr = p.item(ptr, options)
		if options != nil && options.err != nil {
			return parent
		}
	}
	if p.isPtr {
		ptr = xunsafe.DerefPointer(ptr)
	}
	if options.shallSetMarker() {
		_ = p.setMarker(parent)
	}
	return ptr
}

func (p *path) item(ptr unsafe.Pointer, options *pathOptions) unsafe.Pointer {
	sliceLen := p.slice.Len(ptr)
	index := options.index()
	if index < 0 || index >= sliceLen {
		if options != nil {
			options.err = fmt.Errorf("index out of range: %v, len: %v", index, sliceLen)
		}
		return ptr
	}
	return p.slice.PointerAt(ptr, uintptr(index))
}

func WithPathIndex(indexes ...int) PathOption {
	return func(o *pathOptions) {
		o.indexes = indexes
	}
}
