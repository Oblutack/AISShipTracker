package ais

import "strings"

// Port is a coordinate we can point a computed route at.
type Port struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

// majorPorts covers a few hundred of the world's busiest container, bulk,
// and tanker ports - built from training-data knowledge of major port
// locations, not a validated port authority (e.g. UN/LOCODE) dataset.
// Coordinates are approximate (port-area/city precision, not a specific
// berth) - good enough to point a line at on a world map, not for
// navigation. This deliberately covers major hubs rather than being
// exhaustive: AIS Destination text is free-form and frequently won't match
// anything here at all (a small fishing village, a terminal name, "FOR
// ORDERS", a typo, blank) - that's expected, not a bug; see MatchDestination.
var majorPorts = []Port{
	// Northern Europe
	{"ROTTERDAM", 51.9244, 4.4777},
	{"ANTWERP", 51.2194, 4.4025},
	{"AMSTERDAM", 52.3676, 4.9041},
	{"HAMBURG", 53.5511, 9.9937},
	{"BREMERHAVEN", 53.5396, 8.5809},
	{"FELIXSTOWE", 51.9539, 1.3513},
	{"SOUTHAMPTON", 50.9097, -1.4044},
	{"LONDON", 51.5072, -0.1276},
	{"LIVERPOOL", 53.4084, -2.9916},
	{"DUBLIN", 53.3498, -6.2603},
	{"LE HAVRE", 49.4944, 0.1079},
	{"ZEEBRUGGE", 51.3308, 3.2078},
	{"GOTHENBURG", 57.7089, 11.9746},
	{"COPENHAGEN", 55.6761, 12.5683},
	{"AARHUS", 56.1629, 10.2039},
	{"OSLO", 59.9139, 10.7522},
	{"BERGEN", 60.3913, 5.3221},
	{"STAVANGER", 58.9700, 5.7331},
	{"HELSINKI", 60.1699, 24.9384},
	{"ST PETERSBURG", 59.9311, 30.3609},
	{"KLAIPEDA", 55.7033, 21.1443},
	{"RIGA", 56.9496, 24.1052},
	{"TALLINN", 59.4370, 24.7536},
	{"GDANSK", 54.3520, 18.6466},
	{"GDYNIA", 54.5189, 18.5305},

	// Southern Europe / Mediterranean
	{"BARCELONA", 41.3851, 2.1734},
	{"VALENCIA", 39.4699, -0.3763},
	{"ALGECIRAS", 36.1408, -5.4562},
	{"MARSEILLE", 43.2965, 5.3698},
	{"GENOA", 44.4056, 8.9463},
	{"GIOIA TAURO", 38.4244, 15.8994},
	{"NAPLES", 40.8518, 14.2681},
	{"LA SPEZIA", 44.1024, 9.8241},
	{"PIRAEUS", 37.9475, 23.6373},
	{"ISTANBUL", 41.0082, 28.9784},
	{"CONSTANTA", 44.1733, 28.6383},
	{"LISBON", 38.7223, -9.1393},
	{"SINES", 37.9564, -8.8647},

	// Middle East / South Asia
	{"DUBAI", 25.2048, 55.2708},
	{"JEBEL ALI", 24.9857, 55.0270},
	{"JEDDAH", 21.4858, 39.1925},
	{"DAMMAM", 26.4207, 50.0888},
	{"SOKHNA", 29.6000, 32.3333},
	{"KARACHI", 24.8607, 67.0011},
	{"MUMBAI", 18.9490, 72.8258},
	{"NHAVA SHEVA", 18.9490, 72.9489},
	{"CHENNAI", 13.0827, 80.2707},
	{"COLOMBO", 6.9271, 79.8612},

	// East / Southeast Asia
	{"SINGAPORE", 1.2644, 103.8200},
	{"HONG KONG", 22.2793, 114.1628},
	{"SHANGHAI", 31.2304, 121.4737},
	{"SHENZHEN", 22.5431, 114.0579},
	{"NINGBO", 29.8683, 121.5440},
	{"QINGDAO", 36.0671, 120.3826},
	{"TIANJIN", 39.0842, 117.2009},
	{"GUANGZHOU", 23.1291, 113.2644},
	{"XIAMEN", 24.4798, 118.0894},
	{"BUSAN", 35.1796, 129.0756},
	{"KAOHSIUNG", 22.6273, 120.3014},
	{"KEELUNG", 25.1276, 121.7392},
	{"TOKYO", 35.6762, 139.6503},
	{"YOKOHAMA", 35.4437, 139.6380},
	{"NAGOYA", 35.1815, 136.9066},
	{"OSAKA", 34.6937, 135.5023},
	{"KOBE", 34.6901, 135.1955},
	{"MANILA", 14.5995, 120.9842},
	{"JAKARTA", -6.1045, 106.8865},
	{"SURABAYA", -7.2575, 112.7521},
	{"PORT KLANG", 3.0000, 101.4000},
	{"TANJUNG PELEPAS", 1.3667, 103.5500},
	{"HO CHI MINH", 10.7626, 106.6602},
	{"HAIPHONG", 20.8449, 106.6881},
	{"BANGKOK", 13.7563, 100.5018},
	{"LAEM CHABANG", 13.0827, 100.8833},

	// Africa
	{"DURBAN", -29.8587, 31.0218},
	{"CAPE TOWN", -33.9249, 18.4241},
	{"LAGOS", 6.5244, 3.3792},
	{"MOMBASA", -4.0435, 39.6682},
	{"ALEXANDRIA", 31.2001, 29.9187},
	{"PORT SAID", 31.2653, 32.3019},
	{"TANGER MED", 35.8833, -5.5000},
	{"CASABLANCA", 33.5731, -7.5898},

	// North America
	{"NEW YORK", 40.6892, -74.0445},
	{"SAVANNAH", 32.0809, -81.0912},
	{"CHARLESTON", 32.7765, -79.9311},
	{"NORFOLK", 36.8508, -76.2859},
	{"HOUSTON", 29.7604, -95.3698},
	{"NEW ORLEANS", 29.9511, -90.0715},
	{"MIAMI", 25.7617, -80.1918},
	{"PORT EVERGLADES", 26.0947, -80.1214},
	{"JACKSONVILLE", 30.3322, -81.6557},
	{"BALTIMORE", 39.2904, -76.6122},
	{"LOS ANGELES", 33.7395, -118.2610},
	{"LONG BEACH", 33.7701, -118.1937},
	{"OAKLAND", 37.8044, -122.2712},
	{"SEATTLE", 47.6062, -122.3321},
	{"TACOMA", 47.2529, -122.4443},
	{"VANCOUVER", 49.2827, -123.1207},
	{"PRINCE RUPERT", 54.3150, -130.3208},
	{"MONTREAL", 45.5019, -73.5674},
	{"HALIFAX", 44.6488, -63.5752},

	// Central / South America
	{"PANAMA", 8.9824, -79.5199},
	{"BALBOA", 8.9500, -79.5667},
	{"COLON", 9.3592, -79.9014},
	{"CARTAGENA", 10.3910, -75.4794},
	{"SANTOS", -23.9608, -46.3336},
	{"RIO DE JANEIRO", -22.9068, -43.1729},
	{"BUENOS AIRES", -34.6037, -58.3816},
	{"CALLAO", -12.0566, -77.1181},

	// Oceania
	{"SYDNEY", -33.8688, 151.2093},
	{"MELBOURNE", -37.8136, 144.9631},
	{"BRISBANE", -27.4698, 153.0251},
	{"FREMANTLE", -32.0569, 115.7439},
	{"AUCKLAND", -36.8485, 174.7633},
}

// MatchDestination tries to resolve a free-text AIS Destination field to a
// known port. AIS destination text has no standard format - real examples
// captured off the live feed include plain city names ("ROTTERDAM"),
// terminal-qualified names, trailing whitespace, and plenty of values that
// won't match anything (blank, "FOR ORDERS", a berth code, a typo). This
// does an exact match first, then a loose substring match in both
// directions; it does not attempt fuzzy/edit-distance matching, so a
// misspelled destination simply won't match - a safe failure (no route
// drawn) rather than a confidently wrong one.
func MatchDestination(destination string) (Port, bool) {
	d := normalizeDestination(destination)
	if d == "" {
		return Port{}, false
	}

	for _, p := range majorPorts {
		if d == p.Name {
			return p, true
		}
	}

	// Substring matching only kicks in above a minimum length - a short
	// fragment ("LA", "SFO") risks landing inside an unrelated port name
	// by coincidence rather than meaning anything.
	if len(d) >= 4 {
		for _, p := range majorPorts {
			if strings.Contains(d, p.Name) || strings.Contains(p.Name, d) {
				return p, true
			}
		}
	}
	return Port{}, false
}

func normalizeDestination(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	s = strings.Join(strings.Fields(s), " ") // collapse internal whitespace
	return s
}
