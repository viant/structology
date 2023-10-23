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

func (v Values) Name() (Values, string) {
	text := string(v)
	index := strings.Index(text, ",")
	if index == -1 {
		return "", text
	}
	name := text[:index]
	if eqIndex := strings.Index(name, "="); eqIndex != -1 {
		return v, ""
	}
	return Values(text[index+1:]), name
}
