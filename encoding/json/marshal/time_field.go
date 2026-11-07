package marshal

import (
	"github.com/viant/tagly/format"
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
			session.buf = strconv.AppendQuote(session.buf, formatFieldTime(field.timeFormat, timestamp, field.timeLayout))
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
	timestamp := value.Elem().Interface().(time.Time)
	if field.nullable && timestamp.IsZero() {
		session.buf = append(session.buf, "null"...)
	} else {
		session.buf = strconv.AppendQuote(session.buf, formatFieldTime(field.timeFormat, timestamp, field.timeLayout))
	}
	return true
}

func formatFieldTime(policy *format.Tag, value time.Time, layout string) string {
	if policy == nil || policy.Timezone == "" {
		return value.Format(layout)
	}
	// Copy field metadata to avoid mutating a shared compiled plan when a
	// directly constructed tag has not yet resolved its timezone.
	copy := *policy
	copy.TimeLayout = layout
	return copy.FormatTime(&value)
}
