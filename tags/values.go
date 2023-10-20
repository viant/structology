package tags

import "github.com/viant/parsly"

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
