package tags

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestValues_MatchPairs(t *testing.T) {
	var testCases = []struct {
		description string
		input       string
		expect      map[string]string
	}{

		{

			description: "mixed",
			input:       ",omitempty,path=@exclude-ids",
			expect: map[string]string{
				"omitempty": "",
				"path":      "@exclude-ids",
			},
		},
	}
	for _, testCase := range testCases {
		values := Values(testCase.input)
		actual := map[string]string{}
		err := values.MatchPairs(func(key, value string) error {
			actual[key] = value
			return nil
		})
		assert.Nil(t, err)
		assert.EqualValues(t, testCase.expect, actual, testCase.description)
	}
}
