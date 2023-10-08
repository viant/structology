package text

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDetectCaseFormat(t *testing.T) {
	var testCases = []struct {
		names  []string
		expect CaseFormat
	}{
		{
			names: []string{
				"NAME",
				"EMP_ID",
			},
			expect: NewCaseFormat("uu"),
		},
		{
			names: []string{
				"eventTypeId",
				"event",
			},
			expect: NewCaseFormat("lc"),
		},
		{
			names: []string{
				"EVENT",
			},
			expect: NewCaseFormat("u"),
		},
		{
			names: []string{
				"event",
			},
			expect: NewCaseFormat("l"),
		},
		{
			names: []string{
				"This is sentence",
			},
			expect: NewCaseFormat("s"),
		},
		{
			names: []string{
				"This Is Title",
			},
			expect: NewCaseFormat("t"),
		},
	}

	for i, testCase := range testCases {
		actual := DetectCaseFormat(testCase.names...)
		assert.EqualValues(t, testCase.expect, actual, fmt.Sprintf("detect (%v) ", i)+string(testCase.expect))
	}

}
