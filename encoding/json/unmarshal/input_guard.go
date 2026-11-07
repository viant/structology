package unmarshal

import (
	"github.com/viant/tagly/format"
	"github.com/viant/x/shape"
	"reflect"
	"strings"
	"sync"
)

type guardShape = format.InputScopePlan
type guardShapeKey struct {
	typ      reflect.Type
	metadata *shape.Representation
}
type guardValidation struct {
	active bool
	err    error
}

var guardValidationCache sync.Map

// ValidateInputGuards validates ancestor references from the complete root shape,
// independently of request contents. It never reads JSON or mutates metadata.
func ValidateInputGuards(typ reflect.Type, metadata *shape.Representation) (bool, error) {
	key := guardShapeKey{typ, metadata}
	if cached, ok := guardValidationCache.Load(key); ok {
		r := cached.(guardValidation)
		return r.active, r.err
	}
	if !hasDeclaredFieldPolicy(typ, func(f reflect.StructField) bool { return guardPolicyDeclared(f.Tag) }, metadata) {
		guardValidationCache.Store(key, guardValidation{})
		return false, nil
	}
	memo := map[guardShapeKey]*guardShape{}
	maxDepth := 0
	var build func(reflect.Type, *shape.Representation) (*guardShape, error)
	build = func(t reflect.Type, m *shape.Representation) (*guardShape, error) {
		if t == nil {
			return nil, nil
		}
		k := guardShapeKey{t, m}
		if p := memo[k]; p != nil {
			return p, nil
		}
		p := &guardShape{}
		memo[k] = p
		if hasCustomUnmarshalType(t) {
			return p, nil
		}
		switch t.Kind() {
		case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
			var child *shape.Representation
			if m != nil {
				child = m.Elem()
			}
			q, err := build(t.Elem(), child)
			p.Element = q
			return p, err
		case reflect.Struct:
			fields, err := shape.Linked(t).JSONFields()
			if m != nil {
				fields, err = m.JSONFields()
			}
			if err != nil {
				return nil, err
			}
			p.Scope = format.InputScope{}
			for _, f := range fields {
				if f.Field.Tag.Get("internal") == "true" || f.Field.Tag.Get("setMarker") == "true" {
					continue
				}
				p.Scope[f.Field.Name] = false
				policy, err := declaredInputPolicy(f.Field.Tag)
				if err != nil {
					return nil, err
				}
				guard, err := policy.InputGuard()
				if err != nil {
					return nil, err
				}
				p.Guards = append(p.Guards, guard)
				if guard.Depth > maxDepth {
					maxDepth = guard.Depth
				}
				var child *shape.Representation
				if m != nil {
					child = m.FieldShapeByIndex(f.Field.Index)
				}
				q, err := build(f.Field.ReflectedType, child)
				if err != nil {
					return nil, err
				}
				p.Children = append(p.Children, q)
			}
		}
		return p, nil
	}
	root, err := build(typ, metadata)
	if err == nil && maxDepth > 0 {
		err = format.ValidateInputScopePlan(root)
	}
	r := guardValidation{maxDepth > 0, err}
	guardValidationCache.Store(key, r)
	return r.active, r.err
}

func inputPresence(names *format.InputNameMatcher, members []format.InputMember, selected []bool, goNames []string) format.InputScope {
	scope := format.InputScope{}
	for _, name := range goNames {
		if name != "" {
			scope[name] = false
		}
	}
	for i, member := range members {
		if selected != nil && !selected[i] {
			continue
		}
		index, _, excluded := names.Classify(member.Name)
		if !excluded && index >= 0 && index < len(goNames) && goNames[index] != "" {
			scope[goNames[index]] = true
		}
	}
	return scope
}

func guardPolicyDeclared(tag reflect.StructTag) bool {
	return strings.Contains(strings.ToLower(tag.Get("format")), "inputunless")
}
