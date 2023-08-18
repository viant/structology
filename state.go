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
		rType     reflect.Type
		selectors Selectors
		marker    *Marker
	}

	//State represents a state
	State struct {
		stateType *StateType
		value     interface{}
		ptr       unsafe.Pointer
	}
)

// Lookup returns a selector
func (s *StateType) Lookup(name string) *Selector {
	return s.selectors.Lookup(name)
}

// Type returns state underlying reflect type
func (s *StateType) Type() reflect.Type {
	return s.rType
}

func (s *StateType) IsDefined() bool {
	return len(s.selectors) > 0
}

// Type returns state type
func (s *State) Type() *StateType {
	return s.stateType
}

func (s *State) HasMarker() bool {
	marker := s.stateType.marker
	if marker == nil || marker.holder == nil {
		return false
	}
	return true
}

func (s *State) MarkerHolder() interface{} {
	marker := s.stateType.marker
	if marker == nil || marker.holder == nil {
		return nil
	}
	return marker.holder.Value(s.ptr)
}

// Marker returns marker
func (s *StateType) Marker() *Marker {
	return s.marker
}

// Pointer returns state actual value pointer
func (s *State) Pointer() unsafe.Pointer {
	return s.ptr
}

// State return state value
func (s *State) State() interface{} {
	return s.value
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
	selector, ok := s.stateType.selectors[aPath]
	if !ok {
		return nil, fmt.Errorf("failed to lookup path %v at %s", aPath, s.stateType.rType.String())
	}
	return selector, nil
}

// NewStateType creates a state type
func NewStateType(rType reflect.Type, opts ...SelectorOption) *StateType {
	rType = ensureStruct(rType)
	ret := &StateType{rType: rType}
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
	value := reflect.New(t.rType).Elem().Interface()
	return &State{stateType: t, value: value, ptr: xunsafe.AsPointer(value)}
}
