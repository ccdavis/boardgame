package models

import "strings"

// UnitCapabilities describes what a unit type can do, independent of what it is
// called.
//
// Rules used to be keyed on literal names scattered through the codebase --
// combat looked for "submarine", "destroyer", "cruiser" and "artillery"; the AI
// looked for "factory" in some places and "industrial_complex" in others.
// aaa.gdf calls those units "sub" and "factory", and defines no destroyer,
// cruiser or artillery at all. The result was that submarine first strike,
// destroyer negation, shore bombardment and artillery support were all dead
// against the real board, silently, while their tests passed against fixtures
// that used the other spellings.
//
// Capabilities are derived from the unit roster the board declares, so a board
// may name its units whatever it likes.
type UnitCapabilities struct {
	// IsStructure marks a unit that is captured with its territory rather than
	// fought over: factories and industrial complexes. Structures do not roll
	// defence dice and cannot be chosen as casualties.
	IsStructure bool

	// IsAA marks anti-aircraft artillery, which fires once at attacking
	// aircraft before a battle and is otherwise not a combat unit.
	IsAA bool

	// IsSubmarine marks a unit with submerge and first-strike behaviour.
	IsSubmarine bool

	// NegatesSubmarines marks a unit that cancels submarine special abilities.
	NegatesSubmarines bool

	// CanBombard marks a warship that may support an amphibious assault.
	CanBombard bool

	// SupportsInfantry marks artillery, which raises paired infantry attack.
	SupportsInfantry bool

	// SupportedByArtillery marks infantry, the unit whose attack artillery
	// raises when paired.
	SupportedByArtillery bool

	// CanBlitz marks a unit that may move through an empty enemy territory and
	// keep going.
	CanBlitz bool

	// MaxHits is how many hits the unit absorbs before it is destroyed.
	MaxHits int
}

// canonical reduces a unit name to a comparable form, so "industrial_complex",
// "Industrial Complex" and "industrialcomplex" are one thing.
func canonical(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r != '_' && r != '-' && r != ' ' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// capabilitiesFor derives a unit's abilities from its name and stats.
//
// Names are matched on a canonical form and cover the spellings seen in the
// wild, so both "sub" and "submarine" resolve to the same unit.
func capabilitiesFor(template *Piece) UnitCapabilities {
	caps := UnitCapabilities{MaxHits: 1}

	switch canonical(template.Name) {
	case "factory", "industrialcomplex", "ic":
		caps.IsStructure = true

	case "aaa", "aa", "antiaircraft", "antiaircraftartillery", "aagun":
		caps.IsAA = true

	case "sub", "submarine":
		caps.IsSubmarine = true

	case "destroyer":
		// Destroyers cancel submarine abilities but do not bombard: shore
		// bombardment is a battleship and cruiser capability.
		caps.NegatesSubmarines = true

	case "battleship":
		caps.CanBombard = true
		caps.MaxHits = 2

	case "cruiser":
		caps.CanBombard = true

	case "artillery":
		caps.SupportsInfantry = true

	case "infantry", "inf":
		caps.SupportedByArtillery = true

	case "armor", "tank":
		caps.CanBlitz = true
	}

	// A unit that cannot move and cannot fight is a structure whatever it is
	// called, which keeps a board that invents its own building types working.
	if template.Movement == 0 && template.Attack == 0 && template.Defend == 0 {
		caps.IsStructure = true
	}
	return caps
}

// CapabilitiesOf derives a piece's abilities from the piece itself.
//
// A piece is cloned from its template and carries the template's name and
// stats, so this gives the same answer as looking the name up in the board's
// registry -- without any shared state. Combat uses this rather than a
// package-level registry: the old global was overwritten by every controller
// and read during every battle, so two concurrent games raced it, and two
// games on different boards used each other's unit rules.
func CapabilitiesOf(piece *Piece) UnitCapabilities {
	if piece == nil {
		return UnitCapabilities{MaxHits: 1}
	}
	return capabilitiesFor(piece)
}

// UnitRegistry answers capability questions for a board's unit roster.
type UnitRegistry struct {
	byName map[string]UnitCapabilities
}

// BuildUnitRegistry derives capabilities for every declared unit type.
func BuildUnitRegistry(templates map[string]*Piece) *UnitRegistry {
	registry := &UnitRegistry{byName: make(map[string]UnitCapabilities, len(templates))}
	for name, template := range templates {
		registry.byName[name] = capabilitiesFor(template)
	}
	return registry
}

// Of returns the capabilities of a unit type. Unknown names get the neutral
// default rather than an error: a board may legitimately field a unit this
// build has never heard of, and it should behave like a plain combat unit.
func (r *UnitRegistry) Of(name string) UnitCapabilities {
	if r == nil {
		return capabilitiesFor(&Piece{Name: name})
	}
	if caps, ok := r.byName[name]; ok {
		return caps
	}
	return capabilitiesFor(&Piece{Name: name})
}

// For returns the capabilities of a specific piece.
func (r *UnitRegistry) For(piece *Piece) UnitCapabilities {
	if piece == nil {
		return UnitCapabilities{MaxHits: 1}
	}
	return r.Of(piece.Name)
}

// Units returns the registry for this game's roster, building it on first use.
func (g *Game) Units() *UnitRegistry {
	if g.unitRegistry == nil {
		g.unitRegistry = BuildUnitRegistry(g.GlobalPieceTemplates)
	}
	return g.unitRegistry
}
