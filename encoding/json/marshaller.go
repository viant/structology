package json

import (
	"context"
	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"reflect"
	"strings"
	"sync"
)

// Marshaller retains one configured native engine for encoding and metadata.
// Options and callbacks must be immutable after construction.
type Marshaller struct {
	engine *jsonmarshal.Engine
	typeOf reflect.Type
	once   sync.Once
	shape  *jsonmarshal.WireShape
	err    error
}

func NewMarshaller(typeOf reflect.Type, opts ...Option) (*Marshaller, error) {
	cfg := resolveOptions(context.Background(), opts)
	engine := cfg.marshalEngine()
	if err := engine.ExcludeFields(typeOf, cfg.ExcludedFields); err != nil {
		return nil, err
	}
	return &Marshaller{typeOf: typeOf, engine: engine}, nil
}
func (m *Marshaller) Marshal(value interface{}) ([]byte, error) {
	if m.engine.NameTransform == nil && m.engine.Exclude == nil {
		if typ, ptr, ok := pointerStructMeta(value); ok {
			return m.engine.MarshalTypedPtr(nil, typ, ptr)
		}
		if typ, ptr, ok := structValueMeta(value); ok {
			return m.engine.MarshalTypedPtr(nil, typ, ptr)
		}
	}
	return m.engine.Marshal(value)
}
func (m *Marshaller) Wire() (*jsonmarshal.WireShape, error) {
	m.once.Do(func() { m.shape, m.err = m.engine.Wire(m.typeOf) })
	return m.shape, m.err
}
func (cfg Options) marshalEngine() *jsonmarshal.Engine {
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

	return jsonmarshal.New(transform, exclude, cfg.OmitEmpty, cfg.NilSlicePolicy == NilSliceAsNull, cfg.TimeLayout, caseKey, compileName)
}
