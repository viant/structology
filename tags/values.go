package tags

import (
	"github.com/viant/parsly"
	"strings"
)

// Values represents tag values
type Values string

// MatchPairs matches paris separated by ,
func (v Values) MatchPairs(onMatch func(key, value string) error) error {
	cursor := parsly.NewCursor("", []byte(v), 0)
	for cursor.Pos < len(cursor.Input) {
		key, value := matchPair(cursor)
		if key == "" {
			continue
		}
		if err := onMatch(key, value); err != nil {
			return err
		}
	}
	return nil
}

// Match matches elements separated by ,
func (v Values) Match(onMatch func(value string) error) error {
	cursor := parsly.NewCursor("", []byte(v), 0)
	for cursor.Pos < len(cursor.Input) {
		value := matchElement(cursor)
		if value == "" {
			continue
		}
		if err := onMatch(value); err != nil {
			return err
		}
	}
	return nil
}

// Name returns tag value and remaining values
func (v Values) Name() (string, Values) {
	text := string(v)
	comaIndex := strings.Index(text, ",")
	eqIndex := strings.Index(text, "=")
	if comaIndex == -1 {
		if eqIndex == -1 {
			return text, ""
		}
		return "", v
	} else {
		if eqIndex != -1 && eqIndex > comaIndex {
			name := text[:comaIndex]
			values := text[comaIndex+1:]
			return name, Values(values)
		}
	}
	name := text[:comaIndex]
	eqIndex = strings.Index(text, "=")
	if eqIndex != -1 {
		return "", v
	}
	return name, Values(text[comaIndex+1:])
}
