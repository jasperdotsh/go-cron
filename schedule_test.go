package cron_test

import (
	"fmt"
	"testing"
	"time"

	cron "github.com/jasperdotsh/go-cron"
)

func at(y int, mo time.Month, d, h, mi, s int) time.Time {
	return time.Date(y, mo, d, h, mi, s, 0, time.UTC)
}

func mustParse(t *testing.T, expr string) *cron.Schedule {
	t.Helper()
	s, err := cron.Parse(expr)
	if err != nil {
		t.Fatalf("Parse(%q) error: %v", expr, err)
	}
	return s
}

// scheduleCase maps an (expr, from) to the want for Next or Prev.
type scheduleCase struct {
	name string
	expr string
	from time.Time
	want time.Time
}

// runCases parses each case and checks that op (Next or Prev, named for the output)
// returns the expected match.
func runCases(t *testing.T, name string, op func(*cron.Schedule, time.Time) (time.Time, bool), cases []scheduleCase) {
	t.Helper()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := op(mustParse(t, tc.expr), tc.from)

			if !ok {
				t.Fatalf("%s(%q, %v) = _, false; want %v", name, tc.expr, tc.from, tc.want)
			}

			if !got.Equal(tc.want) {
				t.Errorf("%s(%q, %v) = %v; want %v", name, tc.expr, tc.from, got, tc.want)
			}
		})
	}
}

func TestNext(t *testing.T) {
	runCases(t, "Next", (*cron.Schedule).Next, []scheduleCase{
		{"every 15 minutes", "*/15 * * * *", at(2026, 6, 1, 12, 2, 0), at(2026, 6, 1, 12, 15, 0)},
		{"daily midnight rolls to next day", "0 0 * * *", at(2026, 6, 1, 12, 0, 0), at(2026, 6, 2, 0, 0, 0)},
		{"after an exact match", "0 12 * * *", at(2026, 6, 1, 12, 0, 0), at(2026, 6, 2, 12, 0, 0)},
		{"day of month rolls to next month", "0 12 1 * *", at(2026, 6, 1, 12, 0, 0), at(2026, 7, 1, 12, 0, 0)},
		{"month boundary across a year", "0 0 1 1 *", at(2026, 6, 1, 0, 0, 0), at(2027, 1, 1, 0, 0, 0)},
		{"comma list of minutes", "1,15,30 * * * *", at(2026, 6, 1, 12, 2, 0), at(2026, 6, 1, 12, 15, 0)},
		{"named month", "0 0 1 JAN *", at(2026, 6, 1, 12, 0, 0), at(2027, 1, 1, 0, 0, 0)},
		{"only day of week restricted", "0 0 * * MON", at(2026, 6, 1, 0, 0, 0), at(2026, 6, 8, 0, 0, 0)},
		{"only day of month restricted", "0 0 15 * *", at(2026, 6, 1, 0, 0, 0), at(2026, 6, 15, 0, 0, 0)},
		{"bare value step runs to max", "30/15 * * * *", at(2026, 6, 1, 12, 0, 0), at(2026, 6, 1, 12, 30, 0)},
		{"leap day across years", "0 0 29 2 *", at(2026, 1, 1, 0, 0, 0), at(2028, 2, 29, 0, 0, 0)},
	})
}

func TestPrev(t *testing.T) {
	runCases(t, "Prev", (*cron.Schedule).Prev, []scheduleCase{
		{"every 15 minutes", "*/15 * * * *", at(2026, 6, 1, 12, 2, 0), at(2026, 6, 1, 12, 0, 0)},
		{"daily midnight", "0 0 * * *", at(2026, 6, 1, 12, 0, 0), at(2026, 6, 1, 0, 0, 0)},
		{"strictly before an exact match", "0 12 * * *", at(2026, 6, 1, 12, 0, 0), at(2026, 5, 31, 12, 0, 0)},
		{"day of month rolls to previous month", "0 12 1 * *", at(2026, 6, 1, 0, 0, 0), at(2026, 5, 1, 12, 0, 0)},
		{"month boundary across a year", "0 0 1 1 *", at(2026, 6, 1, 0, 0, 0), at(2026, 1, 1, 0, 0, 0)},
		{"comma list of minutes", "1,15,30 * * * *", at(2026, 6, 1, 12, 2, 0), at(2026, 6, 1, 12, 1, 0)},
		{"month boundary backward across a year", "0 0 1 6 *", at(2026, 1, 1, 0, 0, 0), at(2025, 6, 1, 0, 0, 0)},
		{"only day of week restricted", "0 0 * * MON", at(2026, 6, 1, 0, 0, 0), at(2026, 5, 25, 0, 0, 0)},
	})
}

