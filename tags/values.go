package tags

import "github.com/viant/parsly"

// Values represents tag values
type Values string

// MatchPairs match paris separated by ,
func (v Values) MatchPairs(onMatch func(key, value string) error) error {
	cursor := parsly.NewCursor("", []byte(v), 0)
	for cursor.Pos < len(cursor.Input) {
		key, value := matchPair(cursor)
		if err := onMatch(key, value); err != nil {
			return err
		}
		if key == "" {
			break
		}
	}
	return nil
}
