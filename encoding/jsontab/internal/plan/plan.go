package plan

import (
	"reflect"
	"sync"
	"time"
	"unsafe"

	"github.com/viant/xunsafe"
)

var timeType = reflect.TypeOf(time.Time{})

type FieldKind int

const (
	FieldScalar FieldKind = iota
	FieldStruct
	FieldSliceStruct
)

type Field struct {
	StructName string
	HeaderName string
	XField     *xunsafe.Field
	Type       reflect.Type
	Kind       FieldKind
	Child      *Type
}

type Type struct {
	Type          reflect.Type
	Headers       []string
	Fields        []*Field
	ScalarFields  []*Field
	Children      []*Field
	HeaderToField map[string]*Field
	Presence      *PresencePlan
}

type PresencePlan struct {
	Holder     *xunsafe.Field
	HolderType reflect.Type
	Flags      map[string]*xunsafe.Field
}

type cacheKey struct {
	rType   reflect.Type
	tagName string
	caseKey string
}

var cache sync.Map // map[cacheKey]*Type

func For(rType reflect.Type, tagName, caseKey string, compileName func(string) string) *Type {
	if rType.Kind() == reflect.Ptr {
		rType = rType.Elem()
	}
	key := cacheKey{rType: rType, tagName: tagName, caseKey: caseKey}
	if v, ok := cache.Load(key); ok {
		return v.(*Type)
	}
	seen := map[reflect.Type]bool{}
	compiled := compile(rType, tagName, compileName, seen)
	cache.Store(key, compiled)
	return compiled
}

func compile(rType reflect.Type, tagName string, compileName func(string) string, seen map[reflect.Type]bool) *Type {
	if rType.Kind() == reflect.Ptr {
		rType = rType.Elem()
	}
	ret := &Type{Type: rType, HeaderToField: map[string]*Field{}}
	if rType.Kind() != reflect.Struct {
		return ret
	}
	if seen[rType] {
		return ret
	}
	seen[rType] = true
	defer delete(seen, rType)

	for i := 0; i < rType.NumField(); i++ {
		sf := rType.Field(i)
		if sf.PkgPath != "" {
			continue
		}
		if sf.Tag.Get("setMarker") == "true" {
			if sf.Type.Kind() == reflect.Struct || (sf.Type.Kind() == reflect.Ptr && sf.Type.Elem().Kind() == reflect.Struct) {
				ret.Presence = &PresencePlan{
					Holder:     xunsafe.NewField(sf),
					HolderType: sf.Type,
					Flags:      map[string]*xunsafe.Field{},
				}
				pt := sf.Type
				if pt.Kind() == reflect.Ptr {
					pt = pt.Elem()
				}
				for j := 0; j < pt.NumField(); j++ {
					mf := pt.Field(j)
					if mf.Type.Kind() == reflect.Bool {
						ret.Presence.Flags[mf.Name] = xunsafe.NewField(mf)
					}
				}
			}
			continue
		}

		header := sf.Tag.Get(tagName)
		explicit := header != ""
		if header == "" {
			header = sf.Name
		}
		if header == "-" {
			continue
		}
		if compileName != nil && !explicit {
			header = compileName(header)
		}

		field := &Field{
			StructName: sf.Name,
			HeaderName: header,
			XField:     xunsafe.NewField(sf),
			Type:       sf.Type,
			Kind:       FieldScalar,
		}

		t := sf.Type
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		switch {
		case t.Kind() == reflect.Struct && t != timeType:
			field.Kind = FieldStruct
			field.Child = compile(t, tagName, compileName, seen)
		case sf.Type.Kind() == reflect.Slice:
			elem := sf.Type.Elem()
			for elem.Kind() == reflect.Ptr {
				elem = elem.Elem()
			}
			if elem.Kind() == reflect.Struct && elem != timeType {
				field.Kind = FieldSliceStruct
				field.Child = compile(elem, tagName, compileName, seen)
			}
		}

		ret.Fields = append(ret.Fields, field)
		ret.Headers = append(ret.Headers, header)
		ret.HeaderToField[header] = field
		if field.Kind == FieldScalar {
			ret.ScalarFields = append(ret.ScalarFields, field)
		} else {
			ret.Children = append(ret.Children, field)
		}
	}
	if ret.Presence != nil {
		for _, f := range ret.Fields {
			_ = f
		}
	}
	return ret
}

func EnsurePresenceHolder(ptr unsafe.Pointer, p *PresencePlan) unsafe.Pointer {
	if p == nil || p.Holder == nil {
		return nil
	}
	holderPtr := p.Holder.Pointer(ptr)
	if p.HolderType.Kind() == reflect.Ptr {
		target := (*unsafe.Pointer)(holderPtr)
		if *target == nil {
			alloc := reflect.New(p.HolderType.Elem())
			*target = unsafe.Pointer(alloc.Pointer())
		}
		return *target
	}
	return holderPtr
}
