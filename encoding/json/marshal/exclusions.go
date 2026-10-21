package marshal

import (
	"fmt"
	"reflect"
	"strings"
)

// ExcludeFields compiles canonical Go-field paths through existing serializer
// plans, so aliases/casing/inline holders use the same output names as Marshal.
// Configure the engine before publishing or invoking it.
func (e *Engine) ExcludeFields(t reflect.Type, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	compiler := exclusionCompiler{engine: e, keys: map[fieldLocation]bool{}}
	for _, path := range paths {
		if err := compiler.resolve(t, strings.Split(path, "."), nil); err != nil {
			return fmt.Errorf("JSON exclusion %q: %w", path, err)
		}
	}
	e.fieldExclusions = compiler.keys
	e.hasExclude = true
	return nil
}

type exclusionCompiler struct {
	engine *Engine
	keys   map[fieldLocation]bool
}

func (c *exclusionCompiler) resolve(t reflect.Type, segments, path []string) error {
	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return fmt.Errorf("field path does not traverse a struct")
	}
	if c.engine.hasCustomMarshalerType(t) {
		return fmt.Errorf("opaque encoder for %s cannot project exclusions", t)
	}
	plan := c.engine.getDynamicPlan(t)
	if err := c.checkInline(t, map[fieldLocation]bool{}, map[reflect.Type]bool{}); err != nil {
		return err
	}
	if len(segments) > 0 && plan.hidden[segments[0]] {
		return nil
	}
	for _, field := range plan.fields {
		if len(segments) == 0 || field.fieldName != segments[0] {
			continue
		}
		if field.ignore {
			return nil
		}
		inline := field.anonymous || field.inline
		if len(segments) == 1 {
			if inline {
				return c.inline(field.rType, path, map[reflect.Type]bool{})
			}
			c.keys[fieldLocation{path: strings.Join(path, "\x00"), owner: field.owner, name: field.fieldName}] = true
			return nil
		}
		if !inline {
			name := field.name
			if c.engine.hasTransform && !field.explicit {
				name = c.engine.NameTransform(path, name)
			}
			path = append(append([]string(nil), path...), name)
		}
		return c.resolve(field.rType, segments[1:], path)
	}
	return fmt.Errorf("field %q is not in the serializer plan for %s", segments[0], t)
}
func (c *exclusionCompiler) inline(t reflect.Type, path []string, active map[reflect.Type]bool) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct || c.engine.hasCustomMarshalerType(t) {
		return fmt.Errorf("opaque inline exclusion for %s", t)
	}
	if active[t] {
		return fmt.Errorf("recursive inline exclusion for %s", t)
	}
	active[t] = true
	defer delete(active, t)
	for _, field := range c.engine.getDynamicPlan(t).fields {
		if field.ignore {
			continue
		}
		if field.anonymous || field.inline {
			if err := c.inline(field.rType, path, active); err != nil {
				return err
			}
			continue
		}
		c.keys[fieldLocation{path: strings.Join(path, "\x00"), owner: field.owner, name: field.fieldName}] = true
	}
	return nil
}

type fieldLocation struct {
	path  string
	owner reflect.Type
	name  string
}

func (e *Engine) excludeField(path []string, field fieldPlan) bool {
	return e.fieldExclusions[fieldLocation{path: strings.Join(path, "\x00"), owner: field.owner, name: field.fieldName}] || e.Exclude != nil && e.Exclude(path, field.fieldName)
}

// Repeated inline occurrences of one declaring field cannot be distinguished
// by the native field identity at the same wire path. Reject such exclusions.
func (c *exclusionCompiler) checkInline(t reflect.Type, seen map[fieldLocation]bool, active map[reflect.Type]bool) error {
	if active[t] {
		return fmt.Errorf("recursive inline exclusion for %s", t)
	}
	active[t] = true
	defer delete(active, t)
	for _, field := range c.engine.getDynamicPlan(t).fields {
		if field.ignore {
			continue
		}
		if field.anonymous || field.inline {
			child := field.rType
			for child.Kind() == reflect.Pointer {
				child = child.Elem()
			}
			if child.Kind() != reflect.Struct {
				continue
			}
			if err := c.checkInline(child, seen, active); err != nil {
				return err
			}
		} else {
			key := fieldLocation{owner: field.owner, name: field.fieldName}
			if seen[key] {
				return fmt.Errorf("ambiguous inline exclusion identity %s.%s", field.owner, field.fieldName)
			}
			seen[key] = true
		}
	}
	return nil
}
