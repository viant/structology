package structology

import (
	"github.com/stretchr/testify/assert"
	"github.com/viant/xunsafe"
	"reflect"
	"testing"
)

func TestMarker_IsSet(t *testing.T) {

	var testCases = []struct {
		description string
		provider    func() interface{}
		expectSet   []string
		expectUnset []string
		expectError bool
	}{
		{
			description: "aligned set marker",
			provider: func() interface{} {
				type EntityHas struct {
					Id     bool
					Name   bool
					Active bool
				}
				type Entity struct {
					Id     int
					Name   string
					Active bool
					Has    *EntityHas `setMarker:"true"`
				}
				return &Entity{Has: &EntityHas{Id: true, Active: true}, Id: 1, Active: true}
			},
			expectSet:   []string{"Id", "Active"},
			expectUnset: []string{"Name"},
		},
		{
			description: "un aligned set marker (more filed in the owner struct)",
			provider: func() interface{} {
				type EntityHas struct {
					Id     bool
					Name   bool
					Active bool
				}
				type Entity struct {
					Id     int
					Name   string
					Active bool
					Nums   []int
					Has    *EntityHas `setMarker:"true"`
				}
				return &Entity{Has: &EntityHas{Name: true}, Name: "abc"}
			},
			expectSet:   []string{"Name"},
			expectUnset: []string{"Id", "Active"},
		},
		{
			description: "un aligned set marker (more filed in the marker struct)",
			provider: func() interface{} {
				type EntityHas struct {
					Id     bool
					Name   bool
					Active bool
					Nums   bool
				}
				type Entity struct {
					Id     int
					Name   string
					Active bool
					Has    *EntityHas `setMarker:"true"`
				}
				return &Entity{Has: &EntityHas{Name: true}, Name: "abc"}
			},
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		value := testCase.provider()
		marker, err := NewMarker(reflect.TypeOf(value))

		if testCase.expectError {
			assert.NotNilf(t, err, testCase.description)
			continue
		}

		if !assert.Nil(t, err, testCase.description) {
			continue
		}
		if len(testCase.expectSet) == 0 {
			testCase.expectSet = []string{}
		}
		valuePtr := xunsafe.AsPointer(value)
		for _, name := range testCase.expectSet {
			index := marker.Index(name)
			if !assert.NotEqual(t, index, -1, testCase.description) {
				continue
			}
			assert.True(t, marker.IsSet(valuePtr, index), name+" failed set test for "+testCase.description)
		}
		if len(testCase.expectUnset) == 0 {
			testCase.expectUnset = []string{}
		}
		for _, name := range testCase.expectUnset {
			index := marker.Index(name)
			if !assert.NotEqual(t, index, -1, testCase.description) {
				continue
			}
			assert.False(t, marker.IsSet(valuePtr, index), name+" failed unset test for "+testCase.description)
		}
	}

}
