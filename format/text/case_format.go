package text

import (
	"strings"
	"unicode"
)

// CaseFormat defines case format
type CaseFormat string

const (
	CaseFormatUndefined                  = ""
	CaseFormatUpper           CaseFormat = "upper"
	CaseFormatLower           CaseFormat = "lower"
	CaseFormatUpperCamel      CaseFormat = "upperCamel"
	CaseFormatLowerCamel      CaseFormat = "lowerCamel"
	CaseFormatTitle           CaseFormat = "title"
	CaseFormatSentence        CaseFormat = "sentence"
	CaseFormatUpperUnderscore CaseFormat = "upperUnderscore"
	CaseFormatLowerUnderscore CaseFormat = "lowerUnderscore"
	CaseFormatDash            CaseFormat = "dash"
	CaseFormatLowerDash       CaseFormat = "lowerdash"
	CaseFormatUpperDash       CaseFormat = "upperdash"
)

func (c CaseFormat) isDefined() bool {
	return c.index() > 0
}

// IsDefined returns true if case format is defined
func (c CaseFormat) IsDefined() bool {
	return c.Index() > 0
}

func (c CaseFormat) Format(text string, caseFormat CaseFormat) string {
	return c.To(caseFormat).Format(text)
}
func (c CaseFormat) Index() int {
	switch c {
	case CaseFormatUndefined:
		return 0
	case CaseFormatUpper:
		return 1
	case CaseFormatLower:
		return 2
	case CaseFormatUpperCamel:
		return 3
	case CaseFormatLowerCamel:
		return 4
	case CaseFormatTitle:
		return 5
	case CaseFormatSentence:
		return 6
	case CaseFormatUpperUnderscore:
		return 7
	case CaseFormatLowerUnderscore:
		return 8
	case CaseFormatDash:
		return 9
	case CaseFormatLowerDash:
		return 10
	case CaseFormatUpperDash:
		return 11
	default:
		if alternative := NewCaseFormat(string(c)); alternative != CaseFormatUndefined {
			return alternative.Index()
		}
		return 0
	}
}

func (c CaseFormat) index() int {
	switch c {
	case CaseFormatUndefined:
		return 0
	case CaseFormatUpper:
		return 1
	case CaseFormatLower:
		return 2
	case CaseFormatUpperCamel:
		return 3
	case CaseFormatLowerCamel:
		return 4
	case CaseFormatTitle:
		return 5
	case CaseFormatSentence:
		return 6
	case CaseFormatUpperUnderscore:
		return 7
	case CaseFormatLowerUnderscore:
		return 8
	case CaseFormatDash:
		return 9
	case CaseFormatLowerDash:
		return 10
	case CaseFormatUpperDash:
		return 11
	default:
		return 0
	}
}

func DetectCaseFormat(words ...string) CaseFormat {
	if len(words) == 0 {
		return CaseFormatLowerCamel
	}
	result := ""
	var firstUpper bool
	upperCases := 0
	lowerCases := 0
	camels := 0
	separators := 0
	upperCamels := 0
	sep := ""

outer:
	for _, datum := range words {
		var wasUpper *bool
		for i, r := range datum {
			if unicode.IsLetter(r) {
				isUpper := unicode.IsUpper(r)
				if i == 0 && isUpper {
					firstUpper = true
				}
				if isUpper {
					upperCases++
				} else {
					lowerCases++
				}
				if wasUpper == nil && isUpper {
					upperCamels++
				}
				if wasUpper != nil && isUpper != *wasUpper {
					camels++
				}
				wasUpper = &isUpper
				continue
			}
			sep = string(r)
			if sep != " " || separators > 1 {
				break outer
			}
			separators++
			wasUpper = nil
		}
	}

	result = ""
	if upperCases > 0 && lowerCases == 0 {
		result = "u"
	} else if lowerCases > 0 && upperCases == 0 {
		result = "l"
	} else {
		if camels > 0 {
			result = "l"
			if firstUpper {
				result = "u"
			}
		}
	}

	switch sep {
	case " ":
		if firstUpper {
			result = "s"
		}
		if upperCases > 1 {
			result = "t"
		}
	case "-":
		result += "d"
	case "_":
		result += "u"
	case "":
		if camels > 0 {
			result += "c"
		}

	}
	return NewCaseFormat(result)
}

func NewCaseFormat(name string) CaseFormat {
	name = strings.ToLower(name)
	switch name {
	case "upper", "u":
		return CaseFormatUpper
	case "lower", "l":
		return CaseFormatLower
	case "dash", "d":
		return CaseFormatDash
	case "lowerdash", "ld":
		return CaseFormatLowerDash
	case "upperdash", "ud":
		return CaseFormatUpperDash
	case "lowercamel", "lc", "lowerpascal", "lp":
		return CaseFormatLowerCamel
	case "uppercamel", "uc", "upperpascal", "up":
		return CaseFormatUpperCamel
	case "lowerunderscore", "lu", "lowersnake":
		return CaseFormatLowerUnderscore
	case "upperunderscore", "uu", "uppersnake":
		return CaseFormatUpperUnderscore
	case "title", "t", "start":
		return CaseFormatTitle
	case "sentence", "s":
		return CaseFormatSentence
	default:
		return CaseFormatUndefined
	}
}

func (c CaseFormat) nextWord(offset int, src string) (int, string) {

	var separator string
	ret := -1
	if offset >= len(src) {
		return ret, separator
	}
	wasSpecial := false
	var wasUpper *bool
	for i, r := range src[offset:] {
		if unicode.IsLetter(r) {
			if wasSpecial {
				ret = i
				break
			}
			isUpper := unicode.IsUpper(r)
			if i > 0 { //word boundry detection based on letter case changes
				if wasUpper != nil && isUpper != *wasUpper {
					ret = i
					break
				}
				wasUpper = &isUpper
			}

			continue
		}
		separator = string(r)
		wasSpecial = true
	}
	return ret, separator
}
