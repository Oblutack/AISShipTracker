package ais

// midCountry maps the Maritime Identification Digits (the first 3 digits of
// an MMSI, ITU-assigned per country) to a flag/country name.
//
// This table covers the flag states that dominate real-world merchant
// shipping traffic (open registries like Panama/Liberia/Marshall Islands,
// major trading nations, and common EU/Asian flags) rather than the full
// ITU allocation table (~200 entries). It was built from training-data
// knowledge of the standard, documented MID table, not verified against a
// live ITU source at the time this was written - double check a code here
// against the ITU MID list (itu.int) before relying on it for anything
// beyond a UI label. Unrecognized codes return "Unknown flag" rather than
// a guess.
var midCountry = map[int]string{
	201: "Albania", 205: "Belgium", 209: "Cyprus", 210: "Cyprus",
	211: "Germany", 218: "Germany", 219: "Denmark", 220: "Denmark",
	224: "Spain", 225: "Spain", 226: "France", 227: "France", 228: "France",
	229: "Malta", 230: "Finland", 232: "United Kingdom", 233: "United Kingdom",
	234: "United Kingdom", 235: "United Kingdom", 236: "Gibraltar",
	237: "Greece", 238: "Croatia", 239: "Greece", 240: "Greece", 241: "Greece",
	244: "Netherlands", 245: "Netherlands", 246: "Netherlands", 247: "Italy",
	248: "Malta", 249: "Malta", 250: "Ireland", 251: "Iceland",
	256: "Malta", 257: "Norway", 258: "Norway", 259: "Norway",
	261: "Poland", 263: "Portugal", 264: "Romania", 265: "Sweden",
	266: "Sweden", 269: "Switzerland", 271: "Turkey", 273: "Russia",
	275: "Latvia", 276: "Estonia", 277: "Lithuania",

	303: "USA (Alaska)", 304: "Antigua and Barbuda", 305: "Antigua and Barbuda",
	308: "Bahamas", 309: "Bahamas", 310: "Bermuda", 311: "Bahamas",
	312: "Belize", 314: "Barbados", 316: "Canada", 319: "Cayman Islands",
	321: "Costa Rica", 327: "Dominican Republic", 338: "USA", 339: "Jamaica",
	345: "Mexico", 351: "Panama", 352: "Panama", 353: "Panama",
	354: "Panama", 355: "Panama", 356: "Panama", 357: "Panama",
	358: "Puerto Rico", 366: "USA", 367: "USA", 368: "USA", 369: "USA",
	370: "Panama", 371: "Panama", 372: "Panama", 373: "Panama", 374: "Panama",
	375: "Saint Vincent and the Grenadines", 376: "Saint Vincent and the Grenadines",
	377: "Saint Vincent and the Grenadines",

	412: "China", 413: "China", 414: "China", 416: "Taiwan",
	419: "India", 422: "Iran", 431: "Japan", 432: "Japan",
	440: "South Korea", 441: "South Korea", 445: "North Korea",
	453: "Macao", 461: "Oman", 463: "Pakistan", 466: "Qatar",
	470: "UAE", 471: "UAE", 477: "Hong Kong",

	503: "Australia", 512: "New Zealand", 525: "Indonesia",
	533: "Malaysia", 538: "Marshall Islands", 548: "Philippines",
	563: "Singapore", 564: "Singapore", 565: "Singapore", 566: "Singapore",
	567: "Thailand", 574: "Vietnam", 576: "Vanuatu", 577: "Vanuatu",

	601: "South Africa", 636: "Liberia", 637: "Liberia", 667: "Sierra Leone",

	701: "Argentina", 710: "Brazil", 725: "Chile", 730: "Colombia",
	735: "Ecuador", 750: "Guyana", 760: "Peru", 770: "Uruguay", 775: "Venezuela",
}

// Flag returns the flag/country name for an MMSI, derived from its MID
// (first 3 digits). Returns "Unknown flag" if the MMSI is malformed or the
// MID isn't in the table above.
func Flag(mmsi int) string {
	if mmsi < 200000000 || mmsi > 799999999 {
		return "Unknown flag"
	}
	mid := mmsi / 1000000
	if country, ok := midCountry[mid]; ok {
		return country
	}
	return "Unknown flag"
}
