package ais

import "testing"

func TestMatchDestination(t *testing.T) {
	cases := []struct {
		name        string
		destination string
		wantMatch   bool
		wantPort    string
	}{
		{"exact, real captured example", "ROTTERDAM", true, "ROTTERDAM"},
		{"trailing whitespace, real captured example", "DORDRECHT   ", false, ""}, // not in our major-port list
		{"lowercase + spacing normalizes", "  rotterdam ", true, "ROTTERDAM"},
		{"substring qualifier", "ROTTERDAM EUROPOORT", true, "ROTTERDAM"},
		{"empty destination", "", false, ""},
		{"short garbage doesn't false-match", "NL", false, ""},
		{"unresolvable free text", "FOR ORDERS", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			port, ok := MatchDestination(c.destination)
			if ok != c.wantMatch {
				t.Fatalf("MatchDestination(%q) matched=%v, want %v (port=%v)", c.destination, ok, c.wantMatch, port)
			}
			if ok && port.Name != c.wantPort {
				t.Errorf("MatchDestination(%q) = %q, want %q", c.destination, port.Name, c.wantPort)
			}
		})
	}
}
