package structology

import (
	"fmt"
	"github.com/viant/xunsafe"
	"reflect"
	"unsafe"
)

type (
	//StateType represents a state type
	StateType struct {
		isPtr     bool
		rType     reflect.Type
		selectors Selectors
		marker    *Marker
	}

	//State represents a state
	State struct {
		stateType *StateType
		value     interface{}
		valuePtr  interface{}
		ptr       unsafe.Pointer
	}
)

// Lookup returns a selector
func (s *StateType) Lookup(name string) *Selector {
	return s.selectors.Lookup(name)
}

func (s *StateType) RootSelectors() []*Selector {
	return s.selectors.Root
}

// MatchByTag matches selector by tag name
func (s *StateType) MatchByTag(tagName string) []*Selector {
	var result = make([]*Selector, 0)
	s.selectors.Each(func(key string, selector *Selector) {
		tag := selector.leaf().field.Tag
		if tag == "" {
			return
		}
		if _, ok := tag.Lookup(tagName); ok {
			result = append(result, selector)
		}
	})
	return result
}

// Type returns state underlying reflect type
func (s *StateType) Type() reflect.Type {
	return s.rType
}

func (s *StateType) IsDefined() bool {
	return len(s.selectors.Items) > 0
}

// Type returns state type
func (s *State) Type() *StateType {
	return s.stateType
}

func (s *State) HasMarker() bool {
	return s.stateType.HasMarker()
}

func (s *State) MarkerHolder() interface{} {
	if !s.HasMarker() {
		return nil
	}
	return s.stateType.marker.holder.Value(s.ptr)
}

func (s *State) EnsureMarker() {
	if !s.HasMarker() || s.stateType.marker.holder.Kind() == reflect.Struct {
		return
	}
	if s.stateType.marker.holder.Value(s.ptr) == nil {
		v := reflect.New(s.stateType.marker.holder.Type).Elem().Interface()
		s.stateType.marker.holder.SetValue(s.ptr, v)
	}
}

// Marker returns marker
func (s *StateType) Marker() *Marker {
	return s.marker
}

func (s *StateType) HasMarker() bool {
	if s.marker == nil || s.marker.holder == nil {
		return false
	}
	return true
}

// Pointer returns state actual value pointer
func (s *State) Pointer() unsafe.Pointer {
	return s.ptr
}

// State return state value
func (s *State) State() interface{} {
	return s.value
}

// Sync syncs value ptr ot value if out of sync
func (s *State) Sync() {
	if s.valuePtr != nil && s.valuePtr != s.value {
		s.value = reflect.ValueOf(s.valuePtr).Elem().Interface()
		s.ptr = xunsafe.AsPointer(s.value)
	}
}

// StatePtr return state value
func (s *State) StatePtr() interface{} {
	return s.valuePtr
}

// SetValue set state value
func (s *State) SetValue(aPath string, value interface{}, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	return selector.SetValue(s.ptr, value, pathOptions...)
}

// SetPrimitive sets primitive value
func (s *State) SetPrimitive(aPath string, value interface{}, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	return selector.Set(s.ptr, value, pathOptions...)
}

// SetString set string for supplied state path
func (s *State) SetString(aPath string, value string, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	selector.SetString(s.ptr, value, pathOptions...)
	return nil
}

// SetInt set int for supplied path
func (s *State) SetInt(aPath string, value int, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	selector.SetInt(s.ptr, value, pathOptions...)
	return nil
}

// SetBool set bool for supplied state path
func (s *State) SetBool(aPath string, value bool, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	selector.SetBool(s.ptr, value, pathOptions...)
	return nil
}

// SetFloat64 set float for supplied state path
func (s *State) SetFloat64(aPath string, value float64, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	selector.SetFloat64(s.ptr, value, pathOptions...)
	return nil
}

//SetFloat43 set float for supplied state path

func (s *State) SetFloat32(aPath string, value float32, pathOptions ...PathOption) error {
	selector, err := s.Selector(aPath)
	if err != nil {
		return err
	}
	selector.SetFloat32(s.ptr, value, pathOptions...)
	return nil
}

// Value returns a value for supplied path
func (s *State) Value(aPath string, pathOptions ...PathOption) (interface{}, error) {
	selector, err := s.Selector(aPath)
	if err != nil {
		return nil, err
	}
	return selector.Value(s.ptr, pathOptions...), nil
}

// Values returns a values for supplied path
func (s *State) Values(aPath string, pathOptions ...PathOption) ([]interface{}, error) {
	selector, err := s.Selector(aPath)
	if err != nil {
		return nil, err
	}
	return selector.Values(s.ptr, pathOptions...), nil
}

// Bool returns a value for supplied path
func (s *State) Bool(aPath string, pathOptions ...PathOption) (bool, error) {
	selector, err := s.Selector(aPath)
	if err != nil {
		return false, err
	}
	return selector.Bool(s.ptr, pathOptions...), nil
}

// Bool returns a value for supplied path
func (s *State) String(aPath string, pathOptions ...PathOption) (string, error) {
	selector, err := s.Selector(aPath)
	if err != nil {
		return "", err
	}
	return selector.String(s.ptr, pathOptions...), nil
}

func (s *State) Float64(aPath string, pathOptions ...PathOption) (float64, error) {
	selector, err := s.Selector(aPath)
	if err != nil {
		return 0.0, err
	}
	return selector.Float64(s.ptr, pathOptions...), nil
}

// Selector returns a state selector for supplied path
func (s *State) Selector(aPath string) (*Selector, error) {
	index, ok := s.stateType.selectors.Map[aPath]
	if !ok {
		return nil, fmt.Errorf("failed to lookup path %v at %s", aPath, s.stateType.rType.String())
	}
	return s.stateType.selectors.Items[index], nil
}

// NewStateType creates a state type
func NewStateType(rType reflect.Type, opts ...SelectorOption) *StateType {
	ret := &StateType{rType: rType, isPtr: rType.Kind() == reflect.Ptr}
	ret.selectors, ret.marker = NewSelectors(rType, opts...)
	return ret
}

// WithValue creates a state with value
func (t *StateType) WithValue(value interface{}) *State {
	//TODO assignable assertion
	return &State{stateType: t, value: value, ptr: xunsafe.AsPointer(value)}
}

// NewState creates a state
func (t *StateType) NewState() *State {
	var valuePtr reflect.Value
	if t.isPtr {
		valuePtr = reflect.New(t.rType.Elem())
	} else {
		valuePtr = reflect.New(t.rType)
	}
	if t.rType.Kind() == reflect.Slice {
		sliceValue := reflect.MakeSlice(t.rType, 0, 1)
		valuePtr.Elem().Set(sliceValue)
	}

	ret := &State{stateType: t}
	if t.isPtr {
		ret.value = valuePtr.Interface()
		ret.valuePtr = ret.value
	} else {
		ret.valuePtr = valuePtr.Interface()
		ret.value = valuePtr.Elem().Interface()
	}
	ret.ptr = xunsafe.AsPointer(ret.value)
	return ret
}
