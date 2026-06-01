package cron

import "time"

// searchHorizonYears caps how far Next and Prev scan before giving up, so a
// schedule that can never match (like Feb 30th) won't loop forever.
const searchHorizonYears = 5

// Schedule is a parsed cron expression. Each field is a bitmap of allowed values.
type Schedule struct {
	fields             [numFields]uint64
	dayOfMonthWildcard bool
	dayOfWeekWildcard  bool
}

// newSchedule wraps the parsed bitmaps and notes which day fields are
// wildcards (needed by dayMatches)
func newSchedule(parsed [numFields]uint64) *Schedule {
	s := Schedule{fields: parsed}
	s.dayOfMonthWildcard = s.fields[dayOfMonth] == wildcard(dayOfMonth)
	s.dayOfWeekWildcard = s.fields[dayOfWeek] == wildcard(dayOfWeek)
	return &s
}

// dayMatches applies the day-of-month/day-of-week rule: OR when both are
// restricted, otherwise AND (a wildcard adds no constraint).
func (s *Schedule) dayMatches(t time.Time) bool {
	dayOfMonthMatches := s.fields[dayOfMonth]&bit(t.Day()) != 0
	dayOfWeekMatches := s.fields[dayOfWeek]&bit(int(t.Weekday())) != 0
	if !s.dayOfMonthWildcard && !s.dayOfWeekWildcard {
		return dayOfMonthMatches || dayOfWeekMatches
	}

	return dayOfMonthMatches && dayOfWeekMatches
}

// Next returns the earliest time strictly after the given one that the schedule
// matches, in the same location. ok is false when nothing matches within the
// search horizon.
func (s *Schedule) Next(after time.Time) (_ time.Time, ok bool) {
	// Earliest candidate strictly after the start of the next minute.
	t := after.Truncate(time.Minute).Add(time.Minute)

	yearLimit := t.Year() + searchHorizonYears
	aligned := false

restart:
	for {
		if t.Year() > yearLimit {
			return time.Time{}, false
		}

		for s.fields[month]&bit(int(t.Month())) == 0 {
			if !aligned {
				aligned, t = true, startOfMonth(t)
			}

			year := t.Year()
			if t = t.AddDate(0, 1, 0); t.Year() != year {
				continue restart // new year: recheck
			}
		}

		for !s.dayMatches(t) {
			if !aligned {
				aligned, t = true, startOfDay(t)
			}

			mon := t.Month()
			if t = t.AddDate(0, 0, 1); t.Month() != mon {
				continue restart // new month: recheck month
			}
		}

		for s.fields[hour]&bit(t.Hour()) == 0 {
			if !aligned {
				aligned, t = true, startOfHour(t)
			}

			day := t.Day()
			if t = t.Add(time.Hour); t.Day() != day {
				continue restart // crossed midnight: recheck day
			}
		}

		for s.fields[minute]&bit(t.Minute()) == 0 {
			h := t.Hour()
			if t = t.Add(time.Minute); t.Hour() != h {
				continue restart // new hour: recheck hour
			}
		}

		return t, true
	}
}

// Prev returns the latest time strictly before the given one that the schedule matches.
func (s *Schedule) Prev(before time.Time) (_ time.Time, ok bool) {
	t := before.Truncate(time.Minute)
	if !t.Before(before) {
		t = t.Add(-time.Minute)
	}

	yearLimit := t.Year() - searchHorizonYears
	aligned := false

restart:
	for {
		if t.Year() < yearLimit {
			return time.Time{}, false
		}

		for s.fields[month]&bit(int(t.Month())) == 0 {
			// Jump to the last minute of the previous month: skips the
			// unmatched month and maxes out every finer field at once.
			aligned, t = true, startOfMonth(t).Add(-time.Minute)
			if t.Month() == time.December {
				continue restart // stepped into the previous year
			}
		}

		for !s.dayMatches(t) {
			if !aligned {
				aligned, t = true, endOfDay(t)
			}
			mon := t.Month()
			if t = t.AddDate(0, 0, -1); t.Month() != mon {
				continue restart
			}
		}

		for s.fields[hour]&bit(t.Hour()) == 0 {
			if !aligned {
				aligned, t = true, endOfHour(t)
			}
			day := t.Day()
			if t = t.Add(-time.Hour); t.Day() != day {
				continue restart
			}
		}

		for s.fields[minute]&bit(t.Minute()) == 0 {
			h := t.Hour()
			if t = t.Add(-time.Minute); t.Hour() != h {
				continue restart
			}
		}

		return t, true
	}
}

func bit(n int) uint64 {
	return 1 << n
}

// wildcard is the bitmap "*" expands to: every legal value, with day-of-week 7
// folded onto 0 to match the parser.
func wildcard(f field) uint64 {
	b := cronBounds[f]
	var m uint64
	for v := b.from; v <= b.to; v++ {
		if f == dayOfWeek && v == 7 {
			continue
		}

		m |= bit(v)
	}
	return m
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func startOfHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 0, 0, t.Location())
}

func endOfHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 59, 0, 0, t.Location())
}
