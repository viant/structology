package structology

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestNewSelector(t *testing.T) {

	var testCases = []struct {
		description string
		pathOptions func() []PathOption
		new         func() interface{}
		selector    string
		prev        interface{}
		value       interface{}
		checkMarker []string
	}{

		{
			description: "typed slice item selector",
			pathOptions: func() []PathOption {
				return []PathOption{WithPathIndex(1)}
			},
			new: func() interface{} {
				type Dummy struct {
					Id  int
					Any []int
				}
				return &Dummy{Id: 1, Any: []int{1, 2, 3}}
			},
			prev:     2,
			selector: "Any",
			value:    44,
		},
		{
			description: "interface item selector",
			pathOptions: func() []PathOption {
				return []PathOption{WithPathIndex(1)}
			},
			new: func() interface{} {
				type Dummy struct {
					Id  int
					Any []interface{}
				}
				return &Dummy{Id: 1, Any: []interface{}{1, 2, 3}}
			},
			prev:     2,
			selector: "Any",
			value:    "b",
		},

		{
			description: "slice selector",
			pathOptions: func() []PathOption {
				return []PathOption{WithPathIndex(1)}
			},
			new: func() interface{} {
				type Foo struct {
					Id   int
					Name string
				}
				type Dummy struct {
					Id   int
					Foos []*Foo
				}
				return &Dummy{Id: 1, Foos: []*Foo{
					{Id: 1, Name: "Name 1"},
					{Id: 2, Name: "Name 2"},
				}}
			},
			prev:     "Name 2",
			selector: "Foos.Name",
			value:    "abc",
		},
		{
			description: "basic selector with marker",
			new: func() interface{} {
				type FooHas struct {
					Id   bool
					Name bool
				}

				type Foo struct {
					Id   int
					Name string
					Has  *FooHas `setMarker:"true"`
				}
				return &Foo{Id: 1, Name: "text"}
			},
			prev:        "text",
			selector:    "Name",
			value:       "abc",
			checkMarker: []string{"Has.Name"},
		},

		{
			description: "multi selector with marker",
			new: func() interface{} {
				type FooHas struct {
					Id   bool
					Name bool
				}
				type DummyHas struct {
					Id  bool
					Foo bool
				}

				type Foo struct {
					Id   int
					Name string
					Has  *FooHas `setMarker:"true"`
				}

				type Dummy struct {
					Id  int
					Foo *Foo
					Has *DummyHas `setMarker:"true"`
				}

				return &Dummy{Foo: &Foo{Id: 1, Name: "text"}}
			},
			prev:        "text",
			selector:    "Foo.Name",
			value:       "abc",
			checkMarker: []string{"Has.Foo", "Foo.Has.Name"},
		},

		{
			description: "double nested ptr selector",
			new: func() interface{} {
				type Foo struct {
					Id   int
					Name string
				}
				type Bar struct {
					Id  int
					Foo *Foo
				}
				type Dummy struct {
					Dummy *Dummy
					Bar   *Bar
				}
				return &Dummy{Bar: &Bar{Id: 20, Foo: &Foo{Id: 1, Name: "text"}}}
			},
			prev:     "text",
			selector: "Bar.Foo.Name",
			value:    "abc",
		},
		{
			description: "nested ptr selector",
			new: func() interface{} {
				type Foo struct {
					Id   int
					Name string
				}
				type Bar struct {
					Id  int
					Foo *Foo
				}
				return &Bar{Id: 20, Foo: &Foo{Id: 1, Name: "text"}}
			},
			prev:     "text",
			selector: "Foo.Name",
			value:    "abc",
		},
		{
			description: "nested  selector",
			new: func() interface{} {
				type Foo struct {
					Id   int
					Name string
				}
				type Bar struct {
					Id  int
					Foo Foo
				}
				return &Bar{Id: 20, Foo: Foo{Id: 1, Name: "text"}}
			},
			prev:     "text",
			selector: "Foo.Name",
			value:    "abc",
		},
		{
			description: "basic selector",
			new: func() interface{} {
				type Foo struct {
					Id   int
					Name string
				}
				return &Foo{Id: 1, Name: "text"}
			},
			prev:     "text",
			selector: "Name",
			value:    "abc",
		},
		{
			description: "interface values selector",
			new: func() interface{} {
				type Dummy struct {
					Id  int
					Any []interface{}
				}
				return &Dummy{Id: 1, Any: []interface{}{1, 2, 3}}
			},
			prev:     []interface{}{1, 2, 3},
			selector: "Any",
			value:    []interface{}{"a", "b"},
		},
	}

	for _, testCase := range testCases {
		value := testCase.new()
		valueType := reflect.TypeOf(value)
		stateType := NewStateType(valueType)
		state := stateType.WithValue(value)
		var options []PathOption
		if testCase.pathOptions != nil {
			options = testCase.pathOptions()
		}

		prev, err := state.Value(testCase.selector, options...)
		if !assert.Nil(t, err, testCase.description) {
			continue
		}
		assert.EqualValues(t, testCase.prev, prev)
		if testCase.pathOptions != nil {
			options = testCase.pathOptions()
		}
		_ = state.SetValue(testCase.selector, testCase.value, options...)
		if testCase.pathOptions != nil {
			options = testCase.pathOptions()
		}
		actual, _ := state.Value(testCase.selector, options...)
		assert.EqualValues(t, testCase.value, actual)
		for _, setMarker := range testCase.checkMarker {
			markerValue, err := state.Value(setMarker)

			if !assert.Nil(t, err, testCase.description) {
				continue
			}
			assert.EqualValues(t, true, markerValue, testCase.description)

		}
	}

}

func TestSetter(t *testing.T) {

	var testCases = []struct {
		description string
		new         func() interface{}
		selector    string
		expect      interface{}
		value       interface{}
	}{
		{
			description: "repeated int",
			selector:    "Values",
			new: func() interface{} {
				type FooHas struct {
					Id     bool
					Values bool
				}
				type Foo struct {
					Id     int
					Values []int
					//HAs    *FooHas `setMarker:"true"`
				}
				return &Foo{}
			},
			value:  "1,2,3",
			expect: []int{1, 2, 3},
		},
		{
			description: "repeated int",
			selector:    "Values",
			new: func() interface{} {
				type BarHas struct {
					Id     bool
					Values bool
				}
				type Bar struct {
					Id     int
					Values []float64
					Has    *BarHas `setMarker:"true"`
				}
				return &Bar{Has: &BarHas{}}
			},
			value:  "1,2,3.1",
			expect: []float64{1.0, 2.0, 3.1},
		},
	}

	for _, testCase := range testCases {

		value := testCase.new()
		valueType := reflect.TypeOf(value)
		stateType := NewStateType(valueType)
		state := stateType.WithValue(value)

		err := state.SetValue(testCase.selector, testCase.value)
		assert.Nil(t, err, testCase.description)
		marker := stateType.Marker()
		if marker != nil {
			assert.True(t, marker.IsFieldSet(state.Pointer(), testCase.selector))
		}
		actual, err := state.Value(testCase.selector)
		assert.Nil(t, err, testCase.description)
		assert.EqualValues(t, testCase.expect, actual, testCase.description)

	}
}
