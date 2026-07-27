package web

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestScheduledRestartState(t *testing.T) {
	var state scheduledRestartState
	start := time.Unix(1000, 0)

	due, err := state.shouldRun(start, true, 2, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || due {
		t.Fatalf("initial enable should start timer: due=%v err=%v", due, err)
	}

	due, err = state.shouldRun(start.Add(119*time.Second), true, 2, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || due {
		t.Fatalf("should not be due early: due=%v err=%v", due, err)
	}

	due, err = state.shouldRun(start.Add(2*time.Minute), true, 2, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || !due {
		t.Fatalf("should be due at interval: due=%v err=%v", due, err)
	}

	// Changing mode starts a fresh countdown and avoids an immediate panel restart.
	due, err = state.shouldRun(start.Add(3*time.Minute), true, 2, entity.ScheduledRestartUnitMinutes, true)
	if err != nil || due {
		t.Fatalf("mode change should reset timer: due=%v err=%v", due, err)
	}

	due, err = state.shouldRun(start.Add(10*time.Minute), false, 2, entity.ScheduledRestartUnitMinutes, true)
	if err != nil || due {
		t.Fatalf("disabled scheduler must not run: due=%v err=%v", due, err)
	}
}
