package entity

import (
	"testing"
	"time"
)

func TestScheduledRestartDuration(t *testing.T) {
	tests := []struct {
		name     string
		interval int
		unit     string
		want     time.Duration
		wantErr  bool
	}{
		{name: "minutes", interval: 5, unit: ScheduledRestartUnitMinutes, want: 5 * time.Minute},
		{name: "hours", interval: 2, unit: ScheduledRestartUnitHours, want: 2 * time.Hour},
		{name: "days", interval: 3, unit: ScheduledRestartUnitDays, want: 72 * time.Hour},
		{name: "zero", interval: 0, unit: ScheduledRestartUnitHours, wantErr: true},
		{name: "bad unit", interval: 1, unit: "weeks", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ScheduledRestartDuration(tt.interval, tt.unit)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got duration %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAllSettingDefaultsScheduledRestart(t *testing.T) {
	s := &AllSetting{WebPort: 2053, SubPort: 2096}
	if err := s.CheckValid(); err != nil {
		t.Fatalf("CheckValid returned error: %v", err)
	}
	if s.ScheduledRestartInterval != 24 || s.ScheduledRestartUnit != ScheduledRestartUnitHours {
		t.Fatalf("unexpected defaults: interval=%d unit=%q", s.ScheduledRestartInterval, s.ScheduledRestartUnit)
	}
}
