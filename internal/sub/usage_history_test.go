package sub

import (
	"testing"
	"time"
)

func TestSubscriptionUsageSpec(t *testing.T) {
	loc := time.FixedZone("panel", 3*60*60+30*60)
	now := time.Date(2026, 8, 4, 1, 42, 0, 0, loc)

	period, start, count, _, err := subscriptionUsageSpec("24h", now, loc)
	if err != nil || period != "hour" || count != 24 {
		t.Fatalf("24h spec = %q %d %v", period, count, err)
	}
	if want := time.Date(2026, 8, 3, 2, 0, 0, 0, loc); !start.Equal(want) {
		t.Fatalf("24h start = %v, want %v", start, want)
	}

	period, start, count, _, err = subscriptionUsageSpec("30d", now, loc)
	if err != nil || period != "day" || count != 30 {
		t.Fatalf("30d spec = %q %d %v", period, count, err)
	}
	if want := time.Date(2026, 7, 6, 0, 0, 0, 0, loc); !start.Equal(want) {
		t.Fatalf("30d start = %v, want %v", start, want)
	}
}

func TestMakeSubscriptionUsagePointsFillsMissingBuckets(t *testing.T) {
	loc := time.UTC
	start := time.Date(2026, 8, 3, 0, 0, 0, 0, loc)
	rows := []subscriptionUsageAggregate{{BucketStart: start.Add(time.Hour).UnixMilli(), Upload: 10, Download: 20}}
	points := makeSubscriptionUsagePoints(rows, "hour", start, 3, func(t time.Time) time.Time { return t.Add(time.Hour) }, loc)
	if len(points) != 3 || points[0].Total != 0 || points[1].Total != 30 || points[2].Total != 0 {
		t.Fatalf("unexpected points: %#v", points)
	}
}
