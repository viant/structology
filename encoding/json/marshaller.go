package json

import (
	"context"
	jsonmarshal "github.com/viant/structology/encoding/json/marshal"
	"github.com/viant/x/shape"
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
	return m.MarshalContext(context.Background(), value)
}

// MarshalContext encodes with operation-local cancellation on the prepared engine.
func (m *Marshaller) MarshalContext(ctx context.Context, value interface{}) ([]byte, error) {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if m.engine.NameTransform == nil && m.engine.Exclude == nil {
		if typ, ptr, ok := pointerStructMeta(value); ok {
			return m.engine.MarshalTypedPtrContext(ctx, nil, typ, ptr)
		}
		if typ, ptr, ok := structValueMeta(value); ok {
			return m.engine.MarshalTypedPtrContext(ctx, nil, typ, ptr)
		}
	}
	return m.engine.MarshalContext(ctx, value)
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

	return jsonmarshal.New(transform, exclude, cfg.OmitEmpty, cfg.NilSlicePolicy == NilSliceAsNull, cfg.TimeLayout, caseKey, compileName).WithRepresentation(cfg.Representation)
}

// MarshalWithRepresentation retains this marshaller's configured engine while
// applying opt-in metadata to the original instance.
func (m *Marshaller) MarshalWithRepresentation(value any, metadata *shape.Representation) ([]byte, error) {
	return m.engine.WithRepresentation(metadata).Marshal(value)
}

// MarshalWithRepresentationContext applies opt-in metadata and operation-local context.
func (m *Marshaller) MarshalWithRepresentationContext(ctx context.Context, value any, metadata *shape.Representation) ([]byte, error) {
	return m.engine.WithRepresentation(metadata).MarshalContext(ctx, value)
}
