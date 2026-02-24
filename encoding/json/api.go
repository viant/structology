package json

import (
	"context"
	"reflect"
	"strings"
	"time"
	"unsafe"

	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	jsonunmarshal "github.com/viant/structology/encoding/json/unmarshal"
	"github.com/viant/xunsafe"
)

var apiTimeType = reflect.TypeOf(time.Time{})

var (
	defaultMarshalEngine   = jsonmarshal.New(nil, nil, false, true, "", "", nil)
	defaultUnmarshalEngine = jsonunmarshal.New(
		context.Background(),
		scalarScannerHooks{},
		jsonunmarshal.IgnoreUnknown,
		jsonunmarshal.CoerceNumbers,
		jsonunmarshal.CompatNulls,
		jsonunmarshal.LastWins,
		jsonunmarshal.Tolerant,
		"",
		"",
		nil,
		nil,
	)
)

// MarshalContext marshals with an explicit context.
func MarshalContext(ctx context.Context, value interface{}, opts ...Option) ([]byte, error) {
	if len(opts) == 0 && isDefaultContext(ctx) {
		if elemType, ptr, ok := pointerStructMeta(value); ok {
			return defaultMarshalEngine.MarshalTypedPtr(nil, elemType, ptr)
		}
		if elemType, ptr, ok := structValueMeta(value); ok {
			return defaultMarshalEngine.MarshalTypedPtr(nil, elemType, ptr)
		}
		return defaultMarshalEngine.Marshal(value)
	}
	cfg := resolveOptions(ctx, opts)

	var transform func(path []string, field string) string
	caseKey := ""
	var compileName func(string) string
	if cfg.PathName != nil {
		transform = cfg.PathName.TransformPath
	} else if tr, ok := cfg.NameTransformer.(caseFormatTransformer); ok {
		caseKey = string(tr.caseFormat)
		compileName = func(field string) string { return tr.Transform("", field) }
	} else if _, ok := cfg.NameTransformer.(defaultNameTransformer); !ok && cfg.NameTransformer != nil {
		transform = func(path []string, field string) string {
			return cfg.NameTransformer.Transform(strings.Join(path, "."), field)
		}
	}

	var exclude func(path []string, field string) bool
	if cfg.PathExcluder != nil {
		exclude = cfg.PathExcluder.ExcludePath
	} else if _, ok := cfg.FieldExcluder.(noExcluder); !ok && cfg.FieldExcluder != nil {
		exclude = func(path []string, field string) bool {
			return cfg.FieldExcluder.Exclude(strings.Join(path, "."), field)
		}
	}

	m := jsonmarshal.New(transform, exclude, cfg.OmitEmpty, cfg.NilSlicePolicy == NilSliceAsNull, cfg.TimeLayout, caseKey, compileName)
	if transform == nil && exclude == nil {
		if elemType, ptr, ok := pointerStructMeta(value); ok {
			return m.MarshalTypedPtr(nil, elemType, ptr)
		}
		if elemType, ptr, ok := structValueMeta(value); ok {
			return m.MarshalTypedPtr(nil, elemType, ptr)
		}
	}
	return m.Marshal(value)
}

// Marshal marshals using context.Background unless overridden by options.
func Marshal(value interface{}, opts ...Option) ([]byte, error) {
	return MarshalContext(context.Background(), value, opts...)
}

// UnmarshalContext unmarshals with an explicit context.
func UnmarshalContext(ctx context.Context, data []byte, dest interface{}, opts ...Option) error {
	if len(opts) == 0 && isDefaultContext(ctx) {
		return defaultUnmarshalEngine.Unmarshal(data, dest)
	}
	cfg := resolveOptions(ctx, opts)
	unknown := jsonunmarshal.IgnoreUnknown
	if cfg.UnknownFieldPolicy == ErrorOnUnknown {
		unknown = jsonunmarshal.ErrorOnUnknown
	}
	number := jsonunmarshal.CoerceNumbers
	if cfg.NumberPolicy == ExactNumbers {
		number = jsonunmarshal.ExactNumbers
	}
	nulls := jsonunmarshal.CompatNulls
	if cfg.NullPolicy == StrictNulls {
		nulls = jsonunmarshal.StrictNulls
	}
	duplicates := jsonunmarshal.LastWins
	if cfg.DuplicateKeyPolicy == ErrorOnDuplicate {
		duplicates = jsonunmarshal.ErrorOnDuplicate
	}
	malformed := jsonunmarshal.Tolerant
	if cfg.MalformedPolicy == FailFast {
		malformed = jsonunmarshal.FailFast
	}
	caseKey := ""
	var compileName func(string) string
	if tr, ok := cfg.NameTransformer.(caseFormatTransformer); ok {
		caseKey = string(tr.caseFormat)
		compileName = func(field string) string { return tr.Transform("", field) }
	}
	u := jsonunmarshal.New(cfg.Ctx, cfg.scannerHooks, unknown, number, nulls, duplicates, malformed, cfg.TimeLayout, caseKey, compileName, cfg.PathUnmarshalHook)
	return u.Unmarshal(data, dest)
}

// Unmarshal unmarshals using context.Background unless overridden by options.
func Unmarshal(data []byte, dest interface{}, opts ...Option) error {
	return UnmarshalContext(context.Background(), data, dest, opts...)
}

func isPointerToStruct(v interface{}) bool {
	if v == nil {
		return false
	}
	rt := reflect.TypeOf(v)
	return rt.Kind() == reflect.Ptr && rt.Elem().Kind() == reflect.Struct
}

func pointerStructMeta(v interface{}) (reflect.Type, unsafe.Pointer, bool) {
	if !isPointerToStruct(v) {
		return nil, nil, false
	}
	rt := reflect.TypeOf(v)
	if rt.Elem() == apiTimeType {
		return nil, nil, false
	}
	return rt.Elem(), xunsafe.AsPointer(v), true
}

func structValueMeta(v interface{}) (reflect.Type, unsafe.Pointer, bool) {
	if v == nil {
		return nil, nil, false
	}
	rt := reflect.TypeOf(v)
	if rt.Kind() != reflect.Struct {
		return nil, nil, false
	}
	if rt == apiTimeType {
		return nil, nil, false
	}
	return rt, xunsafe.AsPointer(v), true
}

func isDefaultContext(ctx context.Context) bool {
	return ctx == nil || ctx == context.Background() || ctx == context.TODO()
}
