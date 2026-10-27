package service

import (
	"testing"
	"time"
)

func TestReportCalendarDatesAndTimezoneBoundary(t *testing.T) {
	for _, tc := range []struct{ date, want string }{
		{"2026-10-27T12:34:56Z", "1405/08/05 12:34:56"},
		{"2025-03-20T00:00:00Z", "1403/12/30 00:00:00"},
		{"2025-03-21T00:00:00Z", "1404/01/01 00:00:00"},
		{"2026-03-20T00:00:00Z", "1404/12/29 00:00:00"},
		{"2026-03-21T00:00:00Z", "1405/01/01 00:00:00"},
		{"2000-03-20T00:00:00Z", "1379/01/01 00:00:00"},
	} {
		date, err := time.Parse(time.RFC3339, tc.date)
		if err != nil {
			t.Fatal(err)
		}
		if got := formatReportTime(date, "jalalian"); got != tc.want {
			t.Errorf("%s = %s, want %s", tc.date, got, tc.want)
		}
		if got := formatReportTime(date, "gregorian"); got != date.Format("2006-01-02 15:04:05") {
			t.Fatal(got)
		}
	}
	setupBulkDB(t)
	svc := &SettingService{}
	if err := svc.setString("datepicker", "jalalian"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetScheduledRestartTimezone("tehran"); err != nil {
		t.Fatal(err)
	}
	date, _ := time.Parse(time.RFC3339, "2026-10-27T21:00:00Z")
	if got := svc.FormatReportTime(date); got != "1405/08/06 00:30:00" {
		t.Fatal(got)
	}
}
