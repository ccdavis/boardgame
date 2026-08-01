package game

import (
	_ "embed"
	"strings"
)

// Operation codenames.
//
// Every standing plan -- an invasion forming, a garrison holding, a squadron
// under orders -- carries a codename, the way both sides named their
// operations in the real war. The names come from vendored wordlists,
// flavoured by POWER where the side's habit would ring false: the UK and USA
// sound like OVERLORD and JUBILEE did (plain words, deliberately
// meaningless); Germany sounds like BARBAROSSA and NORDLICHT (weather, beasts
// and old iron); Italy names its operations in Latin; the USSR uses its real
// operation names (URANUS, BAGRATION) and Russian words; Japan uses its
// operation names as English histories render them (SHO-GO, ICHI-GO) and the
// nature-names its navy favoured. They decorate the transcripts and give a
// human player something to say out loud.
//
// Assignment is by plan ID, so a replayed game christens every operation
// identically, and consecutive operations of one power never share a name
// until a list wraps.

//go:embed opnames_allied.txt
var alliedNamesRaw string

//go:embed opnames_axis.txt
var axisNamesRaw string

//go:embed opnames_italy.txt
var italyNamesRaw string

//go:embed opnames_ussr.txt
var ussrNamesRaw string

//go:embed opnames_japan.txt
var japanNamesRaw string

var (
	alliedNames = parseNameList(alliedNamesRaw)
	axisNames   = parseNameList(axisNamesRaw)

	// powerNames overrides the side list for powers with their own voice.
	// Anyone not listed falls back to the side flavour: the UK and USA on the
	// Allied list, Germany on the Axis list -- and so does any new power a
	// future board might declare.
	powerNames = map[string][]string{
		"Italy": parseNameList(italyNamesRaw),
		"USSR":  parseNameList(ussrNamesRaw),
		"Japan": parseNameList(japanNamesRaw),
	}
)

func parseNameList(raw string) []string {
	var names []string
	for _, line := range strings.Split(raw, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}

// operationName picks the codename for the id-th operation of a power: the
// power's own list when it has one, the side's list otherwise.
func operationName(power, side string, id int) string {
	list := alliedNames // powers of no declared side draw from the Allied list
	if side == "Axis" {
		list = axisNames
	}
	if own, ok := powerNames[power]; ok && len(own) > 0 {
		list = own
	}
	if len(list) == 0 || id < 1 {
		return ""
	}
	return list[(id-1)%len(list)]
}
