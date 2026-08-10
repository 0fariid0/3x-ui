package xray

import (
	"slices"
	"testing"
)

func newOnlineTestProcess() *Process {
	return &Process{newProcess(nil)}
}

func assertSameSet(t *testing.T, label string, got, want []string) {
	t.Helper()
	g := append([]string(nil), got...)
	w := append([]string(nil), want...)
	slices.Sort(g)
	slices.Sort(w)
	if !slices.Equal(g, w) {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// TestMergedNodeTreesScopesPerGuid pins #4983/#4809: each node's clients stay
// under that node's GUID, so a client on one node is never attributed to
// another — and a sub-node's clients (reported under their own GUID inside a
// parent's tree) compose upward without collapsing onto the parent.
func TestMergedNodeTreesScopesPerGuid(t *testing.T) {
	p := newOnlineTestProcess()
	// Node A (direct) reports its own clients plus sub-node B's tree.
	p.SetNodeOnlineTree(1, map[string][]string{
		"guid-a": {"user1", "user2"},
		"guid-b": {"user3"}, // B is behind A; still keyed by B's own GUID
	})
	p.SetNodeOnlineTree(2, map[string][]string{
		"guid-c": {"user4"},
	})

	merged := p.GetMergedNodeTrees()
	assertSameSet(t, "guid-a", merged["guid-a"], []string{"user1", "user2"})
	assertSameSet(t, "guid-b", merged["guid-b"], []string{"user3"})
	assertSameSet(t, "guid-c", merged["guid-c"], []string{"user4"})

	if slices.Contains(merged["guid-a"], "user3") {
		t.Errorf("user3 (on sub-node B) leaked onto node A: %v", merged["guid-a"])
	}
}

// TestMergedNodeTreesOmitsEmpty keeps the payload small: empty GUID sets don't
// appear as keys.
func TestMergedNodeTreesOmitsEmpty(t *testing.T) {
	p := newOnlineTestProcess()
	p.SetNodeOnlineTree(1, map[string][]string{
		"guid-a": {"user1"},
		"guid-x": {},
	})
	if _, ok := p.GetMergedNodeTrees()["guid-x"]; ok {
		t.Errorf("empty GUID set should be omitted: %v", p.GetMergedNodeTrees())
	}
}

// TestGetOnlineClientsUnionDedupes confirms the flat union (client-centric /
// total-count views) merges local + every node and dedupes.
func TestGetOnlineClientsUnionDedupes(t *testing.T) {
	p := newOnlineTestProcess()
	p.RefreshLocalOnline([]string{"user1"}, nil, 1000, 20000)
	p.SetNodeOnlineTree(1, map[string][]string{"guid-a": {"user1", "user2"}})

	assertSameSet(t, "union", p.GetOnlineClients(), []string{"user1", "user2"})
}

// TestRefreshLocalOnlineGraceWindow checks the in-memory local set honours the
// grace window: idle-but-recent clients stay online, stale ones age out, and
// the set is derived only from local activity (never the shared DB column).
func TestRefreshLocalOnlineGraceWindow(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = 20000

	p.RefreshLocalOnline([]string{"user1"}, nil, 1000, grace)
	if got := p.GetLocalOnlineClients(); !slices.Contains(got, "user1") {
		t.Fatalf("user1 should be online right after activity, got %v", got)
	}

	p.RefreshLocalOnline([]string{"user2"}, nil, 11000, grace)
	got := p.GetLocalOnlineClients()
	if !slices.Contains(got, "user1") || !slices.Contains(got, "user2") {
		t.Fatalf("both within grace window, got %v", got)
	}

	p.RefreshLocalOnline(nil, nil, 22000, grace)
	got = p.GetLocalOnlineClients()
	if slices.Contains(got, "user1") {
		t.Errorf("user1 (idle 21s, past grace) should have aged out, got %v", got)
	}
	if !slices.Contains(got, "user2") {
		t.Errorf("user2 (idle 11s, within grace) should still be online, got %v", got)
	}
}

// TestGetLocalActiveInboundsTracksGraceWindow pins exact email/inbound
// attribution: a multi-inbound client counts online only on tags observed with
// that same email in the access log, and stale tag observations age out.
func TestGetLocalActiveInboundsTracksGraceWindow(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = 20000

	p.RefreshLocalOnline([]string{"alice"}, nil, 1000, grace)
	p.RefreshLocalOnlineInbounds(map[string][]string{"alice": {"inbound-a"}}, 1000, grace)
	assertSameSet(t, "active after first observation", p.GetLocalActiveInbounds(), []string{"inbound-a"})

	p.RefreshLocalOnline([]string{"alice"}, nil, 11000, grace)
	p.RefreshLocalOnlineInbounds(map[string][]string{"alice": {"inbound-b"}}, 11000, grace)
	assertSameSet(t, "both within grace", p.GetLocalActiveInbounds(), []string{"inbound-a", "inbound-b"})

	// Keep the email itself online while pruning only stale inbound observations.
	p.RefreshLocalOnline([]string{"alice"}, nil, 22000, grace)
	p.RefreshLocalOnlineInbounds(nil, 22000, grace)
	assertSameSet(t, "inbound-a (idle 21s) aged out, inbound-b kept", p.GetLocalActiveInbounds(), []string{"inbound-b"})

	p.RefreshLocalOnline([]string{"alice"}, nil, 40000, grace)
	p.RefreshLocalOnlineInbounds(nil, 40000, grace)
	if got := p.GetLocalActiveInbounds(); len(got) != 0 {
		t.Errorf("all inbound observations idle past grace, want empty, got %v", got)
	}
}

// TestExactInboundAttributionDoesNotCrossClients is the regression test for
// the panel bug fixed in v3.6.25. Aggregate user/inbound activity can tell us
// that Alice and Bob are online and that A and B are active, but it cannot tell
// which user used which inbound. The exact map must preserve those pairs.
func TestExactInboundAttributionDoesNotCrossClients(t *testing.T) {
	p := newOnlineTestProcess()
	const grace = 20000

	p.RefreshLocalOnline([]string{"alice", "bob"}, nil, 1000, grace)
	p.RefreshLocalOnlineInbounds(map[string][]string{
		"alice": {"inbound-a"},
		"bob":   {"inbound-b"},
	}, 1000, grace)

	exact := p.GetLocalOnlineInbounds()
	assertSameSet(t, "alice exact inbounds", exact["alice"], []string{"inbound-a"})
	assertSameSet(t, "bob exact inbounds", exact["bob"], []string{"inbound-b"})
	if slices.Contains(exact["alice"], "inbound-b") {
		t.Fatalf("bob's inbound leaked onto alice: %v", exact)
	}
	if slices.Contains(exact["bob"], "inbound-a") {
		t.Fatalf("alice's inbound leaked onto bob: %v", exact)
	}
}

// TestMergedNodeOnlineInboundTreesScopesPerGuid keeps exact attribution intact
// through a multi-node chain and deduplicates repeated tags.
func TestMergedNodeOnlineInboundTreesScopesPerGuid(t *testing.T) {
	p := newOnlineTestProcess()
	p.SetNodeOnlineInboundTree(1, map[string]map[string][]string{
		"guid-a": {"alice": {"in-a", "in-a"}},
		"guid-b": {"bob": {"in-b"}},
	})
	p.SetNodeOnlineInboundTree(2, map[string]map[string][]string{
		"guid-c": {"carol": {"in-c"}},
	})

	merged := p.GetMergedNodeOnlineInboundTrees()
	assertSameSet(t, "guid-a/alice", merged["guid-a"]["alice"], []string{"in-a"})
	assertSameSet(t, "guid-b/bob", merged["guid-b"]["bob"], []string{"in-b"})
	assertSameSet(t, "guid-c/carol", merged["guid-c"]["carol"], []string{"in-c"})
	if _, leaked := merged["guid-a"]["bob"]; leaked {
		t.Fatalf("guid-b client leaked into guid-a: %v", merged)
	}
}

// TestClearNodeOnlineClientsDropsNode mirrors a failed node probe: the node's
// whole subtree contribution disappears immediately.
func TestClearNodeOnlineClientsDropsNode(t *testing.T) {
	p := newOnlineTestProcess()
	p.SetNodeOnlineTree(3, map[string][]string{"guid-a": {"user1"}})
	p.ClearNodeOnlineClients(3)

	if _, ok := p.GetMergedNodeTrees()["guid-a"]; ok {
		t.Errorf("node 3's subtree should be absent after ClearNodeOnlineClients")
	}
}

// TestOnlineAPISupportTriState pins the lazy capability probe contract: a new
// process starts Unknown (so the first caller probes), and the flag holds
// whatever the probe recorded until the process is replaced on restart.
func TestOnlineAPISupportTriState(t *testing.T) {
	p := newOnlineTestProcess()
	if got := p.OnlineAPISupport(); got != OnlineAPIUnknown {
		t.Fatalf("new process must start with OnlineAPIUnknown, got %v", got)
	}
	p.SetOnlineAPISupport(OnlineAPISupported)
	if got := p.OnlineAPISupport(); got != OnlineAPISupported {
		t.Fatalf("expected OnlineAPISupported, got %v", got)
	}
	p.SetOnlineAPISupport(OnlineAPIUnsupported)
	if got := p.OnlineAPISupport(); got != OnlineAPIUnsupported {
		t.Fatalf("expected OnlineAPIUnsupported, got %v", got)
	}
}
