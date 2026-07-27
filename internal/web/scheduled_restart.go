package web

import (
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

type scheduledRestartState struct {
	mu          sync.Mutex
	active      bool
	fingerprint string
	lastSlot    int64
}

// alignedRestartSlot maps the local wall clock to a stable schedule slot.
// Minute schedules align to 00:00 plus N-minute boundaries, hour schedules to
// local midnight plus N-hour boundaries, and day schedules to local midnights.
// Using civil time (rather than Unix-duration truncation) keeps full-hour
// restarts on :00 even in time zones with half-hour offsets.
func alignedRestartSlot(now time.Time, interval int, unit string) (int64, error) {
	if _, err := entity.ScheduledRestartDuration(interval, unit); err != nil {
		return 0, err
	}
	year, month, day := now.Date()
	// UTC here is intentional: it creates a timezone-independent serial number
	// for the local calendar date without applying the local UTC offset twice.
	civilDay := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / int64(24*time.Hour/time.Second)

	switch unit {
	case entity.ScheduledRestartUnitMinutes:
		totalMinutes := civilDay*24*60 + int64(now.Hour()*60+now.Minute())
		return totalMinutes / int64(interval), nil
	case entity.ScheduledRestartUnitHours:
		totalHours := civilDay*24 + int64(now.Hour())
		return totalHours / int64(interval), nil
	case entity.ScheduledRestartUnitDays:
		return civilDay / int64(interval), nil
	default:
		return 0, fmt.Errorf("invalid scheduled restart unit: %s", unit)
	}
}

// shouldRun aligns scheduled restarts to real local-clock boundaries rather
// than counting from the last save or panel start. Examples: 15 minutes runs at
// :00/:15/:30/:45, 30 minutes at :00/:30, 1 hour at every full hour, and
// 2 hours at 00:00/02:00/04:00. Enabling or changing the schedule records the
// current slot and waits for the next boundary, so saving never restarts Xray
// immediately.
func (s *scheduledRestartState) shouldRun(
	now time.Time,
	enabled bool,
	interval int,
	unit string,
	restartPanel bool,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !enabled {
		s.active = false
		s.fingerprint = ""
		s.lastSlot = 0
		return false, nil
	}

	currentSlot, err := alignedRestartSlot(now, interval, unit)
	if err != nil {
		return false, err
	}
	fingerprint := fmt.Sprintf("%d:%s:%t", interval, unit, restartPanel)
	if !s.active || s.fingerprint != fingerprint {
		s.active = true
		s.fingerprint = fingerprint
		s.lastSlot = currentSlot
		return false, nil
	}
	if currentSlot <= s.lastSlot {
		return false, nil
	}
	s.lastSlot = currentSlot
	return true, nil
}
