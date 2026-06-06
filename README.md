# go-cron

A light, dependency-free Go library for parsing standard five-field cron expressions (`Minute Hour DayOfMonth Month DayOfWeek`) and computing next and previous execution.

## Installation

```shell
go get github.com/jasperdotsh/go-cron
```

## Usage

```go
package main

import (
	"fmt"
	"time"

	cron "github.com/jasperdotsh/go-cron"
)

func main() {
	sched, err := cron.Parse("0 9 * * MON-FRI") // 09:00 on weekdays
	if err != nil {
		panic(err)
	}

	if next, ok := sched.Next(time.Now()); ok {
		fmt.Println("next run:", next)
	}
	
	if prev, ok := sched.Prev(time.Now()); ok {
		fmt.Println("previous run:", prev)
	}
}
```

## Behavior

`Next` finds the first match after a given time. `Prev` finds the last one before a given time. Matches land on a minute boundary (second `0`) and keep the inputs location.

- **Names:** Months and weekdays also accept three-letter names like `MON-FRI` or `JAN` (case-insensitive).
- **Day of month and day of week:** When both are restricted, a day matches if either one does, so `0 0 13 * 5` runs on the 13th and on every Friday. If one is `*`, the other one decides.
- When a schedule is impossible (like `0 0 30 2 *`, February 30th) or nothing falls within five years, `Next` and `Prev` return `false`.

## Steps

A step picks every nth value. After a range or wildcard thats the usual `*/15` or `0-30/10`. After a bare value it runs from there to the field's max, so `30/15` in the minute field gives minutes 30 and 45.
