package job

import (
	"testing"
	"time"
)

func TestXrayHealthStateConfirmsFailureAndUsesCooldown(t *testing.T) {
	var state xrayHealthState
	policy := xrayHealthPolicy{
		enabled: true, failureThreshold: 2,
		cooldown: 5 * time.Minute, maxRestarts: 3, window: 30 * time.Minute,
	}
	now := time.Unix(1000, 0)
	if ok, _ := state.shouldRestart(now, true, policy); ok {
		t.Fatal("first failed check must not restart")
	}
	if ok, _ := state.shouldRestart(now.Add(time.Second), true, policy); !ok {
		t.Fatal("second consecutive failed check should restart")
	}
	if ok, _ := state.shouldRestart(now.Add(2*time.Second), true, policy); ok {
		t.Fatal("confirmation must start over after an attempt")
	}
	if ok, reason := state.shouldRestart(now.Add(3*time.Second), true, policy); ok || reason != "restart cooldown is active" {
		t.Fatalf("cooldown should block another attempt: ok=%v reason=%q", ok, reason)
	}
	if ok, _ := state.shouldRestart(now.Add(5*time.Minute+time.Second), true, policy); ok {
		t.Fatal("first check after cooldown only confirms failure")
	}
	if ok, _ := state.shouldRestart(now.Add(5*time.Minute+2*time.Second), true, policy); !ok {
		t.Fatal("second check after cooldown should allow restart")
	}
}

func TestXrayHealthStateCircuitBreaker(t *testing.T) {
	var state xrayHealthState
	policy := xrayHealthPolicy{
		enabled: true, failureThreshold: 1,
		cooldown: time.Minute, maxRestarts: 2, window: 10 * time.Minute,
	}
	now := time.Unix(2000, 0)
	if ok, _ := state.shouldRestart(now, true, policy); !ok {
		t.Fatal("first attempt should be allowed")
	}
	if ok, _ := state.shouldRestart(now.Add(time.Minute), true, policy); !ok {
		t.Fatal("second attempt should be allowed")
	}
	if ok, reason := state.shouldRestart(now.Add(2*time.Minute), true, policy); ok || reason != "maximum restart attempts reached" {
		t.Fatalf("third attempt should open circuit: ok=%v reason=%q", ok, reason)
	}
	if ok, reason := state.shouldRestart(now.Add(3*time.Minute), true, policy); ok || reason != "restart circuit breaker is open" {
		t.Fatalf("open circuit should block retries: ok=%v reason=%q", ok, reason)
	}
	if ok, _ := state.shouldRestart(now.Add(11*time.Minute), true, policy); !ok {
		t.Fatal("attempt should be allowed after rolling window expires")
	}
}

func TestXrayHealthStateHealthyAndDisabledReset(t *testing.T) {
	var state xrayHealthState
	policy := defaultXrayHealthPolicy()
	now := time.Unix(3000, 0)
	_, _ = state.shouldRestart(now, true, policy)
	if state.consecutiveFailures != 1 {
		t.Fatalf("failures=%d, want 1", state.consecutiveFailures)
	}
	_, _ = state.shouldRestart(now.Add(time.Second), false, policy)
	if state.consecutiveFailures != 0 {
		t.Fatal("healthy check should reset confirmation count")
	}
	policy.enabled = false
	state.restartAttempts = []time.Time{now}
	state.cooldownUntil = now.Add(time.Hour)
	_, _ = state.shouldRestart(now.Add(2*time.Second), true, policy)
	if len(state.restartAttempts) != 0 || !state.cooldownUntil.IsZero() {
		t.Fatal("disabling monitor should clear limiter state")
	}
}
