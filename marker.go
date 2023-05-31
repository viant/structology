package structology

import (
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"strings"
	"unsafe"
)

//Marker field set marker
type Marker struct {
	holder        *xunsafe.Field
	fields        []*xunsafe.Field
	filedPos      map[string]int
	IdentityIndex int
}

//FieldIndex returns mapped field index or -1
func (p *Marker) Index(name string) int {
	if len(p.filedPos) == 0 {
		return -1
	}
	pos, ok := p.filedPos[name]
	if !ok {
		return -1
	}
	return pos
}

//SetAll sets all marker field with supplied flag
func (p *Marker) SetAll(ptr unsafe.Pointer, flag bool) error {
	if !p.CanUseHolder(ptr) {
		return fmt.Errorf("holder was empty")
	}
	markerPtr := p.holder.ValuePointer(ptr)
	for _, field := range p.fields {
		if field == nil {
			continue
		}
		field.SetBool(markerPtr, flag)
	}
	return nil
}

func (p *Marker) CanUseHolder(ptr unsafe.Pointer) bool {
	if p.holder == nil || p.holder.IsNil(ptr) {
		return false
	}
	return true
}

//Set sets field marker
func (p *Marker) Set(ptr unsafe.Pointer, index int, flag bool) error {
	if !p.CanUseHolder(ptr) {
		return fmt.Errorf("holder was empty")
	}

	markerPtr := p.holder.ValuePointer(ptr)
	if index >= len(p.fields) || p.fields[index] == nil {
		return fmt.Errorf("field at index %v was missing in set marker", index)
	}
	p.fields[index].SetBool(markerPtr, flag)
	return nil
}

//IsSet returns true if field has been set
func (p *Marker) IsSet(ptr unsafe.Pointer, index int) bool {
	if p.holder == nil || p.holder.IsNil(ptr) {
		return true //we do not have field presence provider so we assume all fields are set
	}
	if p.holder.IsNil(ptr) {
		return true //holder is nil
	}
	return p.has(ptr, index)
}

//Has checks if filed value was flagged as set
func (p *Marker) has(ptr unsafe.Pointer, index int) bool {
	markerPtr := p.holder.ValuePointer(ptr)
	if index >= len(p.fields) || p.fields[index] == nil {
		return false
	}
	return p.fields[index].Bool(markerPtr)
}

//Init initialises field set marker
func (p *Marker) init() error {
	if p.holder == nil {
		return fmt.Errorf("holder was empty")
	}
	if len(p.filedPos) == 0 {
		return fmt.Errorf("struct has no markable fields")
	}
	if holder := p.holder; holder != nil {
		p.fields = make([]*xunsafe.Field, len(p.filedPos))
		holderType := ensureStruct(holder.Type)
		for i := 0; i < holderType.NumField(); i++ {
			markerField := holderType.Field(i)
			pos, ok := p.filedPos[markerField.Name]
			if !ok {
				return fmt.Errorf("marker filed: '%v' does not have corresponding struct field", markerField.Name)
			}
			p.fields[pos] = xunsafe.NewField(markerField)
		}
	}
	return nil
}

//NewMarker returns new struct field set marker
func NewMarker(t reflect.Type) (*Marker, error) {
	if t = ensureStruct(t); t == nil {
		return nil, fmt.Errorf("supplied type is not struct")
	}
	numFiled := t.NumField()
	var result = &Marker{fields: make([]*xunsafe.Field, numFiled), filedPos: make(map[string]int, numFiled)}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		result.filedPos[field.Name] = field.Index[0]
		if _, ok := field.Tag.Lookup(SetMarkerTag); ok || strings.Contains(string(field.Tag), legacyTagFragment) {
			result.holder = xunsafe.NewField(field)
		}
	}
	return result, result.init()
}
