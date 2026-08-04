package sub

import "testing"

func TestUniqueSubscriptionEmails(t *testing.T) {
	got := uniqueSubscriptionEmails([]string{" alice ", "bob", "alice", "", "bob"})
	if len(got) != 2 || got[0] != "alice" || got[1] != "bob" {
		t.Fatalf("uniqueSubscriptionEmails() = %#v", got)
	}
}

func TestSubscriptionHistoryPeriod(t *testing.T) {
	if got := subscriptionHistoryPeriod("24h"); got != "hour" {
		t.Fatalf("24h period = %q", got)
	}
	if got := subscriptionHistoryPeriod("30d"); got != "day" {
		t.Fatalf("30d period = %q", got)
	}
}
