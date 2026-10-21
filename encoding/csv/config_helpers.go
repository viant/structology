package csv

import (
	"fmt"
	scalar "github.com/viant/structology/encoding/csv/internal/scalar"
	"reflect"
	"strings"
	"unicode/utf8"
)

// StringifierConfig retains the ported numeric/field formatting options without
// exposing a dependency on SQLX.
type StringifierConfig = scalar.StringifierConfig
type StringifierFloat32Config = scalar.StringifierFloat32Config
type StringifierFloat64Config = scalar.StringifierFloat64Config

func cloneConfig(source *Config) *Config {
	if source == nil {
		return &Config{}
	}
	copy := *source
	copy.UniqueFields = append([]string(nil), source.UniqueFields...)
	copy.ExcludedPaths = append([]string(nil), source.ExcludedPaths...)
	copy.StringifierConfig.Fields = append([]string(nil), source.StringifierConfig.Fields...)
	copy.References = make([]*Reference, len(source.References))
	for i, ref := range source.References {
		if ref != nil {
			value := *ref
			copy.References[i] = &value
		}
	}
	return &copy
}
func validateConfig(config *Config) error {
	if len(config.UniqueFields) > 0 || len(config.References) > 0 {
		return fmt.Errorf("CSV identity grouping is not supported; decode returns one root per CSV record")
	}
	if !utf8.ValidString(config.FieldSeparator) || utf8.RuneCountInString(config.FieldSeparator) != 1 || strings.ContainsAny(config.FieldSeparator, "\r\n\x00\"") {
		return fmt.Errorf("CSV delimiter must be one valid non-quote character")
	}
	if strings.ContainsAny(config.NullValue, "\r\n\"") || strings.Contains(config.NullValue, config.FieldSeparator) {
		return fmt.Errorf("CSV null marker cannot contain a delimiter, quote or newline")
	}
	for _, ref := range config.References {
		if ref == nil {
			return fmt.Errorf("CSV reference is nil")
		}
	}
	return nil
}

func rowType(t reflect.Type) (reflect.Type, error) {
	if t == nil {
		return nil, fmt.Errorf("CSV type is nil")
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("CSV requires a struct, pointer, or one collection of structs")
	}
	return t, nil
}
