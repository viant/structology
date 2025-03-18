package visitor

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_StructVisitor_Visit(t *testing.T) {

	type Employee struct {
		ID      int
		Name    string
		Company string
	}

	emp := &Employee{ID: 1, Name: "John Doe", Company: "OpenAI"}

	visit, err := StructVisitorOf(emp)
	if !assert.Nil(t, err) {
		return
	}
	var clone = &Employee{}
	err = visit(func(key string, value interface{}) (bool, error) {
		switch key {
		case "ID":
			clone.ID = value.(int)
		case "Name":
			clone.Name = value.(string)
		case "Company":
			clone.Company = value.(string)
		}
		return true, nil
	})
	assert.Nil(t, err)
	assert.EqualValues(t, emp, clone)
}
