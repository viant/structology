package marshal

import (
	"fmt"
	"github.com/viant/structology/encoding/json/internal/tagutil"
	"reflect"
)

// selectedField retains physical ancestry while selection uses the final wire
// name. Exclusion and nil/omitempty checks intentionally happen after selection.
type selectedField struct {
	field    fieldPlan
	parents  []int
	optional bool
	name     string
}

func (e *Engine) selectedFields(plan *structPlan, path []string) ([]selectedField, error) {
	var all []selectedField
	active := map[*structPlan]bool{}
	var collect func(*structPlan, []int, bool) error
	collect = func(p *structPlan, parents []int, optional bool) error {
		if active[p] {
			return fmt.Errorf("recursive inline JSON field")
		}
		active[p] = true
		defer delete(active, p)
		for _, f := range p.fields {
			if f.ignore {
				continue
			}
			if (f.anonymous || f.inline) && !f.inlineRaw {
				typ := f.rType
				nullable := false
				for typ.Kind() == reflect.Pointer {
					nullable = true
					typ = typ.Elem()
				}
				if typ.Kind() == reflect.Struct {
					nested := e.getDynamicPlan(typ, f.representation)
					if nested.inlineIdx < 0 {
						next := append(append([]int(nil), parents...), f.index)
						if err := collect(nested, next, optional || nullable); err != nil {
							return err
						}
						continue
					}
				}
			}
			name := f.name
			if e.hasTransform && !f.explicit {
				name = e.NameTransform(path, name)
			}
			all = append(all, selectedField{f, parents, optional, name})
		}
		return nil
	}
	if err := collect(plan, nil, false); err != nil {
		return nil, err
	}
	candidates := []tagutil.NameCandidate{}
	positions := []int{}
	for i, f := range all {
		if !f.field.anonymous && !f.field.inline {
			candidates = append(candidates, tagutil.NameCandidate{Name: f.name, Depth: len(f.parents), Explicit: f.field.explicit})
			positions = append(positions, i)
		}
	}
	selected, err := tagutil.NameWinners(candidates)
	if err != nil {
		return nil, err
	}
	keep := map[int]bool{}
	for i := range selected {
		keep[positions[i]] = true
	}
	result := make([]selectedField, 0, len(all))
	for i, f := range all {
		if keep[i] || f.field.anonymous || f.field.inline {
			result = append(result, f)
		}
	}
	return result, nil
}

func (e *Engine) appendSelectedObject(sess *encoderSession, rv reflect.Value, plan *structPlan) error {
	selected, err := e.selectedFields(plan, e.currentPath(sess))
	if err != nil {
		return err
	}
	sess.buf = append(sess.buf, '{')
	counter := 0
	for _, item := range selected {
		owner := rv
		for _, index := range item.parents {
			owner = owner.Field(index)
			for owner.Kind() == reflect.Pointer || owner.Kind() == reflect.Interface {
				if owner.IsNil() {
					owner = reflect.Value{}
					break
				}
				owner = owner.Elem()
			}
			if !owner.IsValid() {
				break
			}
		}
		if !owner.IsValid() {
			continue
		}
		owner, ptr := addressableStruct(owner)
		if e.hasTransform || e.hasExclude {
			single := structPlan{fields: []fieldPlan{item.field}, inlineIdx: -1}
			if err = e.appendStructFieldsDynamic(sess, owner, &single, ptr, &counter); err != nil {
				return err
			}
		} else if err = compileStaticFieldOp(item.field)(e, sess, ptr, &counter); err != nil {
			return err
		}
	}
	sess.buf = append(sess.buf, '}')
	return nil
}
