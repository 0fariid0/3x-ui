package service

import (
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
)

func (s *SettingService) FormatReportTime(value time.Time) string {
	if database.GetDB() == nil {
		return formatReportTime(value.In(time.Local), "gregorian")
	}
	calendar, _ := s.GetDatepicker()
	return formatReportTime(value.In(insightPanelLocation()), calendar)
}

func formatReportTime(value time.Time, calendar string) string {
	if calendar != "jalalian" {
		return value.Format("2006-01-02 15:04:05")
	}
	gy, gm, gd := value.Year(), int(value.Month()), value.Day()
	a := (14 - gm) / 12
	y := gy + 4800 - a
	m := gm + 12*a - 3
	days := gd + (153*m+2)/5 + 365*y + y/4 - y/100 + y/400 - 32045 - 2460311
	jy, jm, jd := 1402, 10, 11
	monthDays := func(year, month int) int {
		if month <= 6 {
			return 31
		}
		if month <= 11 {
			return 30
		}
		switch year % 33 {
		case 1, 5, 9, 13, 17, 22, 26, 30:
			return 30
		}
		return 29
	}
	for days > 0 {
		remaining := monthDays(jy, jm) - jd + 1
		if days < remaining {
			jd += days
			days = 0
			break
		}
		days -= remaining
		jm++
		if jm > 12 {
			jm = 1
			jy++
		}
		jd = 1
	}
	for days < 0 {
		jd += days
		for jd < 1 {
			jm--
			if jm < 1 {
				jm = 12
				jy--
			}
			jd += monthDays(jy, jm)
		}
		days = 0
	}
	return fmt.Sprintf("%04d/%02d/%02d %s", jy, jm, jd, value.Format("15:04:05"))
}
