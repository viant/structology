package json

import (
	"context"
	stdjson "encoding/json"
	"fmt"
	"strings"

	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
)

// MarshalStandard applies field exclusion while retaining encoding/json v1
// field dominance, tags and value encoding. Custom value encoders own their
// complete value. Formatting transforms belong to MarshalContext instead.
func MarshalStandard(value any, opts ...Option) ([]byte, error) {
	if len(opts) == 0 {
		return stdjson.Marshal(value)
	}
	cfg := resolveOptions(context.Background(), opts)
	_, standardNames := cfg.NameTransformer.(defaultNameTransformer)
	if cfg.CaseFormat != "" || cfg.FormatTag != nil || cfg.OmitEmpty || cfg.PathName != nil || (!standardNames && cfg.NameTransformer != nil) {
		return nil, fmt.Errorf("standard JSON selection does not accept format transforms")
	}
	var exclude func([]string, string) bool
	if cfg.PathExcluder != nil {
		exclude = cfg.PathExcluder.ExcludePath
	} else if _, empty := cfg.FieldExcluder.(noExcluder); !empty && cfg.FieldExcluder != nil {
		exclude = func(path []string, name string) bool { return cfg.FieldExcluder.Exclude(strings.Join(path, "."), name) }
	}
	var byIndex func([]string, []int) bool
	if indexed, ok := cfg.PathExcluder.(IndexedPathFieldExcluder); ok {
		byIndex = indexed.ExcludeField
	}
	return jsonmarshal.NewStandardEncoder(exclude, byIndex).Marshal(value)
}

// IndexedPathFieldExcluder optionally supplies exact promoted-field identity to
// standard encoding. It uses the same parent wire paths as PathFieldExcluder.
type IndexedPathFieldExcluder interface {
	PathFieldExcluder
	ExcludeField(path []string, index []int) bool
}
