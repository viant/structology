package tags

import (
	"fmt"
	"github.com/viant/structology/format/text"
	"github.com/viant/xunsafe"
	"reflect"
	"strconv"
	"strings"
)

const TagName = "tag"

type (
	//Tag
	Tag struct {
		Name   string
		Values Values
	}

	//Tags represents tags
	Tags []*Tag
)

// Stringify returns stringified tags representations
func (t Tags) Stringify() string {
	builder := strings.Builder{}
	for i, tag := range t {
		if i > 0 {
			builder.WriteString(" ")
		}
		builder.WriteString(tag.Name)
		builder.WriteString(":")
		value := strconv.Quote(string(tag.Values))
		builder.WriteString(value)
	}
	return builder.String()
}

// Append appends tag value element
func (e *Tag) Append(value string) {
	if value == "" {
		return
	}
	if e.Values == "" {
		e.Values = Values(value)
		return
	}
	e.Values = Values(string(e.Values) + "," + value)
}

// Lookup returns matched by name tag
func (t Tags) Lookup(name string) *Tag {
	for _, candidate := range t {
		if candidate.Name == name {
			return candidate
		}
	}
	return nil
}

// Set sets tag value
func (t *Tags) Set(tag string, value string) {
	if len(value) == 0 {
		return
	}
	aTag := t.Lookup(tag)
	if aTag == nil {
		aTag = &Tag{}
		*t = append(*t, aTag)
	}
	aTag.Values = Values(value)
}

// Set sets tag value
func (t *Tags) Append(tag string, value string) {
	if len(value) == 0 {
		return
	}
	aTag := t.Lookup(tag)
	if aTag == nil {
		aTag = &Tag{}
		*t = append(*t, aTag)
	}
	aTag.Append(value)
}

// NewTags create a tags for supplied tag literal
func NewTags(tagLiteral string) Tags {
	var result []*Tag
	for tagLiteral != "" {
		i := 0
		for i < len(tagLiteral) && tagLiteral[i] == ' ' {
			i++
		}
		tagLiteral = tagLiteral[i:]
		if tagLiteral == "" {
			break
		}
		i = 0
		for i < len(tagLiteral) && tagLiteral[i] > ' ' && tagLiteral[i] != ':' && tagLiteral[i] != '"' && tagLiteral[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tagLiteral) || tagLiteral[i] != ':' || tagLiteral[i+1] != '"' {
			break
		}
		name := tagLiteral[:i]
		tagLiteral = tagLiteral[i+1:]
		i = 1
		for i < len(tagLiteral) && tagLiteral[i] != '"' {
			if tagLiteral[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tagLiteral) {
			break
		}
		quotedValue := tagLiteral[:i+1]
		tagLiteral = tagLiteral[i+1:]
		value, err := strconv.Unquote(quotedValue)
		if err != nil {
			break
		}
		aTag := &Tag{Name: name, Values: Values(value)}
		result = append(result, aTag)
	}
	return result
}

// NewTag creates a tag for supplied tag type
func NewTag(name string, value interface{}) *Tag {
	if value == nil {
		return nil
	}

	rType := reflect.TypeOf(value)
	xStruct := xunsafe.NewStruct(rType)
	ptr := xunsafe.AsPointer(value)
	ret := &Tag{Name: name}
	for i := range xStruct.Fields {
		aField := &xStruct.Fields[i]
		name := aField.Tag.Get(TagName)
		if name == "-" {
			continue
		}
		if name == "" {
			caseFormat := text.DetectCaseFormat(aField.Name)
			name = caseFormat.Format(aField.Name, text.CaseFormatLowerCamel)
		}

		value := aField.Value(ptr)
		switch actual := value.(type) {
		case string:
			ret.Append(name + "=" + wrapValueIfNeeded(actual))
		case int:
			ret.Append(name + "=" + strconv.Itoa(actual))
		case bool:
			ret.Append(name + "=" + strconv.FormatBool(actual))
		case float64:
			ret.Append(name + "=" + strconv.FormatFloat(actual, 'f', -1, 32))
		default:
			aText := fmt.Sprintf("%s", actual)
			ret.Append(name + "=" + wrapValueIfNeeded(aText))
		}
	}
	return ret
}

func wrapValueIfNeeded(actual string) string {
	if strings.Contains(actual, ",") && !strings.HasPrefix(actual, "{") {
		actual = "{" + actual + "}"
	}
	return actual
}
