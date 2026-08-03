package sub

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/entity"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

type subscriptionUsagePoint struct {
	PeriodStart string `json:"period_start"`
	Upload      int64  `json:"upload"`
	Download    int64  `json:"download"`
	Total       int64  `json:"total_traffic"`
}

type subscriptionUsageAggregate struct {
	BucketStart int64 `gorm:"column:bucket_start"`
	Upload      int64 `gorm:"column:upload"`
	Download    int64 `gorm:"column:download"`
}

func subscriptionUsageLocation() *time.Location {
	timezone, err := (&service.SettingService{}).GetScheduledRestartTimezone()
	if err != nil {
		return time.Local
	}
	loc, _, err := entity.ScheduledRestartLocation(timezone)
	if err != nil || loc == nil {
		return time.Local
	}
	return loc
}

func subscriptionUsageHourStart(at time.Time, loc *time.Location) time.Time {
	local := at.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), local.Hour(), 0, 0, 0, loc)
}

func subscriptionUsageDayStart(at time.Time, loc *time.Location) time.Time {
	local := at.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
}

func subscriptionUsageSpec(rangeName string, now time.Time, loc *time.Location) (period string, start time.Time, count int, step func(time.Time) time.Time, err error) {
	switch rangeName {
	case "24h":
		end := subscriptionUsageHourStart(now, loc)
		return "hour", end.Add(-23 * time.Hour), 24, func(t time.Time) time.Time { return t.Add(time.Hour) }, nil
	case "30d":
		end := subscriptionUsageDayStart(now, loc)
		return "day", end.AddDate(0, 0, -29), 30, func(t time.Time) time.Time { return t.AddDate(0, 0, 1) }, nil
	default:
		return "", time.Time{}, 0, nil, errors.New("range must be 24h or 30d")
	}
}

// usageHistory exposes the existing compact traffic rollups to subscription
// themes. It resolves emails only from a valid subscription token and never
// accepts a client identifier from the browser.
func (a *SUBController) usageHistory(c *gin.Context) {
	rangeName := strings.ToLower(strings.TrimSpace(c.DefaultQuery("range", "30d")))
	if rangeName != "24h" && rangeName != "30d" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "range must be 24h or 30d"})
		return
	}

	subID := c.Param("subid")
	_, host, _, _ := a.subService.ResolveRequest(c)
	subReq := a.subService.ForRequest(host)
	subs, emails, _, _, err := subReq.getSubs(subID)
	if err != nil || len(subs) == 0 {
		writeSubError(c, err)
		return
	}

	points, period, err := loadSubscriptionUsageHistory(dedupeEmails(emails), rangeName, time.Now())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	setNoCacheHeaders(c)
	c.JSON(http.StatusOK, gin.H{
		"range":  rangeName,
		"period": period,
		"stats": gin.H{
			"usage": points,
		},
	})
}

func loadSubscriptionUsageHistory(emails []string, rangeName string, now time.Time) ([]subscriptionUsagePoint, string, error) {
	loc := subscriptionUsageLocation()
	period, start, count, step, err := subscriptionUsageSpec(rangeName, now, loc)
	if err != nil {
		return nil, "", err
	}
	if len(emails) == 0 {
		return makeSubscriptionUsagePoints(nil, period, start, count, step, loc), period, nil
	}

	db := database.GetDB()
	if db == nil {
		return nil, "", errors.New("database is not initialized")
	}

	rows := make([]subscriptionUsageAggregate, 0, count)
	if period == "hour" {
		err = db.Model(&model.ClientTrafficHourBucket{}).
			Select("bucket_start, SUM(up) AS upload, SUM(down) AS download").
			Where("email IN ? AND bucket_start >= ?", emails, start.UnixMilli()).
			Group("bucket_start").
			Order("bucket_start ASC").
			Scan(&rows).Error
	} else {
		err = db.Model(&model.ClientTrafficDayBucket{}).
			Select("bucket_start, SUM(up) AS upload, SUM(down) AS download").
			Where("email IN ? AND bucket_start >= ?", emails, start.UnixMilli()).
			Group("bucket_start").
			Order("bucket_start ASC").
			Scan(&rows).Error
	}
	if err != nil {
		return nil, period, err
	}

	return makeSubscriptionUsagePoints(rows, period, start, count, step, loc), period, nil
}

func makeSubscriptionUsagePoints(rows []subscriptionUsageAggregate, period string, start time.Time, count int, step func(time.Time) time.Time, loc *time.Location) []subscriptionUsagePoint {
	totals := make(map[int64]subscriptionUsagePoint, len(rows))
	for _, row := range rows {
		at := time.UnixMilli(row.BucketStart).In(loc)
		if period == "hour" {
			at = subscriptionUsageHourStart(at, loc)
		} else {
			at = subscriptionUsageDayStart(at, loc)
		}
		key := at.UnixMilli()
		point := totals[key]
		point.Upload += row.Upload
		point.Download += row.Download
		point.Total += row.Upload + row.Download
		totals[key] = point
	}

	points := make([]subscriptionUsagePoint, 0, count)
	cursor := start
	for i := 0; i < count; i++ {
		point := totals[cursor.UnixMilli()]
		point.PeriodStart = cursor.Format(time.RFC3339)
		points = append(points, point)
		cursor = step(cursor)
	}
	return points
}
