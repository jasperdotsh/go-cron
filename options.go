package cron

import (
	"fmt"
)

type Option func(*config)

// Macro is a named alias that expands to an expression, like @daily for "0 0 * * *"
type Macro struct {
	Name  string
	Value string
}

// StandardMacros are reasonable default macros.
var StandardMacros = []Macro{
	{"@hourly", "0 * * * *"},
	{"@daily", "0 0 * * *"},
	{"@weekly", "0 0 * * 0"},
	{"@monthly", "0 0 1 * *"},
	{"@yearly", "0 0 1 1 *"},
}

// standardFields is the default set of fields in cron syntax
var standardFields = []Field{Minute, Hour, DayOfMonth, Month, DayOfWeek}

// standardFieldsWithSeconds is the default set of fields, prefixed by a Seconds field.
var standardFieldsWithSeconds = []Field{Seconds, Minute, Hour, DayOfMonth, Month, DayOfWeek}

func WithSeconds() Option {
	return WithFields(standardFieldsWithSeconds...)
}

func WithFields(fields ...Field) Option {
	return func(c *config) {
		c.fields = fields

		for _, field := range fields {
			if !field.Valid() {
				c.err = fmt.Errorf("invalid field: %d", field)
				return
			}
		}
	}
}

func WithMacros(macros ...Macro) Option {
	return func(c *config) {
		for _, macro := range macros {
			c.macros = append(c.macros, macro)
		}
	}
}

// New instantiates the Parser with optionally provided options.
//
// By default the parser will have no Macros and follows the format:
// Minutes Hours DayOfMonth Month DayOfWeek
//
// New returns nil and an error when an unknown field was provided.
func New(options ...Option) (*Parser, error) {
	cfg := &config{
		fields: standardFields,
	}

	for _, opt := range options {
		opt(cfg)
	}

	if cfg.err != nil {
		return nil, cfg.err
	}

	return &Parser{cfg: cfg}, nil
}

type config struct {
	macros []Macro
	fields []Field

	err error
}
