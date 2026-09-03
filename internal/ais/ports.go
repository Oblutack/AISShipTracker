package ais

import (
	_ "embed"
	"encoding/csv"
	"sort"
	"strconv"
	"strings"
)

// Port is a coordinate we can point a computed route at.
type Port struct {
	Locode string  `json:"locode"`
	Name   string  `json:"name"`
	Lat    float64 `json:"lat"`
	Lon    float64 `json:"lon"`
}

// portsCSV is the official UN/LOCODE dataset (UNECE), sourced via the
// community-maintained mirror at github.com/datasets/un-locode
// (data/code-list.csv), filtered down to rows where:
//   - Function starts with "1" (the location is flagged as a seaport), and
//   - Coordinates is non-empty
//
// ...then trimmed to just the columns this package needs (Country,
// Location, Name, Status, Coordinates). 11,829 rows as of the 2025
// edition. This replaces an earlier hand-typed ~150-port table built from
// training-data knowledge alone - that table's coordinates and coverage
// were never independently verified; this dataset is the authoritative
// source UN/LOCODE codes themselves are drawn from, so both the coverage
// and the coordinates are far more trustworthy.
//
// Two known gaps, confirmed against real data, neither of which this
// package can fix by parsing harder - both are gaps in the source data
// itself:
//   - Coordinates is genuinely blank for some well-known ports in UN/
//     LOCODE's own data, including major ones - e.g. GBSOU (Southampton)
//     and ESBCN (Barcelona) both have a valid, correct code and name but
//     no coordinates at all, so a destination of "GB SOU" or "BARCELONA"
//     will not resolve even though the port is real and well-known.
//   - Some ports are recorded under a native-language name rather than
//     the common English exonym (Napoli, not Naples; likewise Genova,
//     Lisboa, ...). A destination string using the English name won't
//     match unless it happens to also be recorded that way - see
//     TestMatchDestination_EnglishExonymLimitation.
//
// To regenerate: download data/code-list.csv from
// https://github.com/datasets/un-locode, then keep rows where
// Coordinates != "" and Function[:1] == "1", keeping only the Country,
// Location, Name, Status, Coordinates columns.
//
//go:embed data/unlocode_ports.csv
var portsCSV string

var (
	// byLocode is keyed by the 5-letter UN/LOCODE (country+location, e.g.
	// "NLRTM"). Exact and unambiguous - this is what a destination like
	// "NLRTM" or "NL RTM" resolves through.
	byLocode map[string]Port

	// byName is keyed by normalized port name. Ambiguous by nature - UN/
	// LOCODE has multiple locations sharing the same city name across
	// different countries (e.g. plain "NAPLES" collides with a ~700-
	// person town in Maine, not just Naples, Italy - which UN/LOCODE
	// itself records under its native name, "Napoli", so an English-
	// language "NAPLES" destination string won't match it either way).
	// Collisions are resolved by unlocodeStatusRank (an approved entry
	// beats an unverified/requested one); ties after that keep whichever
	// was seen first. This is UN/LOCODE's own confidence signal, not an
	// invented heuristic, but it's still best-effort - there's no
	// traffic/size ranking in the data to fall back on.
	byName map[string]Port
)

func init() {
	byLocode = make(map[string]Port, 12000)
	byName = make(map[string]Port, 12000)
	nameRank := make(map[string]int, 12000)

	r := csv.NewReader(strings.NewReader(portsCSV))
	rows, err := r.ReadAll()
	if err != nil {
		panic("ais: failed to parse embedded UN/LOCODE data: " + err.Error())
	}
	for _, row := range rows[1:] { // skip header
		country, location, name, status, coords := row[0], row[1], row[2], row[3], row[4]
		lat, lon, ok := parseUNLOCODECoordinates(coords)
		if !ok {
			continue
		}
		p := Port{Locode: country + location, Name: strings.ToUpper(name), Lat: lat, Lon: lon}
		byLocode[p.Locode] = p

		rank := unlocodeStatusRank(status)
		if existingRank, exists := nameRank[p.Name]; !exists || rank < existingRank {
			byName[p.Name] = p
			nameRank[p.Name] = rank
		}
	}
}

// unlocodeStatusRank scores a UN/LOCODE Status code by how much
// confidence the Maintenance Agency has in the entry - lower is more
// trustworthy. Used only to pick a winner when multiple locations share a
// name; see byName.
func unlocodeStatusRank(status string) int {
	switch {
	case strings.HasPrefix(status, "A"): // AA/AC/AF/AI/AM/AQ/AS - approved by someone
		return 0
	case status == "QQ": // was verified once, just stale
		return 1
	case strings.HasPrefix(status, "R"): // RL/RN/RQ/RR - not yet approved, or rejected
		return 2
	case status == "XX": // being removed from the next edition
		return 3
	default:
		return 2
	}
}

