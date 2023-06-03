package structology

import "reflect"

//GenMarkerFields generate marker struct fields
func GenMarkerFields(t reflect.Type) []reflect.StructField {
	var result []reflect.StructField
	if t = ensureStruct(t); t == nil {
		return result
	}
	boolType := reflect.TypeOf(true)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if IsSetMarker(field.Tag) {
			continue
		}
		result = append(result, reflect.StructField{Name: field.Name, PkgPath: t.PkgPath(), Type: boolType})
	}
	return result
}
