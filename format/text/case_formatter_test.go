package text

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCase_Format(t *testing.T) {
	var useCases = []struct {
		description string
		from        CaseFormat
		to          CaseFormat
		input       string
		expect      string
	}{

		{
			description: "lower camel to upper underscore",
			input:       "abcXyzId",
			from:        CaseFormatLowerCamel,
			to:          CaseFormatUpperUnderscore,
			expect:      "ABC_XYZ_ID",
		},
		{
			description: "upper camel to upper underscore",
			input:       "AbcXyzId",
			from:        CaseFormatUpperCamel,
			to:          CaseFormatUpperUnderscore,
			expect:      "ABC_XYZ_ID",
		},
		{
			description: "upper underscore to upper camel ",
			input:       "ABC_XYZ_ID",
			from:        CaseFormatUpperUnderscore,
			to:          CaseFormatUpperCamel,
			expect:      "AbcXyzId",
		},
		{
			description: "upper underscore to sentence",
			input:       "ABC_XYZ_ID",
			from:        CaseFormatUpperUnderscore,
			to:          CaseFormatSentence,
			expect:      "Abc xyz id",
		},
		{
			description: "lower camel dash",
			input:       "abcXyzID",
			from:        CaseFormatLowerCamel,
			to:          CaseFormatDash,
			expect:      "abc-Xyz-ID",
		},
		{
			description: "lower camel dash",
			input:       "abcXyzID",
			from:        CaseFormatLowerCamel,
			to:          CaseFormatLowerUnderscore,
			expect:      "abc_xyz_id",
		},
	}

	for _, useCase := range useCases {
		formatter := useCase.from.To(useCase.to)
		actual := formatter.Format(useCase.input)
		assert.EqualValues(t, useCase.expect, actual, useCase.description)
	}

}
