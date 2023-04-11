package common

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// PrettyDuration is a pretty printed version of a time.Duration value that cuts
// the unnecessary precision off from the formatted textual representation.
type PrettyDuration time.Duration

var prettyDurationRe = regexp.MustCompile(`\.[0-9]+`)

// String implements the Stringer interface, allowing pretty printing of duration
// values rounded to three decimals.
func (d PrettyDuration) String() string {
	label := fmt.Sprintf("%v", time.Duration(d))
	if match := prettyDurationRe.FindString(label); len(match) > 4 {
		label = strings.Replace(label, match, match[:4], 1)
	}
	return label
}

// PrettyAge is a pretty printed version of a time.Duration value that rounds
// the values up to a single most significant unit, days/weeks/years included.
type PrettyAge time.Time

// ageUnits is a list of units the age pretty printing uses.
var ageUnits = []struct {
	Size   time.Duration
	Symbol string
}{
	{12 * 30 * 24 * time.Hour, "y"},
	{30 * 24 * time.Hour, "mo"},
	{7 * 24 * time.Hour, "w"},
	{24 * time.Hour, "d"},
	{time.Hour, "h"},
	{time.Minute, "m"},
	{time.Second, "s"},
}

// String implements the Stringer interface, allowing pretty printing of duration
// values rounded to the most significant time unit.
func (t PrettyAge) String() string {
	// Calculate the time difference and handle the 0 cornercase
	diff := time.Since(time.Time(t))
	if diff < time.Second {
		return "0"
	}
	// Accumulate a precision of 3 components before returning
	result, prec := "", 0

	for _, unit := range ageUnits {
		if diff > unit.Size {
			result = fmt.Sprintf("%s%d%s", result, diff/unit.Size, unit.Symbol)
			diff %= unit.Size

			if prec += 1; prec >= 3 {
				break
			}
		}
	}
	return result
}
