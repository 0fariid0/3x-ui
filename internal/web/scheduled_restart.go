package web

import (
	"fmt"
	"sync"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

type scheduledRestartState struct {
	mu sync.Mutex
}

// alignedRestartSlot maps a wall clock to a stable schedule slot. Minute
// schedules align to 00:00 plus N-minute boundaries, hour schedules to local
// midnight plus N-hour boundaries, and day schedules to local midnights.
// Civil time is used rather than Unix-duration truncation so full-hour jobs stay
// on :00 in half-hour zones such as Tehran.
func alignedRestartSlot(now time.Time, interval int, unit string) (int64, error) {
	if _, err := entity.ScheduledRestartDuration(interval, unit); err != nil {
		return 0, err
	}
	year, month, day := now.Date()
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

// scheduledRestartFingerprint identifies a schedule definition. Persisting it
// with the last completed slot allows a full x-ui process restart to resume the
// same recurring schedule without either firing twice in one slot or forgetting
// to run on later boundaries.
func scheduledRestartFingerprint(interval int, unit string, restartPanel bool, timezone string) string {
	return fmt.Sprintf("%d:%s:%t:%s", interval, unit, restartPanel, timezone)
}

func isAlignedRestartBoundary(now time.Time, interval int, unit string) bool {
	switch unit {
	case entity.ScheduledRestartUnitMinutes:
		return (now.Hour()*60+now.Minute())%interval == 0
	case entity.ScheduledRestartUnitHours:
		return now.Minute() == 0 && now.Hour()%interval == 0
	case entity.ScheduledRestartUnitDays:
		year, month, day := now.Date()
		civilDay := time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Unix() / int64(24*time.Hour/time.Second)
		return now.Hour() == 0 && now.Minute() == 0 && civilDay%int64(interval) == 0
	default:
		return false
	}
}

// shouldRun evaluates a recurring aligned schedule against the state persisted
// in the settings table. The returned persist flag means fingerprint/slot must
// be saved before any restart is attempted. Saving first is critical for a full
// panel restart because the current process exits immediately afterwards.
func (s *scheduledRestartState) shouldRun(
	now time.Time,
	enabled bool,
	interval int,
	unit string,
	restartPanel bool,
	timezone string,
	persistedFingerprint string,
	persistedSlot int64,
) (due bool, fingerprint string, currentSlot int64, persist bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !enabled {
		if persistedFingerprint != "" || persistedSlot != -1 {
			return false, "", -1, true, nil
		}
		return false, "", -1, false, nil
	}

	currentSlot, err = alignedRestartSlot(now, interval, unit)
	if err != nil {
		return false, "", 0, false, err
	}
	fingerprint = scheduledRestartFingerprint(interval, unit, restartPanel, timezone)

	// First enable or any schedule change starts from the current slot and waits
	// for the next aligned boundary. Saving settings therefore never triggers an
	// immediate surprise restart.
	if persistedFingerprint != fingerprint || persistedSlot < 0 {
		return false, fingerprint, currentSlot, true, nil
	}
	if currentSlot <= persistedSlot {
		return false, fingerprint, currentSlot, false, nil
	}
	// Do not catch up at an arbitrary time after downtime. Advance the stored
	// slot silently and wait for the next real clock boundary.
	if !isAlignedRestartBoundary(now, interval, unit) {
		return false, fingerprint, currentSlot, true, nil
	}
	return true, fingerprint, currentSlot, true, nil
}
