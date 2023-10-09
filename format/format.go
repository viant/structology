package format

import (
	"fmt"
	"github.com/viant/structology/format/text"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
	"time"
)

func (t *Tag) FormatTime(ts *time.Time) string {
	if t.TimeLayout == "" {
		return ts.Format(time.RFC3339)
	}
	return ts.Format(t.TimeLayout)
}

func (t *Tag) FormatName() string {
	return t.CaseFormatName("")
}

func (t *Tag) CaseFormatName(defaultCaseFormat text.CaseFormat) string {
	if t.CaseFormat == "-" {
		return t.Name
	}
	caseFormat := t.CaseFormat
	if caseFormat == "" {
		caseFormat = string(defaultCaseFormat)
	}
	if caseFormat == "" {
		return t.Name
	}

	if t.formatter != nil {
		if string(t.formatter.To()) != caseFormat {
			t.formatter = nil
		}
	}
	if t.formatter == nil {
		to := text.NewCaseFormat(caseFormat)
		t.formatter = text.CaseFormatUpperCamel.To(to)
		t.CaseFormat = string(to)
	}
	return t.formatter.Format(t.Name)
}

func (t *Tag) FormatFloat(f float64) (string, error) {
	//TODO load printer language from tag
	p := message.NewPrinter(language.AmericanEnglish)
	switch t.Format {
	case "Decimal":
		return p.Sprintf("%v", number.Decimal(f)), nil
	default:
		return "", fmt.Errorf("foramt: %s not yet supported", t.Format)
	}
}
