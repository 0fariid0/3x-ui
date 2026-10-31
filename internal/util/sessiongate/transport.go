package sessiongate

import (
	"context"
	"sync"
)

type streamKey struct{}
type Stream struct {
	mu      sync.Mutex
	closed  bool
	next    uint64
	closers map[uint64]func()
}

func WithStream(ctx context.Context) context.Context {
	return context.WithValue(ctx, streamKey{}, &Stream{closers: make(map[uint64]func())})
}
func AttachTransport(ctx context.Context, closeOwnedStream func()) func() {
	s, _ := ctx.Value(streamKey{}).(*Stream)
	if s == nil {
		return func() {}
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		closeOwnedStream()
		return func() {}
	}
	s.next++
	id := s.next
	s.closers[id] = closeOwnedStream
	s.mu.Unlock()
	return func() { s.mu.Lock(); delete(s.closers, id); s.mu.Unlock() }
}
func CloseTransport(ctx context.Context) {
	s, _ := ctx.Value(streamKey{}).(*Stream)
	if s == nil {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	callbacks := make([]func(), 0, len(s.closers))
	for _, f := range s.closers {
		callbacks = append(callbacks, f)
	}
	clear(s.closers)
	s.mu.Unlock()
	for _, f := range callbacks {
		f()
	}
}
