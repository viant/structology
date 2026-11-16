package marshal

import (
	"context"
	"encoding/json"
	"fmt"
	xshape "github.com/viant/x/shape"
	"reflect"
)

// traversalState belongs to one encoding operation, never to a cached engine.
// Identity includes slice length so overlapping slice views remain valid.
type traversalState struct {
	active map[standardPointer]bool
	depth  int
}

func pointerIdentity(v reflect.Value) (standardPointer, bool) {
	if !v.IsValid() {
		return standardPointer{}, false
	}
	key := standardPointer{}
	switch v.Kind() {
	case reflect.Pointer, reflect.Slice:
		if v.IsNil() {
			return standardPointer{}, false
		}
		key.ptr = v.Pointer()
		if v.Kind() == reflect.Slice {
			key.length = v.Len()
		}
	case reflect.Map:
		if v.IsNil() {
			return standardPointer{}, false
		}
		key.ptr = uintptr(v.UnsafePointer())
	default:
		return standardPointer{}, false
	}
	key.typ = v.Type()
	return key, true
}

func (s *traversalState) enter(v reflect.Value, depth int) error {
	if err := checkDepth(depth); err != nil {
		return err
	}
	if key, ok := pointerIdentity(v); ok {
		if s.active[key] {
			return &json.UnsupportedValueError{Value: v, Str: "encountered a cycle via " + v.Type().String()}
		}
		if s.active == nil {
			s.active = make(map[standardPointer]bool)
		}
		s.active[key] = true
	}
	return nil
}
func (s *traversalState) leave(v reflect.Value) {
	if key, ok := pointerIdentity(v); ok {
		delete(s.active, key)
	}
}
func contextError(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}
func (s *encoderSession) checkContext() error { return contextError(s.ctx) }
func (s *encoderSession) enter(v reflect.Value) error {
	if err := s.checkContext(); err != nil {
		return err
	}
	if err := s.traversal.enter(v, s.traversal.depth); err != nil {
		return err
	}
	s.traversal.depth++
	return nil
}
func (s *encoderSession) leave(v reflect.Value) { s.traversal.depth--; s.traversal.leave(v) }

// appendInlineFields shares the same traversal state while existing field owners
// retain promotion, omission, and wire rendering authority.
func (e *Engine) appendInlineFields(s *encoderSession, v reflect.Value, count *int, representation *xshape.Representation, dynamic bool) error {
	if err := s.enter(v); err != nil {
		return err
	}
	defer s.leave(v)
	if v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil
		}
		if representation != nil && representation.Original() == v.Type() {
			representation = representation.Elem()
		}
		return e.appendInlineFields(s, v.Elem(), count, representation, dynamic)
	}
	switch v.Kind() {
	case reflect.Struct:
		if dynamic {
			v, ptr := addressableStruct(v)
			return e.appendStructFieldsDynamic(s, v, e.getDynamicPlan(v.Type(), representation), ptr, count)
		}
		return e.appendStructFieldsStatic(s, v, count, representation)
	case reflect.Map:
		return e.appendMapFields(s, v, count, representation)
	}
	return nil
}

func checkDepth(depth int) error {
	if depth > 10000 {
		return fmt.Errorf("JSON value exceeds maximum nesting depth")
	}
	return nil
}
