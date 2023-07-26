package structology

import (
	"fmt"
	"github.com/viant/xunsafe"
	"unsafe"
)

type (
	path struct {
		field  *xunsafe.Field
		slice  *xunsafe.Slice
		marker *Marker
		isPtr  bool
	}

	paths []*path

	pathOptions struct {
		indexes       []int
		indexPos      int
		withMarkerSet bool
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

func (p *path) setSliceItem(holderPtr unsafe.Pointer, value interface{}, options *pathOptions) {
	if p.field != nil {
		holderPtr = p.field.Pointer(holderPtr)
	}
	idx := options.index()
	p.slice.SetValueAt(holderPtr, idx, value)
}

func (p paths) upstream(ptr unsafe.Pointer, options *pathOptions) (unsafe.Pointer, *path) {
	count := len(p)
	if count == 1 {
		return ptr, p[0]
	}
	for i := 0; i < count-1; i++ {
		ptr = p[i].pointer(ptr, options)
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
	if index >= sliceLen {
		panic(fmt.Sprintf("IndexOutOfRange: %v, len: %v", index, sliceLen))
	}
	return p.slice.PointerAt(ptr, uintptr(index))
}

func WithPathIndex(indexes ...int) PathOption {
	return func(o *pathOptions) {
		o.indexes = indexes
	}
}
