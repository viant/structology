package format

import (
	"fmt"
	"github.com/viant/structology/format/text"
	ftime "github.com/viant/structology/format/time"
	"github.com/viant/structology/tags"
	"reflect"
	"strings"
)

const (
	TagName = "format"
)

type Tag struct {
	Name string `tag:"name,omitempty"` //source for output name, is case formater is not defined, use Name otherwise use Name in UpperCamel format
	//to format output name with specified CaseFormat

	CaseFormat string `tag:"caseFormat,omitempty"`

	DateFormat string `tag:"dataFormat,omitempty"`
	TimeLayout string `tag:"timeLayout,omitempty"`
	FormatMask string `tag:"formatMask,omitempty"`
	//Nullable flag to output null value as opposed zero/empty value
	Nullable  *bool `tag:"nullable,omitempty"`
	Inline    bool  `tag:"inline,omitempty"`
	Omitempty bool  `tag:"omitempty,omitempty"`
	Ignore    bool  `tag:"ignore,omitempty"`

	//TBD
	Precision int    `tag:"-"`
	Scale     int    `tag:"-"`
	Language  string `tag:"-"`

	formatter *text.CaseFormatter `tag:"-"`
}

func (t *Tag) IsNullable() bool {
	if t.Nullable == nil {
		return false
	}
	return *t.Nullable
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
		t.FormatMask = value
	case "caseformat":
		t.CaseFormat = value
	case "ignorecaseformatter":
		t.CaseFormat = "-"
	case "inline", "embed", "embedded":
		t.Inline = true
	case "omitempty":
		t.Omitempty = true
	case "nullable":
		nullable := value == "true"
		t.Nullable = &nullable
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

var tagKeys = map[string]bool{
	"name":          true,
	"dateformat":    true,
	"isodateformat": true,
	"timelayout":    true, "datelayout": true, "rfc3339": true,
	"format":              true,
	"caseformat":          true,
	"ignorecaseformatter": true,
	"inline":              true, "embed": true, "embedded": true,
	"omitempty": true,
	"ignore":    true, "transient": true,
	"lang": true, "language": true,
}

func IsValidTagKey(key string) bool {
	return tagKeys[key]
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
			continue
		}

		if index := strings.Index(encoded, ",omitempty"); index != -1 && strings.HasSuffix(encoded, ",omitempty") {
			ret.Omitempty = true
			encoded = encoded[:index]
		}

		if !strings.Contains(encoded, ",") && !strings.Contains(encoded, "=") {
			ret.Name = encoded
			continue
		}

		values := tags.Values(encoded)
		if err := values.MatchPairs(func(key, value string) error {
			return ret.update(key, value, i == 0)
		}); err != nil {
			return nil, err
		}
	}
	return ret, nil
}
