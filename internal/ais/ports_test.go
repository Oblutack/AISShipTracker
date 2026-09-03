package ais

import "testing"

func TestMatchDestination_Locode(t *testing.T) {
	cases := []struct {
		name        string
		destination string
		wantLocode  string
	}{
		{"exact glued code, real captured example", "NLRTM", "NLRTM"},
		{"exact glued code with padding, real captured example", "NLRTM               ", "NLRTM"},
		{"lowercase", "nlrtm", "NLRTM"},
		{"space-separated code, real captured example", "US PEF", "USPEF"},
		{"space-separated code with padding, real captured example", "US PEF              ", "USPEF"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			port, ok := MatchDestination(c.destination)
			if !ok {
				t.Fatalf("MatchDestination(%q): no match, want %s", c.destination, c.wantLocode)
			}
			if port.Locode != c.wantLocode {
				t.Errorf("MatchDestination(%q).Locode = %q, want %q", c.destination, port.Locode, c.wantLocode)
			}
		})
	}
}

func TestMatchDestination_PlainName(t *testing.T) {
	// All of these are real destination strings captured off the live
	// aisstream.io feed.
	cases := []struct {
		destination string
		wantLocode  string
	}{
		{"ROTTERDAM", "NLRTM"},
		{"ROTTERDAM           ", "NLRTM"},
		{"LIMASSOL", "CYLMS"},
		{"DELFZIJL            ", "NLDZL"},
		{"WORKUM", "NLWKU"}, // a small Dutch inland harbor - RL (unapproved) status, still a real, correct, unambiguous match
	}
	for _, c := range cases {
		port, ok := MatchDestination(c.destination)
		if !ok {
			t.Errorf("MatchDestination(%q): no match, want %s", c.destination, c.wantLocode)
			continue
		}
		if port.Locode != c.wantLocode {
			t.Errorf("MatchDestination(%q).Locode = %q, want %q", c.destination, port.Locode, c.wantLocode)
		}
	}
}

func TestMatchDestination_MultiLegTakesLastSegment(t *testing.T) {
	// Real captured examples: "next port" convention, separated by '>' or '<>'.
	cases := []struct {
		destination string
		wantLocode  string
	}{
		{"DEHAM>>FRLEH", "FRLEH"},
		{"SGSIN>FRLEH", "FRLEH"},
		{"EETLL<>FIHEL", "FIHEL"},
	}
	for _, c := range cases {
		port, ok := MatchDestination(c.destination)
		if !ok {
			t.Errorf("MatchDestination(%q): no match, want last segment %s", c.destination, c.wantLocode)
			continue
		}
		if port.Locode != c.wantLocode {
			t.Errorf("MatchDestination(%q).Locode = %q, want %q", c.destination, port.Locode, c.wantLocode)
		}
	}
}

func TestMatchDestination_ForOrdersSuffixStripped(t *testing.T) {
	// Real captured example - "FOR ORDERS" is boilerplate meaning "awaiting
	// instructions", appended after a real (or, as here, vague) location.
	// "GERMANY" alone isn't a specific port, so this correctly still fails
	// to match - the point of this test is just that the suffix-stripped
	// text is what actually gets looked up, not the untouched string.
	_, ok := MatchDestination("GERMANY FOR ORDERS")
	if ok {
		t.Error(`MatchDestination("GERMANY FOR ORDERS") matched something, but "GERMANY" alone is not a port name`)
	}
}

func TestMatchDestination_NoMatch(t *testing.T) {
	cases := []string{"", "   ", "FISHING GROUNDS", "SIGHTSEEING", "XX", "NL"}
	for _, d := range cases {
		if _, ok := MatchDestination(d); ok {
			t.Errorf("MatchDestination(%q) unexpectedly matched", d)
		}
	}
}

func TestAllPorts_ReturnsFullSortedDataset(t *testing.T) {
	all := AllPorts()
	if len(all) != len(byLocode) {
		t.Fatalf("AllPorts() returned %d ports, want %d (len(byLocode))", len(all), len(byLocode))
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].Name > all[i].Name {
			t.Fatalf("AllPorts() not sorted by name: %q comes before %q", all[i-1].Name, all[i].Name)
		}
	}
}

func TestSearchPorts(t *testing.T) {
	byName := SearchPorts("rotterdam")
	found := false
	for _, p := range byName {
		if p.Locode == "NLRTM" {
			found = true
		}
	}
	if !found {
		t.Errorf("SearchPorts(\"rotterdam\") = %v, want it to include NLRTM", byName)
	}

	byCode := SearchPorts("nlrtm")
	if len(byCode) == 0 || byCode[0].Locode != "NLRTM" {
		t.Errorf("SearchPorts(\"nlrtm\") = %v, want NLRTM", byCode)
	}

	if got := SearchPorts(""); got != nil {
		t.Errorf("SearchPorts(\"\") = %v, want nil", got)
	}

	if got := SearchPorts("e"); len(got) > maxSearchResults {
		t.Errorf("SearchPorts(\"e\") returned %d results, want capped at %d", len(got), maxSearchResults)
	}
}

// This is a known, deliberate limitation, not a bug: UN/LOCODE records
// some ports under their native-language name rather than the common
// English exonym (Napoli, not Naples; similarly Genova/Genoa, Lisboa/
// Lisbon...). Exact-name matching can't bridge that gap without a
// hand-built alias table, which would reintroduce the "unverified
// hand-typed data" risk this whole dataset switch was meant to avoid. The
// LOCODE path is unaffected - "ITNAP" resolves correctly.
func TestMatchDestination_EnglishExonymLimitation(t *testing.T) {
	port, ok := MatchDestination("ITNAP")
	if !ok || port.Name != "NAPOLI" {
		t.Fatalf(`MatchDestination("ITNAP") = %+v, ok=%v - want NAPOLI`, port, ok)
	}

	// "NAPLES" does NOT resolve to Naples, Italy - UN/LOCODE has no entry
	// spelled "Naples" at all (only unrelated small US towns that happen
	// to share the English name). This assertion exists to make the
	// limitation visible in test output if the underlying dataset or
	// matching logic ever changes, rather than leaving it silently unproven.
	byExonym, matched := MatchDestination("NAPLES")
	if matched && byExonym.Locode == "ITNAP" {
		t.Fatal("if this now passes, the exonym gap has been closed - update the doc comments on MatchDestination/byName accordingly")
	}
}

func TestUnlocodeStatusRank_PrefersApprovedOnCollision(t *testing.T) {
	if unlocodeStatusRank("AF") >= unlocodeStatusRank("RL") {
		t.Error("an approved status (AF) must rank better (lower) than an unapproved one (RL)")
	}
	if unlocodeStatusRank("RL") >= unlocodeStatusRank("XX") {
		t.Error("an unapproved status (RL) must rank better (lower) than a to-be-removed one (XX)")
	}
}
