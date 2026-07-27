package web

import (
	"testing"
	"time"
)

func TestScheduledPanelFollowUpDelay(t *testing.T) {
	if scheduledPanelFollowUpXrayDelay != 8*time.Second {
		t.Fatalf("unexpected follow-up delay: %s", scheduledPanelFollowUpXrayDelay)
	}
}
