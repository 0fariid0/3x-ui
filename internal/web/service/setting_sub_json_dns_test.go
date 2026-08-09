package service

import (
	"strings"
	"testing"
)

func TestNormalizeSubJsonDNS(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "default on empty", input: "", want: DefaultSubJsonDNS},
		{name: "trims address", input: " 1.1.1.1 ", want: "1.1.1.1"},
		{name: "doh", input: "https://1.1.1.1/dns-query", want: "https://1.1.1.1/dns-query"},
		{name: "reject newline", input: "1.1.1.1\n8.8.8.8", wantErr: true},
		{name: "reject oversized", input: strings.Repeat("a", 513), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := normalizeSubJsonDNS(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("normalizeSubJsonDNS(%q) unexpectedly succeeded", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeSubJsonDNS(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Fatalf("normalizeSubJsonDNS(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestSubJsonDNSDefaultAndPersistence(t *testing.T) {
	setupSettingTestDB(t)
	s := &SettingService{}

	got, err := s.GetSubJsonDNS()
	if err != nil {
		t.Fatal(err)
	}
	if got != DefaultSubJsonDNS {
		t.Fatalf("default DNS = %q, want %q", got, DefaultSubJsonDNS)
	}

	const custom = "https://1.1.1.1/dns-query"
	if err := s.SetSubJsonDNS(custom); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetSubJsonDNS()
	if err != nil {
		t.Fatal(err)
	}
	if got != custom {
		t.Fatalf("stored DNS = %q, want %q", got, custom)
	}
}
