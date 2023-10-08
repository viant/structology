package time

import (
	"strings"
	"time"
)

var iso20220715DateFormatToRfc3339TimeLayoutReplacer = strings.NewReplacer(
	"YYYY", "2006",
	"MM", "01",
	"M", "1",
	"DD", "02",
	"D", "2",
	"+hh:mm", "Z07:00",
	"+hhmm", "Z0700",
	"+hh", "Z07",
	"-hh:mm", "Z07:00",
	"-hhmm", "Z0700",
	"hh", "15",
	"mm", "04",
	"m", "4",
	"ss", "05",
	".SSS", ".999",
	".SS", ".99",
	".S", ".9",
	"-hh", "Z07",
	"Z", "Z07:00",
)

// DateFormatToTimeLayout converts ISO 2022-07-15 date format to RFC3339 time layout
func DateFormatToTimeLayout(dateFormat string) string {
	return iso20220715DateFormatToRfc3339TimeLayoutReplacer.Replace(dateFormat)
}

func Parse(layout, value string) (time.Time, error) {
	if layout == "" {
		layout = time.RFC3339 //TODO add layout autodetection for this case
	}
	t, err := time.Parse(layout, value)
	if err != nil {
		if layout != "" && len(value) > len(layout) {
			value = value[:len(layout)]
			t, err = time.Parse(value, layout)
		}
	}
	return t, err
}
