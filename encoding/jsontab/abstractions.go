package jsontab

import "github.com/viant/tagly/format/text"

type caseFormatTransformer struct {
	caseFormat text.CaseFormat
}

func (c caseFormatTransformer) Transform(fieldName string) string {
	src := text.DetectCaseFormat(fieldName)
	if src == text.CaseFormatUndefined {
		src = text.CaseFormatUpperCamel
	}
	return src.Format(fieldName, c.caseFormat)
}
