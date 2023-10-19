package time

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestParse(t *testing.T) {
	var testCases = []struct {
		description string
		layout      string
		input       string
	}{
		{
			description: "iso time",
			input:       "2023-01-02 01:22:19",
		},
		{
			description: "rfc time",
			input:       "2023-01-02T01:22:19",
		},
		{
			description: "date",
			input:       "2023-01-02",
		},
	}

	for _, testCase := range testCases {
		ts, err := Parse(testCase.layout, testCase.input)
		assert.Nil(t, err, testCase.description)
		fmt.Printf("%s\n", ts.String())
	}
}
