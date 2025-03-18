package visitor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewMapVisitor(t *testing.T) {
	var aMap = map[string]bool{
		"abc": true,
		"def": true}

	{
		cloned := make(map[string]bool)
		visit := MapVisitorOf[string, bool](aMap)
		visit(func(key string, element bool) (bool, error) {
			cloned[key] = element
			return true, nil
		})
		assert.EqualValues(t, aMap, cloned)
	}
	{
		visit, err := AnyMapVisitorOf(aMap)
		assert.Nil(t, err)
		cloned := make(map[string]bool)
		visit(func(key any, element any) (bool, error) {
			cloned[key.(string)] = element.(bool)
			return true, nil
		})
		assert.EqualValues(t, aMap, cloned)
	}
	{
		fMap := map[float64]float64{
			1: 1,
		}
		visit, err := AnyMapVisitorOf(fMap)
		assert.Nil(t, err)
		cloned := make(map[float64]float64)
		visit(func(key any, element any) (bool, error) {
			cloned[key.(float64)] = element.(float64)
			return true, nil
		})
		assert.EqualValues(t, fMap, cloned)
	}

}
