package ais

import "fmt"

// shipTypeLabels maps the AIS "Type of ship and cargo" code (ITU-R M.1371,
// carried in static/voyage data messages) to a human-readable label.
// Codes are grouped in tens: 6x = passenger, 7x = cargo, 8x = tanker, etc.,
// with the last digit refining into a hazard category. Only the codes that
// actually appear in real-world traffic are listed individually; anything
// else falls back to its ten's-group label via TypeLabel.
var shipTypeLabels = map[int]string{
	0: "Not available",

	30: "Fishing",
	31: "Towing",
	32: "Towing (large)",
	33: "Dredging / underwater ops",
	34: "Diving ops",
	35: "Military ops",
	36: "Sailing",
	37: "Pleasure craft",

	50: "Pilot vessel",
	51: "Search and rescue",
	52: "Tug",
	53: "Port tender",
	54: "Anti-pollution equipment",
	55: "Law enforcement",
	58: "Medical transport",

	60: "Passenger",
	69: "Passenger",

	70: "Cargo",
	71: "Cargo (Hazard A)",
	72: "Cargo (Hazard B)",
	73: "Cargo (Hazard C)",
	74: "Cargo (Hazard D)",
	79: "Cargo",

	80: "Tanker",
	81: "Tanker (Hazard A)",
	82: "Tanker (Hazard B)",
	83: "Tanker (Hazard C)",
	84: "Tanker (Hazard D)",
	89: "Tanker",

	90: "Other",
}

// groupLabels is the fallback per ten's-group when the exact code above
// isn't listed (e.g. 62 "Passenger, Hazard B", 41 "HSC, Hazard A").
var groupLabels = map[int]string{
	2: "Wing in ground craft",
	4: "High speed craft",
	6: "Passenger",
	7: "Cargo",
	8: "Tanker",
	9: "Other",
}

// TypeLabel returns a human-readable label for an AIS ship-type code.
// Unknown/reserved codes return "Unknown".
func TypeLabel(code int) string {
	if label, ok := shipTypeLabels[code]; ok {
		return label
	}
	if label, ok := groupLabels[code/10]; ok {
		return label
	}
	return "Unknown"
}

// IsCargoCarrying reports whether a ship-type code belongs to a category
// that plausibly carries freight (cargo or tanker classes). Used to decide
// whether it's meaningful to show an inferred cargo label for a vessel -
// AIS never reports actual cargo contents, only this coarse category.
func IsCargoCarrying(code int) bool {
	group := code / 10
	return group == 7 || group == 8
}

// InferredCargoLabel gives a plausible, clearly-labeled-as-inferred cargo
// description for cargo/tanker-type vessels, derived only from the AIS
// ship-type code. This is NOT real cargo manifest data - no such data is
// publicly available for any vessel - it exists so the UI can show
// something more specific than "Cargo" without pretending to know what's
// actually aboard.
func InferredCargoLabel(code int) string {
	switch {
	case code >= 80 && code <= 89:
		return "Inferred: bulk liquid (oil / chemicals / gas)"
	case code >= 70 && code <= 79:
		return "Inferred: general/container cargo"
	default:
		return fmt.Sprintf("Not a cargo-carrying type (code %d)", code)
	}
}
