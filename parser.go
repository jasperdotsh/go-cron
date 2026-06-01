// Package cron parses standard five-field cron expressions (Minute Hour
// DayOfMonth Month DayOfWeek) into schedules and reports their next/prev run timestamp.
//
// Each field of an expression becomes a bitmap of allowed values.
// The day is checked by DayOfMonth AND DayOfWeek together; when both are restricted,
// a day matches if either does, otherwise the restricted field applies.
package cron

import (
	"fmt"
	"strconv"
	"strings"
)

// field identifies one component of a cron expression.
// The values are ordered as the fields appear in an expression.
type field int

const (
	minute field = iota
	hour
	dayOfMonth
	month
	dayOfWeek
)

var fieldNames = [...]string{
	minute:     "Minute",
	hour:       "Hour",
	dayOfMonth: "DayOfMonth",
	month:      "Month",
	dayOfWeek:  "DayOfWeek",
}

const numFields = len(fieldNames)

func (f field) String() string {
	return fieldNames[f]
}

type valueRange struct{ from, to int }

// cronBounds gives the inclusive range of legal values for each field.
// dayOfWeek allows 7 as an alias for Sunday (0).
var cronBounds = [numFields]valueRange{
	minute:     {0, 59},
	hour:       {0, 23},
	dayOfMonth: {1, 31},
	month:      {1, 12},
	dayOfWeek:  {0, 7},
}

var monthMapping = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var weekdayMapping = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

// Parse reads expr into a Schedule. The expression must have the five standard
// fields, in order, separated by whitespace. The grammar of each field is:
//
//	field = item , { "," , item } ;
//	item  = atom , [ "/" , step ] ;
//	atom  = "*" | value | value , "-" , value ;
//	value = number | month-name | weekday-name ;
func Parse(expr string) (*Schedule, error) {
	parts := strings.Fields(expr)
	if len(parts) != numFields {
		return nil, fmt.Errorf("cron: expected %d fields, got %d", numFields, len(parts))
	}

	var parsed [numFields]uint64
	for i, text := range parts {
		f := field(i)
		bitmap, err := parseField(text, f)
		if err != nil {
			return nil, fmt.Errorf("cron: %s: %w", f, err)
		}

		parsed[f] = bitmap
	}

	return newSchedule(parsed), nil
}

// parseField turns one field such as "1,15,30" or "*/5" into a bitmap where
// bit n is set when value n is selected. Comma-separated items are ORed.
func parseField(text string, f field) (uint64, error) {
	var bitmap uint64
	for item := range strings.SplitSeq(text, ",") {
		spec, err := parseItem(item, f)
		if err != nil {
			return 0, err
		}

		for v := spec.from; v <= spec.to; v += spec.step {
			pos := v

			if f == dayOfWeek && pos == 7 {
				pos = 0 // 7 is an alias for Sunday
			}

			bitmap |= 1 << pos
		}
	}

	return bitmap, nil
}

type itemSpec struct {
	valueRange
	step int
}

// parseItem parses an atom with an optional "/step" suffix. "0-30/10" selects
// every tenth value from 0 to 30. A bare value with a step, such as "2/2", runs
// from that value to the field maximum. An explicit range or wildcard keeps its own bounds.
func parseItem(item string, f field) (itemSpec, error) {
	atom, step, hasStep := strings.Cut(item, "/")

	r, err := parseAtom(atom, f)
	if err != nil {
		return itemSpec{}, err
	}

	n := 1
	if hasStep {
		if n, err = strconv.Atoi(step); err != nil || n <= 0 {
			return itemSpec{}, fmt.Errorf("invalid step %q", step)
		}

		if atom != "*" && !strings.Contains(atom, "-") {
			r.to = cronBounds[f].to
		}
	}

	return itemSpec{valueRange: r, step: n}, nil
}

// parseAtom resolves the range an item covers: "*" is the field's full bounds,
// a single value is a range of one, and "a-b" spans a to b. The range must be
// ordered and within the field's bounds.
func parseAtom(atom string, f field) (valueRange, error) {
	b := cronBounds[f]
	if atom == "*" {
		return b, nil
	}

	lo, hi, isRange := strings.Cut(atom, "-")
	from, err := parseValue(lo, f)
	if err != nil {
		return valueRange{}, err
	}

	to := from
	if isRange {
		if to, err = parseValue(hi, f); err != nil {
			return valueRange{}, err
		}

		if from > to {
			return valueRange{}, fmt.Errorf("range out of order: %d-%d", from, to)
		}
	}

	if from < b.from || to > b.to {
		return valueRange{}, fmt.Errorf("value out of range [%d,%d]", b.from, b.to)
	}

	return valueRange{from, to}, nil
}

// parseValue reads a single value: a number, or a three-letter month or weekday name (case-insensitive).
func parseValue(s string, f field) (int, error) {
	// strconv.Atoi accepts a leading sign ("+5", "-5"), but cron values are
	// bare unsigned numbers, so reject anything that does not start with a digit.
	if n, err := strconv.Atoi(s); err == nil && s[0] != '+' && s[0] != '-' {
		return n, nil
	}

	name := strings.ToUpper(s)
	if f == month {
		if n, ok := monthMapping[name]; ok {
			return n, nil
		}
	}

	if f == dayOfWeek {
		if n, ok := weekdayMapping[name]; ok {
			return n, nil
		}
	}

	return 0, fmt.Errorf("invalid value %s", s)
}
