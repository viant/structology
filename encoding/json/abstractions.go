package json

import "github.com/viant/tagly/format/text"

type defaultNameTransformer struct{}

func (d defaultNameTransformer) Transform(_ string, fieldName string) string {
	return fieldName
}

type caseFormatTransformer struct {
	caseFormat text.CaseFormat
}

func (c caseFormatTransformer) Transform(_ string, fieldName string) string {
	if c.caseFormat == "" {
		return fieldName
	}
	if fieldName == "ID" {
		switch c.caseFormat {
		case text.CaseFormatLower, text.CaseFormatLowerCamel, text.CaseFormatLowerUnderscore:
			return "id"
		}
	}
	src := text.DetectCaseFormat(fieldName)
	if !src.IsDefined() {
		src = text.CaseFormatUpperCamel
	}
	return src.Format(fieldName, c.caseFormat)
}

type noExcluder struct{}

func (n noExcluder) Exclude(_ string, _ string) bool { return false }
