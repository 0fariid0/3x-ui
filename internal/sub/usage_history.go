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

// usageHistory exposes the already-recorded compact panel usage history to a
// valid subscription token. The browser never supplies a client email; the
// email set is resolved from the subscription ID itself.
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
	if err != nil || len(subs) == 0 || len(emails) == 0 {
		writeSubError(c, err)
		return
	}

	points, period, err := loadSubscriptionUsageHistory(emails, rangeName)
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

func loadSubscriptionUsageHistory(emails []string, rangeName string) ([]subscriptionUsagePoint, string, error) {
	db := database.GetDB()
	if db == nil {
		return nil, "", errors.New("database is not initialized")
	}
	emails = uniqueSubscriptionEmails(emails)
	if len(emails) == 0 {
		return []subscriptionUsagePoint{}, subscriptionHistoryPeriod(rangeName), nil
	}

	loc := subscriptionUsageLocation()
	now := time.Now().In(loc)
	if rangeName == "24h" {
		end := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, loc)
		start := end.Add(-23 * time.Hour)
		var rows []model.ClientTrafficHourBucket
		if err := db.Where("email IN ? AND bucket_start >= ? AND bucket_start <= ?", emails, start.UnixMilli(), end.UnixMilli()).
			Order("bucket_start ASC").Find(&rows).Error; err != nil {
			return nil, "hour", err
		}
		totals := make(map[int64]subscriptionUsagePoint, 24)
		for _, row := range rows {
			point := totals[row.BucketStart]
			point.Upload += row.Up
			point.Download += row.Down
			point.Total += row.Up + row.Down
			totals[row.BucketStart] = point
		}
		points := make([]subscriptionUsagePoint, 0, 24)
		cursor := start
		for i := 0; i < 24; i++ {
			key := cursor.UnixMilli()
			point := totals[key]
			point.PeriodStart = cursor.Format(time.RFC3339)
			points = append(points, point)
			cursor = cursor.Add(time.Hour)
		}
		return points, "hour", nil
	}

	end := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	start := end.AddDate(0, 0, -29)
	var rows []model.ClientTrafficDayBucket
	if err := db.Where("email IN ? AND bucket_start >= ? AND bucket_start <= ?", emails, start.UnixMilli(), end.UnixMilli()).
		Order("bucket_start ASC").Find(&rows).Error; err != nil {
		return nil, "day", err
	}
	totals := make(map[string]subscriptionUsagePoint, 30)
	for _, row := range rows {
		day := row.Day
		if strings.TrimSpace(day) == "" {
			day = time.UnixMilli(row.BucketStart).In(loc).Format("2006-01-02")
		}
		point := totals[day]
		point.Upload += row.Up
		point.Download += row.Down
		point.Total += row.Up + row.Down
		totals[day] = point
	}
	points := make([]subscriptionUsagePoint, 0, 30)
	cursor := start
	for i := 0; i < 30; i++ {
		day := cursor.Format("2006-01-02")
		point := totals[day]
		point.PeriodStart = cursor.Format(time.RFC3339)
		points = append(points, point)
		cursor = cursor.AddDate(0, 0, 1)
	}
	return points, "day", nil
}

func subscriptionHistoryPeriod(rangeName string) string {
	if rangeName == "24h" {
		return "hour"
	}
	return "day"
}

func uniqueSubscriptionEmails(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
