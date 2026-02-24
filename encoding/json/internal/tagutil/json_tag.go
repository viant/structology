package tagutil

import "strings"

type JSONTag struct {
	Name      string
	OmitEmpty bool
	Explicit  bool
	Transient bool
}

func ParseJSONTag(defaultName string, raw string) JSONTag {
	if raw == "" {
		return JSONTag{Name: defaultName}
	}
	parts := strings.Split(raw, ",")
	name := parts[0]
	explicit := true
	if name == "" {
		name = defaultName
	}
	tag := JSONTag{
		Name:      name,
		Explicit:  explicit,
		Transient: name == "-",
	}
	for _, p := range parts[1:] {
		if p == "omitempty" {
			tag.OmitEmpty = true
			break
		}
	}
	return tag
}
