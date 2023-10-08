package text

import (
	"strings"
	"sync"
	"unicode"
)

var caseFormatter = make([]*CaseFormatter, 100)
var mux sync.RWMutex

// CaseFormatter defines words case formatter
type CaseFormatter struct {
	nop       bool
	from      CaseFormat
	to        CaseFormat
	sep       string
	firstWord func(word string) string
	nextWords func(word string) string
}

func (c *CaseFormatter) To() CaseFormat {
	return c.to
}

// Format converts source to desired case format
func (c *CaseFormatter) Format(src string) string {
	if c.nop {
		return src
	}
	var result = strings.Builder{}
	result.Grow(3 + len(src))
	offset := 0
	pos, sep := c.from.nextWord(offset, src)
	if pos == -1 {
		pos = len(src) - 1
		return c.firstWord(src)
	}
	result.WriteString(c.firstWord(src[:pos-len(sep)]))
	result.WriteString(c.sep)

	offset = pos

	for i := offset; i < len(src); i++ {
		pos, sep = c.from.nextWord(offset, src)
		if pos == -1 {
			break
		}
		end := offset + pos - len(sep)
		result.WriteString(c.nextWords(src[offset:end]))
		result.WriteString(c.sep)
		offset += pos
	}

	if offset < len(src) {
		result.WriteString(c.nextWords(src[offset:]))
	}
	return result.String()
}

func (c CaseFormat) To(to CaseFormat) *CaseFormatter {
	if c.Index() == 0 || to.Index() == 0 {
		return &CaseFormatter{nop: true}
	}
	index := (c.Index()-1)*10 + to.Index() - 1
	mux.RLock()
	ret := caseFormatter[index]
	mux.RUnlock()
	if ret != nil {
		return ret
	}

	toUpper := false
	toLower := false
	isCamel := false
	ret = &CaseFormatter{
		from:      c,
		to:        to,
		firstWord: rawText,
		nextWords: rawText,
	}
	switch to {
	case CaseFormatUpperUnderscore:
		toUpper = true
		ret.sep = "_"
	case CaseFormatLowerUnderscore:
		ret.sep = "_"
		toLower = true
	case CaseFormatDash:
		ret.sep = "-"
	case CaseFormatUpperDash:
		ret.sep = "-"
		toUpper = true
	case CaseFormatLowerDash:
		ret.sep = "-"
		toLower = true
	case CaseFormatUpper:
		toUpper = true
	case CaseFormatLower:
		toLower = true
	case CaseFormatUpperCamel:
		isCamel = true
	case CaseFormatLowerCamel:
		isCamel = true
	case CaseFormatTitle:
		ret.sep = " "
		isCamel = true
	case CaseFormatSentence:
		ret.sep = " "
	}
	if toUpper {
		ret.firstWord = strings.ToUpper
		ret.nextWords = strings.ToUpper
	} else if toLower {
		ret.firstWord = strings.ToLower
		ret.nextWords = strings.ToLower
	} else if isCamel {
		ret.nextWords = ToTitle
		ret.firstWord = ToTitle
		if to == CaseFormatLowerCamel {
			ret.firstWord = strings.ToLower
		}
	} else {
		if to == CaseFormatSentence {
			ret.nextWords = strings.ToLower
			ret.firstWord = ToTitle
		}
	}
	mux.Lock()
	caseFormatter[index] = ret
	mux.Unlock()
	return ret
}

func rawText(w string) string {
	return w
}

func ToTitle(word string) string {
	if word == "" {
		return word
	}
	var ret = make([]rune, 0, len(word))
	for i, r := range word {
		if i == 0 {
			ret = append(ret, unicode.ToUpper(r))
			continue
		}
		ret = append(ret, unicode.ToLower(r))
	}
	return string(ret)
}
