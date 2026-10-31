package sessiongate

import (
	"context"
	"testing"
	"time"
)

func TestIsolationExpiryAndReentrantRelease(t *testing.T) {
	g := &Gate{}
	now := time.Now()
	a, b, c := 0, 0, 0
	var release func()
	release, ok, err := g.Register("a", "192.0.2.1", func() { a++; release() }, now)
	if err != nil || !ok {
		t.Fatal(err)
	}
	_, _, _ = g.Register("b", "192.0.2.1", func() { b++ }, now)
	_, _, _ = g.Register("a", "192.0.2.2", func() { c++ }, now)
	count, err := g.Block("a", "::ffff:192.0.2.1", now.Add(time.Minute))
	if err != nil || count != 1 || a != 1 || b != 0 || c != 0 {
		t.Fatalf("wrong victim %d %d %d %d %v", count, a, b, c, err)
	}
	_, ok, _ = g.Register("a", "192.0.2.1", func() { a++ }, now)
	if ok || a != 2 {
		t.Fatal("blocked reconnect accepted")
	}
	release, ok, _ = g.Register("a", "192.0.2.1", func() {}, now.Add(2*time.Minute))
	if !ok {
		t.Fatal("expired block retained")
	}
	release()
	release()
}

func TestBracketedCoreSourceMatchesLiteralTarget(t *testing.T) {
	g := &Gate{}
	closed := false
	_, ok, e := g.Register("source-user", "[2001:db8::1]", func() { closed = true }, time.Now())
	if e != nil || !ok {
		t.Fatal(e)
	}
	n, e := g.Block("source-user", "2001:db8::1", time.Now().Add(time.Minute))
	if e != nil || n != 1 || !closed {
		t.Fatal(n, e)
	}
}
func TestTransportCloseBeforeRegistrationDoesNotLeak(t *testing.T) {
	ctx := WithStream(context.Background())
	CloseTransport(ctx)
	closed := 0
	release := AttachTransport(ctx, func() { closed++ })
	release()
	CloseTransport(ctx)
	if closed != 1 {
		t.Fatal(closed)
	}
	other := WithStream(context.Background())
	_ = AttachTransport(other, func() { closed++ })
	CloseTransport(ctx)
	if closed != 1 {
		t.Fatal("another stream was affected")
	}
	CloseTransport(other)
	if closed != 2 {
		t.Fatal("owned transport not closed")
	}
}
