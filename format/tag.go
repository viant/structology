package format

import (
	"fmt"
	"github.com/viant/parsly"
	"github.com/viant/structology/format/text"
	ftime "github.com/viant/structology/format/time"
	"reflect"
	"strings"
)

const (
	TagName = "format"
)

type Tag struct {
	Name string //source for output name, is case formater is not defined, use Name otherwise use Name with UpperCamel format
	//to format output name with specified CaseFormat

	CaseFormat string

	DateFormat string
	TimeLayout string
	Format     string

	Inline    bool
	Omitempty bool
	Ignore    bool

	//TBD
	Precision int
	Scale     int

	Language string

	formatter *text.CaseFormatter
}

func (t *Tag) update(key string, value string, strictMode bool) error {
	switch strings.ToLower(key) {
	case "name":
		t.Name = value
	case "dateformat", "isodateformat", "iso20220715":
		t.DateFormat = value
		t.TimeLayout = ftime.DateFormatToTimeLayout(value)
	case "timelayout", "datelayout", "rfc3339":
		t.TimeLayout = value
	case "format":
		t.Format = value
	case "caseformat":
		t.CaseFormat = value
	case "inline", "embed":
		t.Inline = true
	case "omitempty":
		t.Omitempty = true
	case "ignore", "-", "transient":
		t.Ignore = true
	case "lang", "language":
		t.Language = value
	default:
		if strictMode {
			return fmt.Errorf("Unknown key " + key)
		}
	}
	return nil
}

func Parse(tag reflect.StructTag, names ...string) (*Tag, error) {
	ret := &Tag{}

	names = append([]string{TagName}, names...)
	for i, name := range names {
		encoded := tag.Get(name)
		if encoded == "" {
			continue
		}
		switch encoded {
		case "-":
			ret.Ignore = true
		case ",omitempty":
			ret.Omitempty = true
		}
		cursor := parsly.NewCursor("", []byte(encoded), 0)
		for cursor.Pos < len(cursor.Input) {
			key, value := matchPair(cursor)
			if err := ret.update(key, value, i == 0); err != nil {
				return nil, err
			}
			if key == "" {
				break
			}
		}
	}
	return ret, nil
}

func matchPair(cursor *parsly.Cursor) (string, string) {
	key := ""
	value := ""
	match := cursor.MatchAny(scopeBlockMatcher, comaTerminatorMatcher)
	switch match.Code {
	case scopeBlockToken:
		value = match.Text(cursor)
		value = value[1 : len(value)-1]
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
	}
	return key, value
}
