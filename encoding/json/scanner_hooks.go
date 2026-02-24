package json

// ScannerHooks contains block-scan hooks for decoder whitespace and token scans.
type ScannerHooks interface {
	SkipWhitespace(data []byte, pos int) int
	FindQuoteOrEscape(data []byte, pos int) (quotePos int, escapePos int)
	FindStructural(data []byte, pos int) int
}

type scalarScannerHooks struct{}

func (s scalarScannerHooks) SkipWhitespace(data []byte, pos int) int {
	for pos < len(data) {
		switch data[pos] {
		case ' ', '\n', '\r', '\t':
			pos++
		default:
			return pos
		}
	}
	return pos
}

func (s scalarScannerHooks) FindQuoteOrEscape(data []byte, pos int) (int, int) {
	for i := pos; i < len(data); i++ {
		if data[i] == '"' {
			return i, -1
		}
		if data[i] == '\\' {
			return -1, i
		}
	}
	return -1, -1
}

func (s scalarScannerHooks) FindStructural(data []byte, pos int) int {
	for i := pos; i < len(data); i++ {
		switch data[i] {
		case '{', '}', '[', ']', ':', ',':
			return i
		}
	}
	return -1
}