// TestNextPrevConsistency cross-checks the forward and backward scans against
// each other. For the firing m that Next reports, Prev(m) must land no later than the start
// and Next from there must come back to m - the inverse holds for Prev.
func TestNextPrevConsistency(t *testing.T) {
	exprs := []string{
		"*/15 * * * *",
		"0 0 * * *",
		"0 9 * * MON-FRI",
		"1,15,30,45 0-12/2 1-15 JAN,JUN,DEC *",
		"30 0 1 1 *",
	}
	froms := []time.Time{
		at(2026, 1, 1, 0, 0, 0),
		at(2026, 6, 1, 12, 30, 15),
		at(2026, 12, 31, 23, 59, 59),
	}

	for _, expr := range exprs {
		for _, from := range froms {
			s := mustParse(t, expr)

			if m, ok := s.Next(from); ok {
				if !m.After(from) {
					t.Errorf("Next(%q, %v) = %v; not after the start", expr, from, m)
				}

				if p, ok := s.Prev(m); ok {
					if p.After(from) {
						t.Errorf("Next(%q, %v) = %v; skipped an earlier firing at %v", expr, from, m, p)
					}

					if back, _ := s.Next(p); !back.Equal(m) {
						t.Errorf("Next(Prev(%v)) = %v; want %v for %q", m, back, m, expr)
					}
				}
			}

			if p, ok := s.Prev(from); ok {
				if !p.Before(from) {
					t.Errorf("Prev(%q, %v) = %v; not before the start", expr, from, p)
				}

				if m, ok := s.Next(p); ok {
					if m.Before(from) {
						t.Errorf("Prev(%q, %v) = %v; skipped a later firing at %v", expr, from, p, m)
					}

					if back, _ := s.Prev(m); !back.Equal(p) {
						t.Errorf("Prev(Next(%v)) = %v; want %v for %q", p, back, p, expr)
					}
				}
			}
		}
	}
}

// TestNextDayOfMonthOrDayOfWeek covers the OR rule: when both day fields are
// restricted, either one matching is enough.
func TestNextDayOfMonthOrDayOfWeek(t *testing.T) {
	got, ok := mustParse(t, "0 0 15 * 5").Next(at(2026, 6, 1, 0, 0, 0))

	if !ok {
		t.Fatal("Next returned no match")
	}

	if want := at(2026, 6, 5, 0, 0, 0); !got.Equal(want) {
		t.Errorf("Next = %v, want %v (first Friday)", got, want)
	}
}

// TestImpossibleSchedule checks the horizon: Feb 30th never happens, so Next
// and Prev report no match instead of looping forever.
func TestImpossibleSchedule(t *testing.T) {
	s := mustParse(t, "0 0 30 2 *")

	if got, ok := s.Next(at(2026, 1, 1, 0, 0, 0)); ok {
		t.Errorf("Next = %v, true; want no match", got)
	}

	if got, ok := s.Prev(at(2026, 1, 1, 0, 0, 0)); ok {
		t.Errorf("Prev = %v, true; want no match", got)
	}
}

func ExampleSchedule_Next() {
	s, _ := cron.Parse("0 9 * * MON-FRI")

	next, _ := s.Next(time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	fmt.Println(next.Format(time.RFC3339))
	// Output: 2026-06-01T09:00:00Z
}
