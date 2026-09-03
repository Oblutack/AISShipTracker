package ais

// navStatusLabels maps the AIS navigational status code (carried on every
// position report) to a human-readable label. Fixed by ITU-R M.1371.
var navStatusLabels = map[int]string{
	0:  "Under way using engine",
	1:  "At anchor",
	2:  "Not under command",
	3:  "Restricted maneuverability",
	4:  "Constrained by draught",
	5:  "Moored",
	6:  "Aground",
	7:  "Engaged in fishing",
	8:  "Under way sailing",
	11: "Power-driven vessel towing astern",
	12: "Power-driven vessel pushing ahead",
	14: "AIS-SART / MOB / EPIRB",
	15: "Not defined",
}

// NavStatusLabel returns a human-readable label for an AIS navigational
// status code, or "Not defined" for unrecognized codes.
func NavStatusLabel(code int) string {
	if label, ok := navStatusLabels[code]; ok {
		return label
	}
	return "Not defined"
}
