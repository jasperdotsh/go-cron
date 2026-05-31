// Package cron provides a parser for flexible cron expressions.
// It translates expressions into bitmasks.
package cron

import (
	"fmt"
	"strconv"
	"strings"
)

type Field int

const (
	Minute Field = iota
	Hour
	DayOfMonth
	Month
	DayOfWeek
	Seconds
)

func (f Field) Valid() bool {
	return f >= Minute && f <= Seconds
}

var cronBounds = [6]valueRange{
	{0, 59}, // 0: Minute
	{0, 23}, // 1: Hour
	{1, 31}, // 2: Day of Month
	{1, 12}, // 3: Month
	{0, 7},  // 4: Day of Week (0=Sunday, 6=Saturday)
	{0, 59}, // 5: Seconds
}

var monthMapping = map[string]int{
	"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
	"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
}

var weekdayMapping = map[string]int{
	"SUN": 0, "MON": 1, "TUE": 2, "WED": 3, "THU": 4, "FRI": 5, "SAT": 6,
}

type valueRange struct {
	from int
	to   int
}

type Schedule map[Field]uint64

type Parser struct {
	cfg *config
}

// Parse parses the given expression based on the fields and macros provided in the config.
// Each field is parsed according do this grammar (in EBNF):
//
//	field        = item , { "," , item } ;
//	item         = atom , [ "/" , step ] ;
//	atom         = "*" | value | value , "-" , value ;
//	step         = number ;
//	value        = number | name ;
//	name         = month-name | weekday-name ;
//	month-name   = "JAN" | "FEB" | "MAR" | "APR" | "MAY" | "JUN"
//	             | "JUL" | "AUG" | "SEP" | "OCT" | "NOV" | "DEC" ;
//	weekday-name = "SUN" | "MON" | "TUE" | "WED" | "THU" | "FRI" | "SAT" ;
//	number       = digit , { digit } ;
//	digit        = "0" | "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" ;
func (p *Parser) Parse(expr string) (Schedule, error) {
	if p.cfg == nil {
		return nil, fmt.Errorf("parsing cron: nil config for parser")
	}

	expr = strings.TrimSpace(expr)

	for _, macro := range p.cfg.macros {
		if expr == macro.Name {
			expr = macro.Value
			break
		}
	}

	fields := strings.Fields(expr)

	if len(fields) != len(p.cfg.fields) {
		return nil, fmt.Errorf(
			"parsing cron: fields mismatch: expected %d, got %d",
			len(p.cfg.fields), len(fields),
		)
	}

	sched := make(Schedule, len(fields))

	for i, fieldText := range fields {
		parsed, err := p.parseField(strings.TrimSpace(fieldText), p.cfg.fields[i])
		if err != nil {
			return nil, fmt.Errorf("parsing cron: %w", err)
		}

		sched[p.cfg.fields[i]] = parsed
	}

	return sched, nil
}

// parseField turns one field (like "1,15,30" or "*/5") into a bitmap, where
// bit n is set when value n is selected. Items are split on commas and ORd
// together, so each comma adds more allowed values to the same field.
func (p *Parser) parseField(text string, field Field) (bitmap uint64, err error) {
	rawItems := strings.Split(text, ",")

	items := make([]itemSpec, len(rawItems))

	for idx, item := range rawItems {
		items[idx], err = p.parseItem(item, field)
		if err != nil {
			return 0, fmt.Errorf("parsing field: %w", err)
		}
	}

	for _, item := range items {
		if item.step <= 0 {
			return bitmap, fmt.Errorf("parsing field: step (%d) <= 0", item.step)
		}

		for i := item.from; i <= item.to; i += item.step {
			pos := i

			// support 7 as weekday index by wrapping back to 0
			if field == DayOfWeek && pos == 7 {
				pos = 0
			}

			bitmap = bitmap | 1<<pos
		}
	}

	return
}

type itemSpec struct {
	step int
	valueRange
}

// parseItem parses a single item: an atom with an optional "/step" suffix.
// The atom gives the range and the step is how far to count within it, so
// "0-30/10" means every 10th value from 0 to 30.
//
//	item         = atom , [ "/" , step ] ;
func (p *Parser) parseItem(item string, field Field) (itemSpec, error) {
	parts := strings.Split(item, "/")

	atomBounds, err := p.parseAtom(parts[0], field)
	if err != nil {
		return itemSpec{}, fmt.Errorf("parsing item: %w", err)
	}

	if len(parts) > 2 {
		return itemSpec{}, fmt.Errorf("parsing item: unexpected '/' character")
	}

	step := 1
	if len(parts) == 2 {
		step, err = strconv.Atoi(parts[1])
	}

	return itemSpec{valueRange: atomBounds, step: step}, err
}

// parseAtom resolves the range an item covers: "*" spans the field's full
// bounds, a single value is a range of one, and "a-b" is everything from a to
// b. It also makes sure the range stays in order and within the field's limits.
//
//	atom         = "*" | value | value , "-" , value
func (p *Parser) parseAtom(atom string, field Field) (r valueRange, err error) {
	bounds := cronBounds[field]

	if atom == "*" {
		return bounds, nil
	}

	parts := strings.Split(atom, "-")
	if len(parts) > 2 {
		return r, fmt.Errorf("parsing atom: unexpected '-' character")
	}

	r.from, err = p.parseValue(parts[0])
	if err != nil {
		return r, fmt.Errorf("parsing atom: %w", err)
	}
	r.to = r.from

	if len(parts) == 2 {
		r.to, err = p.parseValue(parts[1])
		if err != nil {
			return r, fmt.Errorf("parsing atom: %w", err)
		}
		if r.from > r.to {
			return r, fmt.Errorf("parsing atom: invalid range: out of order (%d-%d)", r.from, r.to)
		}
	}

	if r.from < bounds.from || r.to > bounds.to {
		return r, fmt.Errorf("parsing atom: value out of bounds [%d-%d] for this field", bounds.from, bounds.to)
	}

	return r, nil
}

// parseValue reads a single value, either a plain number or a three-letter
// name like "JAN" or "MON" (case-insensitive). Names map to their numeric
// equivalent; anything else is an error.
//
//	value        = number | name ;
//	name         = month-name | weekday-name ;
func (p *Parser) parseValue(value string) (int, error) {
	val, err := strconv.Atoi(value)
	if err == nil {
		return val, nil
	}

	value = strings.ToUpper(value)

	if val, ok := monthMapping[value]; ok {
		return val, nil
	}

	if val, ok := weekdayMapping[value]; ok {
		return val, nil
	}

	return 0, fmt.Errorf("parsing value: invalid value '%v'", value)
}