// parseUNLOCODECoordinates parses UN/LOCODE's fixed-width coordinate
// format: 4-digit latitude (DDMM) + N/S, a space, 5-digit longitude
// (DDDMM) + E/W - e.g. "5155N 00430E". Verified against the live dataset:
// every one of the 11,829 filtered rows is exactly this 12-character shape.
func parseUNLOCODECoordinates(s string) (lat, lon float64, ok bool) {
	if len(s) != 12 {
		return 0, 0, false
	}
	latDeg, err1 := strconv.Atoi(s[0:2])
	latMin, err2 := strconv.Atoi(s[2:4])
	latDir := s[4]
	lonDeg, err3 := strconv.Atoi(s[6:9])
	lonMin, err4 := strconv.Atoi(s[9:11])
	lonDir := s[11]
	if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
		return 0, 0, false
	}

	lat = float64(latDeg) + float64(latMin)/60
	if latDir == 'S' {
		lat = -lat
	} else if latDir != 'N' {
		return 0, 0, false
	}

	lon = float64(lonDeg) + float64(lonMin)/60
	if lonDir == 'W' {
		lon = -lon
	} else if lonDir != 'E' {
		return 0, 0, false
	}

	return lat, lon, true
}

// MatchDestination tries to resolve a free-text AIS Destination field to a
// known port. AIS destination text has no enforced format - real examples
// captured off the live feed include exact UN/LOCODE codes ("NLRTM"),
// space-separated codes ("GB SOU"), plain port names ("ROTTERDAM"),
// multi-leg strings using '>' or '<' as a separator ("DEHAM>>FRLEH",
// meaning "next stop FRLEH"), a trailing "FOR ORDERS"/"FOR ORDER" awaiting-
// instructions suffix, and plenty of values that will never resolve to
// anything (blank, a terminal name not in UN/LOCODE, a typo, "XX").
//
// Resolution order, most to least reliable:
//  1. Exact UN/LOCODE match (5-letter glued code, or "XX YYY" spaced form)
//  2. Exact name match
//
// No fuzzy/substring matching is attempted at this dataset's scale
// (~12,000 entries) - it was viable against a curated ~150-port list, but
// a loose substring match against 12,000 names has a much higher chance of
// landing on an unrelated place that happens to share a few letters, which
// would fail silently-wrong rather than safely-no-match.
func MatchDestination(destination string) (Port, bool) {
	d := normalizeDestination(destination)
	if d == "" {
		return Port{}, false
	}
	d = strings.TrimSuffix(d, " FOR ORDERS")
	d = strings.TrimSuffix(d, " FOR ORDER")

	// Multi-leg strings ("A>B", "A<>B", "A>>B") - the segment after the
	// last separator is conventionally the next/final destination.
	candidates := []string{d}
	if last := lastSegment(d, ">", "<"); last != "" && last != d {
		candidates = []string{last, d}
	}

	for _, c := range candidates {
		if p, ok := matchLocode(c); ok {
			return p, true
		}
		if p, ok := byName[c]; ok {
			return p, true
		}
	}
	return Port{}, false
}

// maxSearchResults caps SearchPorts' response size - a broad query (e.g.
// a single common letter) could otherwise match thousands of entries.
const maxSearchResults = 50

// AllPorts returns every port in the loaded UN/LOCODE dataset (11,829 as
// of the 2025 edition), sorted by name. This is real, authoritative data -
// see the doc comment on portsCSV for its source, coverage, and the two
// known gaps in it.
func AllPorts() []Port {
	out := make([]Port, 0, len(byLocode))
	for _, p := range byLocode {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// SearchPorts returns ports whose name or LOCODE contains the query
// (case-insensitive), sorted by name and capped at maxSearchResults.
// Returns nil for an empty/whitespace-only query.
func SearchPorts(query string) []Port {
	q := normalizeDestination(query)
	if q == "" {
		return nil
	}
	var out []Port
	for _, p := range byLocode {
		if strings.Contains(p.Name, q) || strings.Contains(p.Locode, q) {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(out) > maxSearchResults {
		out = out[:maxSearchResults]
	}
	return out
}

// matchLocode recognizes a bare 5-letter code ("NLRTM") or a 2+3 code
// split by whitespace in either order ("NL RTM", "GB SOU").
func matchLocode(s string) (Port, bool) {
	compact := strings.ReplaceAll(s, " ", "")
	if len(compact) == 5 && isAlpha(compact) {
		if p, ok := byLocode[compact]; ok {
			return p, true
		}
	}
	return Port{}, false
}

func isAlpha(s string) bool {
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// lastSegment splits s on runs of any of the given separator characters
// and returns the last non-empty, trimmed segment - or "" if none of the
// separators appear at all.
func lastSegment(s string, seps ...string) string {
	found := false
	cut := s
	for _, sep := range seps {
		cut = strings.ReplaceAll(cut, sep, ">")
		if strings.Contains(s, sep) {
			found = true
		}
	}
	if !found {
		return ""
	}
	parts := strings.Split(cut, ">")
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.TrimSpace(parts[i])
		if p != "" {
			return p
		}
	}
	return ""
}

func normalizeDestination(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ") // collapse internal whitespace
	return s
}
