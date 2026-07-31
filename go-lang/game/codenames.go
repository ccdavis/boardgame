package game

import (
	_ "embed"
	"strings"
)

// Operation codenames.
//
// Every standing plan -- an invasion forming, a garrison holding, a squadron
// under orders -- carries a codename, the way both sides named their
// operations in the real war. The names come from two vendored wordlists,
// flavoured by side: Allied operations sound like OVERLORD and JUBILEE did
// (plain words, deliberately meaningless); Axis operations sound like
// BARBAROSSA and NORDLICHT (weather, beasts and old iron). They decorate the
// transcripts and give a human player something to say out loud.
//
// Assignment is by plan ID, so a replayed game christens every operation
// identically, and consecutive operations of one power never share a name
// until a list wraps.

//go:embed opnames_allied.txt
var alliedNamesRaw string

//go:embed opnames_axis.txt
var axisNamesRaw string

var (
	alliedNames = parseNameList(alliedNamesRaw)
	axisNames   = parseNameList(axisNamesRaw)
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

// operationName picks the codename for the id-th operation of a side.
func operationName(side string, id int) string {
	list := alliedNames // powers of no declared side draw from the Allied list
	if side == "Axis" {
		list = axisNames
	}
	if len(list) == 0 || id < 1 {
		return ""
	}
	return list[(id-1)%len(list)]
}
