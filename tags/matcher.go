package tags

import (
	"github.com/viant/parsly"
	"strings"
)

func matchPair(cursor *parsly.Cursor) (string, string) {
	key := ""
	value := ""
	match := cursor.MatchAny(scopeBlockMatcher, comaTerminatorMatcher)
	switch match.Code {
	case scopeBlockToken:
		value = match.Text(cursor)
		match = cursor.MatchAny(comaTerminatorMatcher)
	case comaTerminatorToken:
		value = match.Text(cursor)
		value = value[:len(value)-1] //exclude ,
	default:
		if cursor.Pos < len(cursor.Input) {
			value = string(cursor.Input[cursor.Pos:])
			cursor.Pos = len(cursor.Input)
		}
	}
	if index := strings.Index(value, "="); index != -1 {
		key = value[:index]
		value = value[index+1:]
	} else {
		key = value
		value = ""
	}
	return key, value
}

func matchElement(cursor *parsly.Cursor) string {
	value := ""
	match := cursor.MatchAny(scopeBlockMatcher, comaTerminatorMatcher)
	switch match.Code {
	case scopeBlockToken:
		value = match.Text(cursor)
		match = cursor.MatchAny(comaTerminatorMatcher)
	case comaTerminatorToken:
		value = match.Text(cursor)
		value = value[:len(value)-1] //exclude ,
	default:
		if cursor.Pos < len(cursor.Input) {
			value = string(cursor.Input[cursor.Pos:])
			cursor.Pos = len(cursor.Input)
		}
	}
	return value
}
