"""Mapping between aaa.gdf territory names and TripleA "World War II Classic" names.

This is the reviewable artifact of the geometry pipeline. Everything else is
mechanical; the correctness of the generated map rests on this table.

Two things worth knowing before editing:

1. `aaa.gdf` names carry load-bearing misspellings ("Caucases", "Philipines",
   "Kazakstan", "Mediteranian", "Carribean", "Hawaian"). They are the literal
   keys the game engine uses. Do not "fix" them here.

2. `aaa.gdf` is closer to TripleA's *classic* (1984 1st edition) board than it is
   to the higher-resolution board scan in the repo, which is a later edition.
   Both aaa.gdf and TripleA merge Norway+Finland and Syria+Jordan, and neither
   has Byelorussia. The scan has all three separately. Where the board is being
   extended to match the scan, the TripleA region has to be split -- see SPLITS.
"""

# --- Land -------------------------------------------------------------------
#
# gdf name -> TripleA polygon name.
#
# Where several gdf names share one TripleA name, that region is cut apart by a
# corresponding entry in SPLITS.
LAND = {
    # Europe
    "Germany":                   "Germany",
    "Britain":                   "United Kingdom",
    "Western Europe":            "West Europe",
    "Southern Europe":           "South Europe",
    "Eastern Europe":            "East Europe",
    "Spain":                     "Spain",
    "Sweden":                    "Sweden",
    "Switzerland":               "Switzerland",
    # USSR
    "Karelia":                   "Karelia S.S.R.",
    "Russia":                    "Russia",
    "Ukraine":                   "Ukraine S.S.R.",
    "Caucases":                  "Caucasus",
    "Kazakstan":                 "Kazakh S.S.R.",
    "Yakutsk SSR":               "Yakut S.S.R.",
    "Evenki":                    "Evenki National Okrug",
    "Novosibirsk":               "Novosibirsk",
    "Soviet Far East":           "Soviet Far East",
    # Asia
    "Mongolia":                  "Mongolia",
    "Afghanistan":               "Afghanistan",
    "Manchuria":                 "Manchuria",
    "Sinkiang Western China":    "Sinkiang",
    "Kwantung Eastern China":    "Kwangtung",
    "Central China":             "China",
    "Japan":                     "Japan",
    "Burma and South East Asia": "French Indo China",
    "India":                     "India",
    # Pacific
    "East Indies":               "East Indies",
    "Philipines":                "Philippines",
    "New Guinea":                "New Guinea",
    "Australia":                 "Australia",
    "New Zealand":               "New Zealand",
    "Caroline Islands":          "Caroline Islands",
    "Solomon Islands":           "Solomon Islands",
    "Hawaii":                    "Hawaiian Islands",
    "Wake Island":               "Wake Island",
    "Midway Island":             "Midway",
    "Borneo":                    "Borneo Celebes",
    "Okinawa":                   "Okinawa",
    # North America
    "Alaska":                    "Alaska",
    "Western Canada":            "West Canada",
    "Eastern Canada":            "East Canada",
    "Western US":                "West US",
    "Eastern US":                "East US",
    "Mexico":                    "Mexico",
    "Central America":           "Panama",
    "West Indies":               "Cuba",
    # South America
    "Colombia":                  "Columbia",        # split
    "Venezuela":                 "Columbia",        # split
    "Brazil":                    "Brazil",
    "Peru":                      "Peru",
    "Chile":                     "Argentina-Chile",  # split
    "Argentina":                 "Argentina-Chile",  # split
    # Middle East
    "Turkey":                    "Turkey",
    "Palestine":                 "Syria Jordan",     # split
    "Syria":                     "Syria Jordan",     # split
    "Iraq":                      "Syria Jordan",     # split
    "Iran":                      "Persia",
    "Arabia":                    "Saudi Arabia",
    # Africa
    "Algeria":                   "Algeria",
    # TripleA draws Rio del Oro (Western Sahara) where this board puts Morocco.
    "Morocco":                   "Rio del Oro",
    "Libya":                     "Libya",
    "Egypt":                     "Anglo Sudan Egypt",
    "French West Africa":        "French West Africa",
    "French East Africa":        "French Equatorial Africa",
    "Ethiopia":                  "Italian East Africa",
    "Kenya":                     "Kenya-Rhodesia",
    "Congo":                     "Congo",
    "South Africa":              "South Africa",
    "Madagascar":                "Madagascar",
    "Mozambique":                "Mozambique",
    "Angola":                    "Angola",
}

