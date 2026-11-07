package marshal

import (
	"reflect"
	"strconv"
	"time"
)

// appendFormattedTimeField retains field-local layout when reflective static
// traversal reaches nested rows. The compiled static ops and dynamic traversal
// already honor this metadata. Unformatted/custom types keep their existing path.
func appendFormattedTimeField(session *encoderSession, field *fieldPlan, value reflect.Value) bool {
	if field.timeLayout == "" {
		return false
	}
	if field.rType == timeType {
		timestamp := value.Interface().(time.Time)
		if field.nullable && timestamp.IsZero() {
			session.buf = append(session.buf, "null"...)
		} else {
			session.buf = strconv.AppendQuote(session.buf, timestamp.Format(field.timeLayout))
		}
		return true
	}
	if field.rType.Kind() != reflect.Ptr || field.rType.Elem() != timeType {
		return false
	}
	if value.IsNil() {
		session.buf = append(session.buf, "null"...)
		return true
	}
	session.buf = strconv.AppendQuote(session.buf, value.Elem().Interface().(time.Time).Format(field.timeLayout))
	return true
}
