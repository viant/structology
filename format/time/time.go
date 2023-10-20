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

var rfc3339TimeLayoutToIso20220715DateFormatReplacer = strings.NewReplacer(
	"2006", "YYYY",
	"Z07:00", "+hh:mm",
	"Z0700", "+hhmm",
	"Z07", "+hh",
	"Z07:00", "-hh:mm",
	"Z0700", "-hhmm",
	"15", "hh",
	"04", "mm",
	"4", "m",
	"05", "ss",
	".999", ".SSS",
	".99", ".SS",
	".9", ".S",
	"Z07", "-hh",
	"Z07:00", "Z",
	"01", "MM",
	"1", "M",
	"02", "DD",
	"2", "D",
)

// DateFormatToTimeLayout converts ISO 2022-07-15 date format to RFC3339 time layout
func DateFormatToTimeLayout(dateFormat string) string {
	return rfc3339TimeLayoutToIso20220715DateFormatReplacer.Replace(dateFormat)
}

// TimeLayoutToDateFormat converts RFC3339 time layout to ISO 2022-07-15 date format
func TimeLayoutToDateFormat(dateFormat string) string {
	return iso20220715DateFormatToRfc3339TimeLayoutReplacer.Replace(dateFormat)
}

func Parse(layout, value string) (time.Time, error) {
	if layout == "" {
		layout = time.RFC3339 //TODO add layout autodetection for this case
	}
	//adjust T fragment
	if strings.Contains(value, "T") != strings.Contains(layout, "T") {
		layout = strings.Replace(layout, "T", " ", 1)
		value = strings.Replace(value, "T", " ", 1)
	}
	t, err := time.ParseInLocation(layout, value, time.UTC)
	if err != nil {
		if len(value) > len(layout) {
			value = value[:len(layout)]
			t, err = time.Parse(layout, value)
		} else {
			layout = layout[:len(value)]
			t, err = time.Parse(layout, value)
		}
	}
	return t, err
}