# TripleA land regions with no counterpart in the current aaa.gdf.
# "Sweden", "Switzerland" and "Rio del Oro" (~Morocco) become real territories
# when the board is extended; "Eire" and "Gibraltar" are not in scope.
UNUSED_LAND = ["Eire", "Gibraltar"]

# --- Sea --------------------------------------------------------------------
#
# The board prints no names on most sea zones, so the aaa.gdf names are the
# author's own invention and cannot be matched to TripleA by name. They are
# resolved structurally instead: a sea zone is identified by the set of land
# territories it borders. See match_seas.py, which generates SEA below.
SEA = {}

# --- Merges -----------------------------------------------------------------
#
# TripleA draws these zones more finely than aaa.gdf does, so several TripleA
# polygons become one gdf territory. See DECISION.md for how these were found.
MERGES = {
    # TripleA has three Mediterranean zones; aaa.gdf has two.
    "Eastern Mediteranian": ["Central Mediteranean Sea Zone", "East Mediteranean Sea Zone"],
    "Eastern USA Atlantic": ["East US Sea Zone", "Gulf of Mexico Sea Zone"],
    "Mexican Pacific":      ["Mexico Sea Zone", "West Panama Sea Zone"],
}

# TripleA regions with no counterpart at all in aaa.gdf, deliberately dropped.
# The Caspian is an inland sea: aaa.gdf has Russia, Kazakstan, Caucases and Iran
# bordering each other directly. Leaving it out renders it as open water, which
# is what a lake should look like anyway.
DROP = ["Caspian Sea Zone"]

# --- Splits -----------------------------------------------------------------
#
# One TripleA region -> several gdf territories. The anchor points that drive
# each split live in splits.json; this list exists so the two files can be
# checked against each other at build time.
SPLITS = {
    "Columbia":        ["Colombia", "Venezuela"],
    "Argentina-Chile": ["Chile", "Argentina"],
    "Syria Jordan":    ["Palestine", "Syria", "Iraq"],
    # Needed only once the board is extended (M0d). Until then the split falls
    # back to the whole region, so no hole appears in the map.
    "Finland Norway":  ["Norway", "Finland"],
    "East Europe":     ["Eastern Europe", "Byelorussia"],
    "South Europe":    ["Southern Europe", "Italy"],
    # aaa.gdf subdivides the Indian Ocean more finely than TripleA does; these
    # are the four extra zones identified by the decision gate.
    "Indian Ocean Sea Zone": ["Indian Ocean", "Bay of Bengal",
                              "North Central Indian Ocean"],
    "West Compass Sea Zone": ["Central Indian Ocean", "Southern Indian Ocean"],
    "East Compass Sea Zone": ["North East Indian Ocean", "Eastern Indian Ocean"],
}

# Sea zones whose structural match is wrong and is pinned by hand. The matcher
# only had sea-neighbour topology to go on for open ocean, and the Indian Ocean
# is where the two boards genuinely disagree.
SEA_OVERRIDE = {
    "Antarctic Ocean southwest of Australia": "South Compass Sea Zone",
}


def land_to_triplea():
    return dict(LAND)


def triplea_to_land():
    """Reverse map. Values are lists because splits are many-to-one."""
    out = {}
    for gdf_name, tri_name in LAND.items():
        out.setdefault(tri_name, []).append(gdf_name)
    return out
