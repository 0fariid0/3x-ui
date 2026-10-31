package clientip

import "testing"

func TestIsTunnelLocal(t *testing.T) {
	for _, ip := range []string{"10.10.1.1", "10.20.255.2", "10.30.3.1", "10.40.0.2", "::ffff:10.30.3.1", "::ffff:a1e:301", " 10.30.3.1 "} {
		if !IsTunnelLocal(ip) {
			t.Errorf("tunnel IP not recognized: %s", ip)
		}
	}
	for _, ip := range []string{"5.210.158.87", "10.0.0.1", "10.31.3.1", "192.168.1.1", "100.64.1.1", "2001:db8::1", "10.30.3.999", "10.30.3.1/24", "10.30.3.1.evil", ""} {
		if IsTunnelLocal(ip) {
			t.Errorf("unrelated IP exempted: %s", ip)
		}
	}
}
