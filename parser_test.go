package cron

import (
	"testing"
)

// bits sets every bit from lo to hi, inclusive.
func bits(lo, hi int) uint64 {
	var b uint64
	for i := lo; i <= hi; i++ {
		b |= 1 << i
	}
	return b
}

// mask sets exactly the given bits.
func mask(vals ...int) uint64 {
	var b uint64
	for _, v := range vals {
		b |= 1 << v
	}
	return b
}

func TestParseField(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		field field
		want  uint64
	}{
		{"wildcard", "*", minute, bits(0, 59)},
		{"single value", "5", minute, mask(5)},
		{"comma list", "1,15,30", minute, mask(1, 15, 30)},
		{"range", "1-5", minute, bits(1, 5)},
		{"step wildcard", "*/15", minute, mask(0, 15, 30, 45)},
		{"step range", "0-30/10", minute, mask(0, 10, 20, 30)},
		{"step single value spans to max", "30/15", minute, mask(30, 45)},
		{"step single value in hour", "6/6", hour, mask(6, 12, 18)},
		{"step single-element range keeps bounds", "2-2/2", minute, mask(2)},
		{"month names", "JAN,DEC", month, mask(1, 12)},
		{"month name lowercase", "jan", month, mask(1)},
		{"weekday name range", "MON-FRI", dayOfWeek, mask(1, 2, 3, 4, 5)},
		{"weekday 7 wraps to 0", "7", dayOfWeek, mask(0)},
		{"weekday 0 is sunday", "0", dayOfWeek, mask(0)},
		{"weekday range wrapping 7", "5-7", dayOfWeek, mask(0, 5, 6)},
		{"weekday name lowercase", "sun", dayOfWeek, mask(0)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseField(tt.text, tt.field)
			if err != nil {
				t.Fatalf("parseField(%q, %v) error: %v", tt.text, tt.field, err)
			}
			if got != tt.want {
				t.Errorf("parseField(%q, %v) = %b, want %b", tt.text, tt.field, got, tt.want)
			}
		})
	}
}

func TestParseFieldErrors(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		field field
	}{
		{"minute out of bounds", "60", minute},
		{"hour out of bounds", "24", hour},
		{"day of month zero", "0", dayOfMonth},
		{"day of month out of bounds", "32", dayOfMonth},
		{"month zero", "0", month},
		{"month out of bounds", "13", month},
		{"weekday out of bounds", "8", dayOfWeek},
		{"reversed range", "5-1", minute},
		{"zero step", "*/0", minute},
		{"negative step", "*/-3", minute},
		{"zero step on value", "5/0", minute},
		{"double dash", "1-2-3", minute},
		{"double slash", "*/5/5", minute},
		{"unknown name", "FOO", minute},
		{"unknown name in range", "5-FOO", minute},
		{"leading plus", "+5", minute},
		{"leading plus in range", "1-+5", minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := parseField(tt.text, tt.field); err == nil {
				t.Errorf("parseField(%q, %v) = %b, want error", tt.text, tt.field, got)
			}
		})
	}
}

func TestParseFieldCount(t *testing.T) {
	for _, expr := range []string{"* * * *", "* * * * * *", "", "   "} {
		if got, err := Parse(expr); err == nil {
			t.Errorf("Parse(%q) = %v, want error", expr, got)
		} else if got != nil {
			t.Errorf("Parse(%q) returned %v alongside error, want nil", expr, got)
		}
	}
}
