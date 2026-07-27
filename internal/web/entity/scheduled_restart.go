package entity

import (
	"fmt"
	"time"
)

const (
	ScheduledRestartUnitMinutes = "minutes"
	ScheduledRestartUnitHours   = "hours"
	ScheduledRestartUnitDays    = "days"
)

// ScheduledRestartDuration validates the configured cadence and converts it to
// a duration. Limits keep accidental values bounded to at most ten years.
func ScheduledRestartDuration(interval int, unit string) (time.Duration, error) {
	if interval < 1 {
		return 0, fmt.Errorf("scheduled restart interval must be at least 1")
	}

	var base time.Duration
	var max int
	switch unit {
	case ScheduledRestartUnitMinutes:
		base, max = time.Minute, 525600 // one year
	case ScheduledRestartUnitHours:
		base, max = time.Hour, 87600 // ten years
	case ScheduledRestartUnitDays:
		base, max = 24*time.Hour, 3650 // ten years
	default:
		return 0, fmt.Errorf("invalid scheduled restart unit: %s", unit)
	}
	if interval > max {
		return 0, fmt.Errorf("scheduled restart interval is too large for %s", unit)
	}
	return time.Duration(interval) * base, nil
}
