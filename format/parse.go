package format

import (
	ftime "github.com/viant/structology/format/time"
	"time"
)

func (t *Tag) ParseTime(value string) (time.Time, error) {
	return ftime.Parse(t.TimeLayout, value)
}
