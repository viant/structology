package tagutil

import (
	"reflect"
	"sync"

	"github.com/viant/tagly/format"
	ftime "github.com/viant/tagly/format/time"
)

// FormatFieldTag captures effective per-field format-tag attributes compiled into runtime plans.
type FormatFieldTag struct {
	Name          string
	HasNameOrCase bool
	OmitEmpty     bool
	Ignore        bool
	Inline        bool
	Nullable      bool
	TimeLayout    string
}

type ResolvedFieldTag struct {
	Name      string
	Explicit  bool
	OmitEmpty bool
	Ignore    bool
	Inline    bool
	Format    FormatFieldTag
}

type cachedFormatTag struct {
	name        string
	caseFormat  string
	hasName     bool
	omitEmpty   bool
	ignore      bool
	inline      bool
	nullable    bool
	hasNullable bool
	timeLayout  string
	dateFormat  string
}

var formatTagCache sync.Map // map[string]cachedFormatTag

// ParseFormatFieldTag parses struct-level `format` tag attributes used by the JSON runtime.
func ParseFormatFieldTag(sf reflect.StructField, baseName string) FormatFieldTag {
	ret := FormatFieldTag{}
	rawTag := string(sf.Tag)
	cached, ok := loadCachedFormatTag(rawTag)
	if !ok {
		return ret
	}

	ret.OmitEmpty = cached.omitEmpty
	ret.Ignore = cached.ignore
	ret.Inline = cached.inline
	if cached.hasNullable {
		ret.Nullable = cached.nullable
	}

	if cached.timeLayout != "" {
		ret.TimeLayout = cached.timeLayout
	} else if cached.dateFormat != "" {
		ret.TimeLayout = ftime.DateFormatToTimeLayout(cached.dateFormat)
	}

	if cached.hasName || cached.caseFormat != "" {
		tag := &format.Tag{
			Name:       cached.name,
			CaseFormat: cached.caseFormat,
		}
		if tag.Name == "" {
			tag.Name = baseName
		}
		ret.Name = tag.CaseFormatName("")
		ret.HasNameOrCase = ret.Name != ""
	}
	return ret
}

// ResolveFieldTag resolves precedence among json, jsonx and format tags.
// Precedence:
// 1) json explicit name/transient wins over format name/case.
// 2) inline is enabled by anonymous or jsonx:inline or format:inline.
// 3) ignore is enabled by json:"-" or internal:true or format:ignore.
// 4) omitempty is enabled by json omitempty OR format omitempty.
func ResolveFieldTag(sf reflect.StructField) ResolvedFieldTag {
	jTag := ParseJSONTag(sf.Name, sf.Tag.Get("json"))
	fTag := ParseFormatFieldTag(sf, jTag.Name)

	name := jTag.Name
	explicit := jTag.Explicit
	if !jTag.Explicit && fTag.HasNameOrCase {
		name = fTag.Name
		explicit = true
	}
	return ResolvedFieldTag{
		Name:      name,
		Explicit:  explicit,
		OmitEmpty: jTag.OmitEmpty || fTag.OmitEmpty,
		Ignore:    jTag.Transient || sf.Tag.Get("internal") == "true" || fTag.Ignore,
		Inline:    sf.Anonymous || sf.Tag.Get("jsonx") == "inline" || fTag.Inline,
		Format:    fTag,
	}
}

func loadCachedFormatTag(rawTag string) (cachedFormatTag, bool) {
	if v, ok := formatTagCache.Load(rawTag); ok {
		return v.(cachedFormatTag), true
	}
	tag, err := format.Parse(reflect.StructTag(rawTag))
	if err != nil || tag == nil {
		empty := cachedFormatTag{}
		formatTagCache.Store(rawTag, empty)
		return empty, false
	}
	cached := cachedFormatTag{
		name:       tag.Name,
		caseFormat: tag.CaseFormat,
		hasName:    tag.Name != "",
		omitEmpty:  tag.Omitempty,
		ignore:     tag.Ignore,
		inline:     tag.Inline,
		timeLayout: tag.TimeLayout,
		dateFormat: tag.DateFormat,
	}
	if tag.Nullable != nil {
		cached.hasNullable = true
		cached.nullable = *tag.Nullable
	}
	formatTagCache.Store(rawTag, cached)
	return cached, true
}
