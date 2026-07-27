package web

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
)

func TestScheduledRestartStatePersistsAcrossPanelRestart(t *testing.T) {
	loc := time.FixedZone("IRST", 3*60*60+30*60)
	var firstProcess scheduledRestartState
	start := time.Date(2026, 7, 27, 1, 23, 0, 0, loc)

	due, fp, slot, persist, err := firstProcess.shouldRun(
		start, true, 1, entity.ScheduledRestartUnitHours, true, entity.ScheduledRestartTimezoneTehran, "", -1,
	)
	if err != nil || due || !persist {
		t.Fatalf("first enable must persist current slot without restarting: due=%v persist=%v err=%v", due, persist, err)
	}

	// At 02:00 the first process schedules a full panel restart and persists the
	// new slot before exiting.
	due, fp, slot, persist, err = firstProcess.shouldRun(
		time.Date(2026, 7, 27, 2, 0, 2, 0, loc), true, 1, entity.ScheduledRestartUnitHours, true,
		entity.ScheduledRestartTimezoneTehran, fp, slot,
	)
	if err != nil || !due || !persist {
		t.Fatalf("02:00 boundary must run: due=%v persist=%v err=%v", due, persist, err)
	}

	// Simulate the new x-ui process. The persisted slot prevents a duplicate at
	// 02:00 but the following hour remains due.
	var secondProcess scheduledRestartState
	due, _, _, persist, err = secondProcess.shouldRun(
		time.Date(2026, 7, 27, 2, 0, 20, 0, loc), true, 1, entity.ScheduledRestartUnitHours, true,
		entity.ScheduledRestartTimezoneTehran, fp, slot,
	)
	if err != nil || due || persist {
		t.Fatalf("new process must not repeat the same slot: due=%v persist=%v err=%v", due, persist, err)
	}
	due, _, _, persist, err = secondProcess.shouldRun(
		time.Date(2026, 7, 27, 3, 0, 1, 0, loc), true, 1, entity.ScheduledRestartUnitHours, true,
		entity.ScheduledRestartTimezoneTehran, fp, slot,
	)
	if err != nil || !due || !persist {
		t.Fatalf("03:00 boundary must still run after process restart: due=%v persist=%v err=%v", due, persist, err)
	}
}

func TestScheduledRestartStateAlignsMinutesToClock(t *testing.T) {
	var state scheduledRestartState
	loc := time.FixedZone("IRST", 3*60*60+30*60)
	start := time.Date(2026, 7, 27, 12, 7, 5, 0, loc)

	due, fp, slot, persist, err := state.shouldRun(
		start, true, 15, entity.ScheduledRestartUnitMinutes, false, entity.ScheduledRestartTimezoneTehran, "", -1,
	)
	if err != nil || due || !persist {
		t.Fatalf("initial enable should wait for next boundary: due=%v persist=%v err=%v", due, persist, err)
	}
	due, _, _, persist, err = state.shouldRun(
		time.Date(2026, 7, 27, 12, 14, 59, 0, loc), true, 15, entity.ScheduledRestartUnitMinutes, false,
		entity.ScheduledRestartTimezoneTehran, fp, slot,
	)
	if err != nil || due || persist {
		t.Fatalf("should not run before :15: due=%v persist=%v err=%v", due, persist, err)
	}
	due, _, _, persist, err = state.shouldRun(
		time.Date(2026, 7, 27, 12, 15, 1, 0, loc), true, 15, entity.ScheduledRestartUnitMinutes, false,
		entity.ScheduledRestartTimezoneTehran, fp, slot,
	)
	if err != nil || !due || !persist {
		t.Fatalf("should run on first check after :15: due=%v persist=%v err=%v", due, persist, err)
	}
}

func TestScheduledRestartDoesNotCatchUpAwayFromBoundary(t *testing.T) {
	loc := time.FixedZone("IRST", 3*60*60+30*60)
	var state scheduledRestartState
	fp := scheduledRestartFingerprint(1, entity.ScheduledRestartUnitHours, true, entity.ScheduledRestartTimezoneTehran)
	oldSlot, err := alignedRestartSlot(time.Date(2026, 7, 27, 2, 0, 0, 0, loc), 1, entity.ScheduledRestartUnitHours)
	if err != nil {
		t.Fatal(err)
	}
	due, _, currentSlot, persist, err := state.shouldRun(
		time.Date(2026, 7, 27, 3, 20, 0, 0, loc), true, 1, entity.ScheduledRestartUnitHours, true,
		entity.ScheduledRestartTimezoneTehran, fp, oldSlot,
	)
	if err != nil || due || !persist || currentSlot <= oldSlot {
		t.Fatalf("missed boundary must be skipped, not replayed: due=%v persist=%v old=%d current=%d err=%v", due, persist, oldSlot, currentSlot, err)
	}
}

func TestScheduledRestartTimezoneFallback(t *testing.T) {
	loc, mode, err := entity.ScheduledRestartLocation("tehran")
	if err != nil || mode != entity.ScheduledRestartTimezoneTehran {
		t.Fatalf("tehran timezone failed: mode=%q err=%v", mode, err)
	}
	_, offset := time.Date(2026, 7, 27, 12, 0, 0, 0, loc).Zone()
	if offset != 3*60*60+30*60 {
		t.Fatalf("unexpected Tehran offset: %d", offset)
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
