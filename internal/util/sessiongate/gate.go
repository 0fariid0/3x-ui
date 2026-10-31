// Package sessiongate provides account/source stream isolation. The pinned
// Fara core build copies this implementation and attaches transport lifetimes.
package sessiongate

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"
)

type Key struct {
	Email string
	IP    netip.Addr
}
type Gate struct {
	mu      sync.Mutex
	next    uint64
	active  map[Key]map[uint64]func()
	blocked map[Key]time.Time
}

func key(email, ip string) (Key, error) {
	if strings.HasPrefix(ip, "[") && strings.HasSuffix(ip, "]") {
		ip = ip[1 : len(ip)-1]
	}
	a, e := netip.ParseAddr(ip)
	if e != nil || a.Zone() != "" || email == "" {
		return Key{}, fmt.Errorf("invalid account or source")
	}
	return Key{email, a.Unmap()}, nil
}

// Register must receive a callback that terminates only this authenticated stream.
// release is idempotent and must run when that stream actually terminates.
func (g *Gate) Register(email, ip string, closeStream func(), now time.Time) (release func(), accepted bool, err error) {
	k, err := key(email, ip)
	if err != nil || closeStream == nil {
		if err == nil {
			err = fmt.Errorf("missing stream closer")
		}
		return nil, false, err
	}
	g.mu.Lock()
	if until, ok := g.blocked[k]; ok && (until.IsZero() || now.Before(until)) {
		g.mu.Unlock()
		closeStream()
		return func() {}, false, nil
	}
	delete(g.blocked, k)
	if g.active == nil {
		g.active = make(map[Key]map[uint64]func())
	}
	if g.active[k] == nil {
		g.active[k] = make(map[uint64]func())
	}
	g.next++
	id := g.next
	g.active[k][id] = closeStream
	g.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			delete(g.active[k], id)
			if len(g.active[k]) == 0 {
				delete(g.active, k)
			}
		})
	}, true, nil
}

// Block closes streams outside the mutex; callbacks may release their registration.
func (g *Gate) Block(email, ip string, until time.Time) (int, error) {
	k, err := key(email, ip)
	if err != nil {
		return 0, err
	}
	g.mu.Lock()
	if g.blocked == nil {
		g.blocked = make(map[Key]time.Time)
	}
	g.blocked[k] = until
	callbacks := make([]func(), 0, len(g.active[k]))
	for _, f := range g.active[k] {
		callbacks = append(callbacks, f)
	}
	delete(g.active, k)
	g.mu.Unlock()
	for _, f := range callbacks {
		f()
	}
	return len(callbacks), nil
}
func (g *Gate) Unblock(email, ip string) error {
	k, err := key(email, ip)
	if err != nil {
		return err
	}
	g.mu.Lock()
	delete(g.blocked, k)
	g.mu.Unlock()
	return nil
}

var Default Gate

type Block struct {
	Email     string
	IP        string
	ExpiresAt int64
}

func (g *Gate) Blocks(now time.Time) []Block {
	g.mu.Lock()
	defer g.mu.Unlock()
	out := make([]Block, 0, len(g.blocked))
	for k, until := range g.blocked {
		if !until.IsZero() && !now.Before(until) {
			delete(g.blocked, k)
			continue
		}
		expires := int64(0)
		if !until.IsZero() {
			expires = until.UnixMilli()
		}
		out = append(out, Block{k.Email, k.IP.String(), expires})
	}
	return out
}
