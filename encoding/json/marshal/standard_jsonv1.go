//go:build !goexperiment.jsonv2

package marshal

import "reflect"

func standardQuotedNamedPointer(reflect.Type, reflect.StructTag) bool { return false }
