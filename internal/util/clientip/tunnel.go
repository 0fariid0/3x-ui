package clientip

import (
	"net/netip"
	"strings"
)

func IsTunnelLocal(value string) bool {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || ip.Zone() != "" {
		return false
	}
	ip = ip.Unmap()
	if !ip.Is4() {
		return false
	}
	octets := ip.As4()
	return octets[0] == 10 && (octets[1] == 10 || octets[1] == 20 || octets[1] == 30 || octets[1] == 40)
}
