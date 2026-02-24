package visitor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewSliceVisitor(t *testing.T) {
	mySlice := []interface{}{"a", 1, 3.14, true}

	visit, err := SliceVisitorOf[any](mySlice)
	if err != nil {
		panic(err)
	}
	clone := []interface{}{}
	err = visit(func(index int, element interface{}) (bool, error) {
		clone = append(clone, element)
		return true, nil // continue iteration
	})
	assert.NoError(t, err)
	assert.EqualValues(t, mySlice, clone)

}
