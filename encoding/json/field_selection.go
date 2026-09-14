package json

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/viant/structology/encoding/json/internal/tagutil"
	xshape "github.com/viant/x/shape"
)

// FieldSelection includes canonical Go fields at a canonical Go holder path.
// An empty Fields list selects no properties at that object. Omitted scopes
// leave their objects unrestricted. Selection never examines row values.
type FieldSelection struct {
	Path    []string
	Fields  []string
	Indexes [][]int
}

type fieldIndexNode struct {
	children map[int]*fieldIndexNode
	excluded bool
}
type fieldScope struct {
	indexes  fieldIndexNode
	excluded map[string]bool
	children map[string]*fieldScope
}

// FieldFilter resolves typed field selections to encoder paths. It retains exact
// Go indexes for standard JSON and is immutable after construction.
type canonicalSelection struct {
	holder []int
	fields [][]int
}
type FieldFilter struct {
	canonical  []canonicalSelection
	root       fieldScope
	typeOf     reflect.Type
	selections []FieldSelection
}

// WithOptions resolves parent paths using the transformed encoder's tag and
// naming authority. The original standard filter remains immutable.
func (f *FieldFilter) WithOptions(opts ...Option) (*FieldFilter, error) {
	cfg := resolveOptions(context.Background(), opts)
	result := &FieldFilter{typeOf: f.typeOf, selections: f.selections, canonical: f.canonical}
	return result, result.compile(&cfg)
}

func NewFieldFilter(t reflect.Type, selections []FieldSelection) (*FieldFilter, error) {
	result := &FieldFilter{typeOf: t}
	for _, scope := range selections {
		copy := FieldSelection{Path: append([]string(nil), scope.Path...), Fields: append([]string(nil), scope.Fields...)}
		for _, index := range scope.Indexes {
			copy.Indexes = append(copy.Indexes, append([]int(nil), index...))
		}
		result.selections = append(result.selections, copy)
	}
	if err := result.compileCanonical(); err != nil {
		return nil, err
	}
	return result, result.compile(nil)
}
func (result *FieldFilter) compile(cfg *Options) error {
	t := result.typeOf
	selections := result.selections
selectionLoop:
	for _, selection := range selections {
		current := t
		var path []string
		for _, holder := range selection.Path {
			fields, err := result.fields(current, cfg)
			if err != nil {
				return err
			}
			found := false
			for _, field := range fields {
				if field.Field.Name != holder {
					continue
				}
				name := field.Name
				if cfg != nil {
					resolved := tagutil.ResolveFieldTag(field.Field.StructField())
					name = resolved.Name
					if !resolved.Explicit {
						if cfg.PathName != nil {
							name = cfg.PathName.TransformPath(path, name)
						} else if cfg.NameTransformer != nil {
							name = cfg.NameTransformer.Transform(strings.Join(path, "."), name)
						}
					}
				}
				path = append(path, name)
				current = field.Field.ReflectedType
				found = true
				break
			}
			if !found {
				// A canonical holder hidden by JSON dominance or json:"-" has
				// no wire descendants to filter. Unknown holders remain errors.
				typ := current
				for typ != nil && (typ.Kind() == reflect.Pointer || typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array) {
					typ = typ.Elem()
				}
				structural, err := xshape.Linked(typ).Fields()
				if err != nil {
					return err
				}
				for _, field := range structural {
					if field.Name == holder {
						continue selectionLoop
					}
				}
				return fmt.Errorf("JSON selection holder %q is not exposed on %v", holder, current)
			}
		}
		fields, err := result.fields(current, cfg)
		if err != nil {
			return err
		}
		include := map[string]bool{}
		for _, name := range selection.Fields {
			include[name] = true
		}
		excluded := map[string]bool{}
		seen := map[string]bool{}
		indexes := fieldIndexNode{}
		for _, field := range fields {
			selected := include[field.Field.Name]
			for _, index := range selection.Indexes {
				if reflect.DeepEqual(index, field.Field.Index) {
					selected = true
					break
				}
			}
			node := &indexes
			for _, part := range field.Field.Index {
				if node.children == nil {
					node.children = map[int]*fieldIndexNode{}
				}
				child := node.children[part]
				if child == nil {
					child = &fieldIndexNode{}
					node.children[part] = child
				}
				node = child
			}
			node.excluded = !selected
			if cfg != nil && seen[field.Field.Name] && excluded[field.Field.Name] != !selected {
				return fmt.Errorf("JSON field selection cannot distinguish promoted fields named %q", field.Field.Name)
			}
			seen[field.Field.Name] = true
			excluded[field.Field.Name] = excluded[field.Field.Name] || !selected
		}
		scope := &result.root
		for _, part := range path {
			if scope.children == nil {
				scope.children = map[string]*fieldScope{}
			}
			next := scope.children[part]
			if next == nil {
				next = &fieldScope{}
				scope.children[part] = next
			}
			scope = next
		}
		if scope.excluded != nil {
			return fmt.Errorf("duplicate JSON selection scope %q", path)
		}
		scope.excluded = excluded
		scope.indexes = indexes
	}
	return nil
}

