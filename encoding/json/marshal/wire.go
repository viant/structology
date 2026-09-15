package marshal

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// WireShape is immutable JSON metadata from this engine's actual field plans.
// It is neither a Go type builder nor a schema dialect.
type WireShape struct {
	source     reflect.Type
	kind       reflect.Kind
	nullable   bool
	length     int
	format     string
	properties []WireProperty
	element    *WireShape
}
type WireProperty struct {
	field    reflect.StructField
	name     string
	required bool
	shape    *WireShape
}

func (s *WireShape) Source() reflect.Type       { return s.source }
func (s *WireShape) Kind() reflect.Kind         { return s.kind }
func (s *WireShape) Nullable() bool             { return s.nullable }
func (s *WireShape) Length() int                { return s.length }
func (s *WireShape) Format() string             { return s.format }
func (s *WireShape) Properties() []WireProperty { return append([]WireProperty(nil), s.properties...) }
func (s *WireShape) Element() *WireShape        { return s.element }

// Field returns provenance indexed from the containing WireShape.Source,
// including anonymous/inline ancestors. Returned indexes are detached.
func (p WireProperty) Field() reflect.StructField {
	field := p.field
	field.Index = append([]int(nil), field.Index...)
	return field
}
func (p WireProperty) Name() string      { return p.name }
func (p WireProperty) Required() bool    { return p.required }
func (p WireProperty) Shape() *WireShape { return p.shape }

// Wire projects existing plans and hooks without sampling any output values.
func (e *Engine) Wire(t reflect.Type) (*WireShape, error) {
	return (&wireCompiler{engine: e, active: map[wireKey]*WireShape{}, inline: map[reflect.Type]bool{}}).value(t, nil)
}

type wireCompiler struct {
	engine *Engine
	active map[wireKey]*WireShape
	inline map[reflect.Type]bool
}

func (c *wireCompiler) value(t reflect.Type, path []string) (*WireShape, error) {
	if t == nil {
		return nil, fmt.Errorf("wire type is required")
	}
	if t.Kind() == reflect.Pointer {
		item, err := c.value(t.Elem(), path)
		if err != nil {
			return nil, err
		}
		return &WireShape{source: t, kind: reflect.Pointer, nullable: true, element: item, length: -1}, nil
	}
	if c.engine.hasCustomMarshalerType(t) {
		shape := customWireShape(t)
		pointer := reflect.PointerTo(t)
		if t.Implements(gojayObjectType) || pointer.Implements(gojayObjectType) || t.Implements(gojayArrayType) || pointer.Implements(gojayArrayType) {
			shape.kind, shape.nullable = reflect.Interface, true
		}
		return shape, nil
	}
	if t == timeType {
		return c.time(t, c.engine.timeLayout), nil
	}
	key := c.key(t, path)
	if prior := c.active[key]; prior != nil {
		if c.engine.Exclude != nil || c.engine.hasTransform {
			return nil, fmt.Errorf("path-dependent recursive wire type %s is not representable", t)
		}
		return prior, nil
	}
	result := &WireShape{source: t, kind: t.Kind(), length: -1}
	c.active[key] = result
	defer delete(c.active, key)
	switch t.Kind() {
	case reflect.Struct:
		plan := c.engine.getDynamicPlan(t)
		if plan.inlineIdx >= 0 {
			return nil, fmt.Errorf("whole-value inline JSON requires explicit wire authority: %s", t)
		}
		if err := c.fields(result, plan, path, nil, false); err != nil {
			return nil, err
		}
	case reflect.Slice, reflect.Array:
		result.nullable = t.Kind() == reflect.Slice && c.engine.nilSliceNull
		if t.Kind() == reflect.Array {
			result.length = t.Len()
		}
		item, err := c.value(t.Elem(), path)
		if err != nil {
			return nil, err
		}
		result.element = item
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("map key %s needs explicit wire encoding", t.Key())
		}
		element := t.Elem()
		for element.Kind() == reflect.Pointer {
			element = element.Elem()
		}
		if (c.engine.hasExclude || c.engine.hasTransform) && (element.Kind() == reflect.Struct || element.Kind() == reflect.Map || element.Kind() == reflect.Slice || element.Kind() == reflect.Array) {
			return nil, fmt.Errorf("dynamic map value paths require explicit wire authority")
		}
		item, err := c.value(t.Elem(), path)
		if err != nil {
			return nil, err
		}
		result.element = item
		result.nullable = true
	case reflect.Interface:
		return nil, fmt.Errorf("dynamic interface %s requires explicit wire authority", t)
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr, reflect.Float32, reflect.Float64:
	default:
		return nil, fmt.Errorf("unsupported JSON wire type %s", t)
	}
	return result, nil
}

