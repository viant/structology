package csv

import (
	"fmt"
	io "github.com/viant/structology/encoding/csv/internal/scalar"
	"github.com/viant/xunsafe"
	"reflect"
	"strings"
	"unsafe"
)

type UnmarshalSession struct {
	buffer     []string
	nulls      []bool
	objects    []*Object
	pathIndex  map[string]int
	parentNode *Object
	dest       interface{}
	destPtr    unsafe.Pointer
}

func (s *UnmarshalSession) init(fields []*Field, refs map[string][]string, accessors map[string]*xunsafe.Field, stringifiers map[reflect.Type]*io.ObjectStringifier, pathTypes map[string]reflect.Type) error {
	s.destPtr = xunsafe.AsPointer(s.dest)
	s.buffer = make([]string, len(fields))
	s.nulls = make([]bool, len(fields))
	for i, field := range fields {
		object := s.getOrCreateObject(field, refs, accessors, stringifiers)
		object.AddHolder(field, &s.buffer[i], &s.nulls[i])
	}

	// Assembled typed output does not need public scalar fields on every
	// holder. Reuse the mapper's path metadata to retain empty ancestors.
	if s.dest == nil {
		for _, field := range fields {
			path := field.path
			for path != "" {
				if index := strings.LastIndexByte(path, '.'); index >= 0 {
					path = path[:index]
				} else {
					path = ""
				}
				if typ := pathTypes[path]; typ != nil {
					s.getOrCreateObject(&Field{path: path, parentType: typ}, refs, accessors, stringifiers)
				}
			}
		}
	}
	parentNode, ok := s.buildParentNode()
	if !ok {
		return fmt.Errorf("none of the parent fields were specified")
	}

	anObject, ok := parentNode.(*Object)
	if !ok {
		return fmt.Errorf("unexpected node type, wanted %T got %T", anObject, parentNode)
	}

	s.parentNode = anObject
	return nil
}

func (s *UnmarshalSession) getOrCreateObject(field *Field, refs map[string][]string, accessors map[string]*xunsafe.Field, stringifiers map[reflect.Type]*io.ObjectStringifier) *Object {
	index, ok := s.pathIndex[field.path]
	if ok {
		return s.objects[index]
	}

	s.pathIndex[field.path] = len(s.objects)
	dest, appender := s.destWithAppender(field)
	var slice *xunsafe.Slice
	xField := accessors[field.path]
	if xField != nil && xField.Type.Kind() == reflect.Slice {
		slice = xunsafe.NewSlice(xField.Type)
	}

	parentID := field.path
	if parentPathIndex := strings.LastIndexByte(parentID, '.'); parentPathIndex != -1 {
		parentID = field.path[:parentPathIndex]
	} else {
		parentID = ""
	}

	var derefs []*xunsafe.Type
	rType := field.parentType
	for rType.Kind() == reflect.Ptr {
		rType = rType.Elem()
		derefs = append(derefs, xunsafe.NewType(rType))
	}

	object := &Object{
		stringifier: stringifiers[field.parentType],
		objType:     field.parentType,
		path:        field.path,
		parentID:    asInterface(field.path, parentID),
		dest:        dest,
		appender:    appender,
		index: &Index{
			positionInSlice: map[interface{}]int{},
			data:            nil,
			objectIndex:     ObjectIndex{},
			appenders:       map[interface{}]*xunsafe.Appender{},
		},
		xField: xField,
		xSlice: slice,
		holder: field.holder,
	}

	s.objects = append(s.objects, object)
	return object
}

func asInterface(fieldPath string, parentID string) interface{} {
	if fieldPath == "" {
		return nil
	}

	return parentID
}

func (s *UnmarshalSession) addRecord(record []string) error {
	copy(s.buffer, record)
	return s.parentNode.Umarshal()
}

func (s *UnmarshalSession) buildParentNode() (Node, bool) {
	nodes := make([]Node, 0, len(s.objects))
	for i, _ := range s.objects {
		nodes = append(nodes, s.objects[i])
	}

	parents := BuildTree(nodes)
	for _, parent := range parents {
		if parent.ID() == "" {
			return parent, true
		}
	}

	return nil, false
}

func (s *UnmarshalSession) destWithAppender(field *Field) (interface{}, *xunsafe.Appender) {
	// Marshal constructs the field tree without an unmarshal destination.
	if s.dest == nil {
		return nil, nil
	}
	if field.path == "" {
		dest := s.dest
		var appender *xunsafe.Appender
		if dest != nil {
			slice := xunsafe.NewSlice(reflect.TypeOf(dest).Elem())
			appender = slice.Appender(xunsafe.AsPointer(dest))
		}
		return dest, appender
	}

	parentType := field.parentType
	if parentType.Kind() != reflect.Slice {
		parentType = reflect.SliceOf(parentType)
	}

	dest := reflect.New(parentType).Interface()
	return dest, xunsafe.NewSlice(parentType).Appender(xunsafe.AsPointer(dest))
}
