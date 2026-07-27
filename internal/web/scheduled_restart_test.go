package web

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestScheduledRestartStateAlignsMinutesToClock(t *testing.T) {
	var state scheduledRestartState
	loc := time.FixedZone("IRST", 3*60*60+30*60)
	start := time.Date(2026, 7, 27, 12, 7, 5, 0, loc)

	due, err := state.shouldRun(start, true, 15, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || due {
		t.Fatalf("initial enable should wait for next boundary: due=%v err=%v", due, err)
	}
	for _, at := range []time.Time{
		time.Date(2026, 7, 27, 12, 14, 59, 0, loc),
		time.Date(2026, 7, 27, 12, 14, 59, 999000000, loc),
	} {
		due, err = state.shouldRun(at, true, 15, entity.ScheduledRestartUnitMinutes, false)
		if err != nil || due {
			t.Fatalf("should not run before :15: due=%v err=%v", due, err)
		}
	}
	due, err = state.shouldRun(time.Date(2026, 7, 27, 12, 15, 1, 0, loc), true, 15, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || !due {
		t.Fatalf("should run on first check after :15: due=%v err=%v", due, err)
	}
	due, err = state.shouldRun(time.Date(2026, 7, 27, 12, 15, 10, 0, loc), true, 15, entity.ScheduledRestartUnitMinutes, false)
	if err != nil || due {
		t.Fatalf("must run once per aligned slot: due=%v err=%v", due, err)
	}
}

func TestScheduledRestartStateAlignsHoursInHalfHourTimezone(t *testing.T) {
	var state scheduledRestartState
	loc := time.FixedZone("IRST", 3*60*60+30*60)
	start := time.Date(2026, 7, 27, 1, 23, 0, 0, loc)

	due, err := state.shouldRun(start, true, 2, entity.ScheduledRestartUnitHours, false)
	if err != nil || due {
		t.Fatalf("initial enable should wait: due=%v err=%v", due, err)
	}
	due, err = state.shouldRun(time.Date(2026, 7, 27, 1, 59, 59, 0, loc), true, 2, entity.ScheduledRestartUnitHours, false)
	if err != nil || due {
		t.Fatalf("should not run before local 02:00: due=%v err=%v", due, err)
	}
	due, err = state.shouldRun(time.Date(2026, 7, 27, 2, 0, 2, 0, loc), true, 2, entity.ScheduledRestartUnitHours, false)
	if err != nil || !due {
		t.Fatalf("should run after local 02:00 boundary: due=%v err=%v", due, err)
	}

	// Changing mode resets the slot and avoids an immediate full panel restart.
	due, err = state.shouldRun(time.Date(2026, 7, 27, 2, 1, 0, 0, loc), true, 2, entity.ScheduledRestartUnitHours, true)
	if err != nil || due {
		t.Fatalf("mode change should wait for next boundary: due=%v err=%v", due, err)
	}

	due, err = state.shouldRun(time.Date(2026, 7, 27, 10, 0, 0, 0, loc), false, 2, entity.ScheduledRestartUnitHours, true)
	if err != nil || due {
		t.Fatalf("disabled scheduler must not run: due=%v err=%v", due, err)
	}
}

func TestAlignedRestartSlotDayBoundary(t *testing.T) {
	loc := time.FixedZone("UTC+0330", 3*60*60+30*60)
	before, err := alignedRestartSlot(time.Date(2026, 7, 27, 23, 59, 59, 0, loc), 1, entity.ScheduledRestartUnitDays)
	if err != nil {
		t.Fatal(err)
	}
	after, err := alignedRestartSlot(time.Date(2026, 7, 28, 0, 0, 1, 0, loc), 1, entity.ScheduledRestartUnitDays)
	if err != nil {
		t.Fatal(err)
	}
	if after != before+1 {
		t.Fatalf("daily slot did not advance at local midnight: before=%d after=%d", before, after)
	}
}
