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
	startedAt   time.Time
}

// shouldRun starts or resets the countdown whenever the scheduler is enabled
// or its cadence/mode changes. It returns true exactly once per elapsed period.
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
		s.startedAt = time.Time{}
		return false, nil
	}

	duration, err := entity.ScheduledRestartDuration(interval, unit)
	if err != nil {
		return false, err
	}
	fingerprint := fmt.Sprintf("%d:%s:%t", interval, unit, restartPanel)
	if !s.active || s.fingerprint != fingerprint {
		s.active = true
		s.fingerprint = fingerprint
		s.startedAt = now
		return false, nil
	}
	if now.Sub(s.startedAt) < duration {
		return false, nil
	}
	s.startedAt = now
	return true, nil
}