func (c *wireCompiler) fields(target *WireShape, plan *structPlan, path []string, indexes []int, optional bool) error {
	for _, field := range plan.fields {
		if field.ignore {
			continue
		}
		if field.anonymous || field.inline {
			if field.inlineRaw {
				return fmt.Errorf("opaque inline JSON field %s", field.fieldName)
			}
			base := field.rType
			nullable := false
			for base.Kind() == reflect.Pointer {
				nullable = true
				base = base.Elem()
			}
			if base.Kind() != reflect.Struct {
				return fmt.Errorf("non-object inline field %s is not representable", field.fieldName)
			}
			if c.inline[base] {
				return fmt.Errorf("recursive inline field %s", field.fieldName)
			}
			c.inline[base] = true
			nested := c.engine.getDynamicPlan(base)
			if nested.inlineIdx >= 0 {
				return fmt.Errorf("whole-value inline JSON field %s", field.fieldName)
			}
			err := c.fields(target, nested, path, append(append([]int(nil), indexes...), field.index), optional || nullable)
			delete(c.inline, base)
			if err != nil {
				return err
			}
			continue
		}
		if c.engine.excludeField(path, field) {
			continue
		}
		name := field.name
		if c.engine.hasTransform && !field.explicit {
			name = c.engine.NameTransform(path, name)
		}
		for _, prior := range target.properties {
			if prior.name == name {
				return fmt.Errorf("JSON name collision %q", name)
			}
		}
		childPath := path
		if c.engine.hasTransform || c.engine.hasExclude {
			childPath = append(append([]string(nil), path...), name)
		}
		item, err := c.value(field.rType, childPath)
		if err != nil {
			return fmt.Errorf("field %s: %w", field.fieldName, err)
		}
		if isTimeTypeOrPtr(field.rType) {
			layout := c.engine.timeLayout
			if field.timeLayout != "" {
				layout = field.timeLayout
			}
			item = c.time(field.rType, layout)
		}
		if field.nullable {
			copy := *item
			copy.nullable = true
			item = &copy
		}
		source := field.owner.Field(field.index)
		source.Index = append(append([]int(nil), indexes...), source.Index...)
		target.properties = append(target.properties, WireProperty{field: source, name: name, required: !optional && !field.omitempty && !c.engine.omitEmpty, shape: item})
	}
	return nil
}
func (c *wireCompiler) time(t reflect.Type, layout string) *WireShape {
	result := &WireShape{source: t, kind: reflect.String, length: -1, nullable: t.Kind() == reflect.Pointer}
	switch layout {
	case time.RFC3339, time.RFC3339Nano:
		result.format = "date-time"
	case "2006-01-02":
		result.format = "date"
	}
	return result
}

type wireKey struct {
	typeOf reflect.Type
	scope  string
}

func (c *wireCompiler) key(t reflect.Type, path []string) wireKey {
	scope := ""
	prefix := strings.Join(path, "\x00")
	for location := range c.engine.fieldExclusions {
		if prefix == "" || location.path == prefix || strings.HasPrefix(location.path, prefix+"\x00") {
			scope = "scoped:" + prefix
			break
		}
	}
	// Arbitrary callbacks cannot establish finite recursive path equivalence.
	// They retain the conservative recursive-type error above.
	return wireKey{typeOf: t, scope: scope}
}