func (f *FieldFilter) fields(t reflect.Type, cfg *Options) ([]xshape.JSONField, error) {
	for t != nil && (t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array) {
		t = t.Elem()
	}
	fields, err := xshape.Linked(t).JSONFields()
	if err != nil || cfg == nil {
		return fields, err
	}
	native := map[string]bool{}
	if err = f.transformedIndexes(t, nil, native, map[reflect.Type]bool{}); err != nil {
		return nil, err
	}
	if len(native) != len(fields) {
		return nil, fmt.Errorf("field selection requires matching standard and transformed JSON field visibility on %v", t)
	}
	for _, field := range fields {
		if !native[fmt.Sprint(field.Field.Index)] {
			return nil, fmt.Errorf("field selection requires matching standard and transformed JSON promotion on %v", t)
		}
	}
	return fields, nil
}

func (f *FieldFilter) ExcludePath(path []string, field string) bool {
	if f == nil {
		return false
	}
	scope := &f.root
	for _, part := range path {
		scope = scope.children[part]
		if scope == nil {
			return false
		}
	}
	return scope.excluded[field]
}

// ExcludeField retains the exact JSON field index when promoted fields share a
// Go name. Standard encoding consumes this alongside PathFieldExcluder.
func (f *FieldFilter) ExcludeField(path []string, index []int) bool {
	if f == nil {
		return false
	}
	scope := &f.root
	for _, part := range path {
		scope = scope.children[part]
		if scope == nil {
			return false
		}
	}
	node := &scope.indexes
	for _, part := range index {
		node = node.children[part]
		if node == nil {
			return false
		}
	}
	return node.excluded
}

// transformedIndexes checks that a path filter has the same field identities in
// both native engines. Structural inline/dominance differences fail closed.
func (f *FieldFilter) transformedIndexes(t reflect.Type, prefix []int, result map[string]bool, active map[reflect.Type]bool) error {
	for t != nil && t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil
	}
	if active[t] {
		return fmt.Errorf("recursive inline selection on %v", t)
	}
	active[t] = true
	defer delete(active, t)
	fields, err := xshape.Linked(t).Fields()
	if err != nil {
		return err
	}
	for _, field := range fields {
		if len(field.Index) != 1 || !field.Exported || field.Tag.Get("setMarker") == "true" {
			continue
		}
		tag := tagutil.ResolveFieldTag(field.StructField())
		if tag.Ignore {
			continue
		}
		index := append(append([]int(nil), prefix...), field.Index...)
		if field.Anonymous || tag.Inline {
			if err := f.transformedIndexes(field.ReflectedType, index, result, active); err != nil {
				return err
			}
			continue
		}
		result[fmt.Sprint(index)] = true
	}
	return nil
}

// compileCanonical resolves the already-authored holder paths through native
// shape indexes. It does not enumerate row values or derive fields from JSON.
func (f *FieldFilter) compileCanonical() error {
	for _, scope := range f.selections {
		current := f.typeOf
		canonical := canonicalSelection{}
		for _, name := range scope.Path {
			fields, err := xshape.Linked(current).Fields()
			if err != nil {
				return err
			}
			found := false
			for _, field := range fields {
				if field.Name == name {
					canonical.holder = append(canonical.holder, field.Index...)
					current = field.ReflectedType
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("selection holder %q is missing on %v", name, current)
			}
		}
		canonical.fields = append(canonical.fields, scope.Indexes...)
		fields, err := xshape.Linked(current).Fields()
		if err != nil {
			return err
		}
		for _, name := range scope.Fields {
			for _, field := range fields {
				if field.Name == name {
					canonical.fields = append(canonical.fields, field.Index)
					break
				}
			}
		}
		f.canonical = append(f.canonical, canonical)
	}
	return nil
}

// ExcludeIndexes applies the selection to a canonical structural field index.
// Container elements do not add indexes. The longest holder scope wins, so two
// occurrences of one row type remain independent. Anonymous holder prefixes
// stay present when they contain a selected promoted field.
func (f *FieldFilter) ExcludeIndexes(index []int) bool {
	var selected *canonicalSelection
	for i := range f.canonical {
		scope := &f.canonical[i]
		if len(index) > len(scope.holder) && f.indexPrefix(index, scope.holder) && (selected == nil || len(scope.holder) > len(selected.holder)) {
			selected = scope
		}
	}
	if selected == nil {
		return false
	}
	relative := index[len(selected.holder):]
	for _, field := range selected.fields {
		if f.indexPrefix(relative, field) || f.indexPrefix(field, relative) {
			return false
		}
	}
	return true
}
func (*FieldFilter) indexPrefix(path, prefix []int) bool {
	if len(prefix) > len(path) {
		return false
	}
	for i, part := range prefix {
		if path[i] != part {
			return false
		}
	}
	return true
}

// AffectsIndexes bounds presentation compilation to the selected holder graph.
// Fully included branches retain their original types and custom codec methods.
func (f *FieldFilter) AffectsIndexes(index []int) bool {
	for _, scope := range f.canonical {
		if f.indexPrefix(scope.holder, index) {
			return true
		}
		if !f.indexPrefix(index, scope.holder) {
			continue
		}
		relative := index[len(scope.holder):]
		complete := false
		for _, field := range scope.fields {
			if f.indexPrefix(relative, field) {
				complete = true
				break
			}
		}
		if !complete {
			return true
		}
	}
	return false
}
