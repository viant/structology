package format

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {

	var testCases = []struct {
		description string
		tag         reflect.StructTag
		tagName     string
	}{

		{
			description: "fallback",
			tagName:     "json",
			tag:         reflect.StructTag(`format:"dateFormat=YYYY-MM-DD,name=startDate" json:"dateFormat=YYYY-MM-DD-hh:mm" `),
		},
	}

	for _, testCase := range testCases {
		tag, err := Parse(testCase.tag, testCase.tagName)
		assert.Nil(t, err)
		fmt.Printf("%+v\n", tag)
	}
}
