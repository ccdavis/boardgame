package layout

import (
	"fmt"
	"sort"

	"boardgame/models"
)

// Thresholds in layout units, against a viewBox 1000 wide.
//
// The gap between Lmin and Lmax is deliberate hysteresis: contact below Lmin is
// not enough to satisfy a declared adjacency, contact above Lmax must be
// declared, and anything between is neither required nor forbidden. Without it
// every corner where four regions meet becomes an argument.
//
// Lmin has to clear the corner-contact ceiling. Measuring shared border with a
// tolerance of contactEps necessarily overcounts near a shared corner, because
// short stretches of the *perpendicular* edges also fall within tolerance: two
// regions meeting at nothing but a corner measure about 1.9. Anything at or
// below that would turn every four-way junction into two invented adjacencies.
// Real shared borders on this map run from roughly 10 to 100 units, so 4.0
// leaves a wide margin on both sides.
const (
	Lmin        = 4.0
	Lmax        = 8.0
	sampleStep  = 0.5
	contactEps  = 0.75
	minRingArea = 0.25
)

// ProblemKind classifies a layout defect.
type ProblemKind string

const (
	MissingTerritory ProblemKind = "MISSING_TERRITORY" // in the .gdf, absent from the layout
	UnknownTerritory ProblemKind = "UNKNOWN_TERRITORY" // in the layout, absent from the .gdf
	KindMismatch     ProblemKind = "KIND_MISMATCH"     // land/sea disagrees with the .gdf
	BadGeometry      ProblemKind = "BAD_GEOMETRY"      // degenerate ring or zero area
	AnchorOutside    ProblemKind = "ANCHOR_OUTSIDE"    // label/marker not inside its region
	MissingContact   ProblemKind = "MISSING_CONTACT"   // adjacency declared, borders do not meet
	UndeclaredTouch  ProblemKind = "UNDECLARED_TOUCH"  // borders meet, no adjacency declared
	Overlap          ProblemKind = "OVERLAP"           // two land regions cover the same point
)

// Problem is one defect, reported against a territory or a pair.
type Problem struct {
	Kind   ProblemKind
	A, B   string
	Detail string
}

// IsError reports whether a problem means the map is broken, as opposed to
// merely disagreeing with the board's abstraction.
//
// UndeclaredTouch is the one warning. It fires where two regions genuinely
// share a border on the drawn map but the .gdf does not connect them -- Alaska
// and Canadian Pacific, for instance. The geometry is not wrong there; the .gdf
// graph is simply coarser than geography, and reconciling the two changes how
// the game plays. That is the board author's call, not the renderer's.
func (p Problem) IsError() bool { return p.Kind != UndeclaredTouch }

func (p Problem) String() string {
	if p.B != "" {
		return fmt.Sprintf("%-17s %q -- %q  %s", p.Kind, p.A, p.B, p.Detail)
	}
	return fmt.Sprintf("%-17s %q  %s", p.Kind, p.A, p.Detail)
}

// Options selects which checks run. Adjacency is the expensive one.
type Options struct {
	SkipAdjacency bool
}

// Validate checks the layout against the board it claims to describe.
func (l *Layout) Validate(game *models.Game, opts Options) []Problem {
	var problems []Problem

	problems = append(problems, l.checkKeys(game)...)
	problems = append(problems, l.checkGeometry(game)...)
	if !opts.SkipAdjacency {
		problems = append(problems, l.checkAdjacency(game)...)
	}
	return problems
}

// ValidateKeys is the cheap gate a server runs when a game is created: it
// answers "is this layout even for this board?" without touching geometry.
func (l *Layout) ValidateKeys(game *models.Game) error {
	problems := l.checkKeys(game)
	if len(problems) == 0 {
		return nil
	}

	var missing, unknown []string
	for _, p := range problems {
		switch p.Kind {
		case MissingTerritory:
			missing = append(missing, p.A)
		case UnknownTerritory:
			unknown = append(unknown, p.A)
		}
	}
	return fmt.Errorf(
		"layout does not match this board: %d territories have no geometry (%s), "+
			"%d have geometry but are not on the board (%s)",
		len(missing), join(missing), len(unknown), join(unknown))
}

func join(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	if len(names) > 6 {
		return fmt.Sprintf("%v and %d more", names[:6], len(names)-6)
	}
	return fmt.Sprintf("%v", names)
}

func (l *Layout) checkKeys(game *models.Game) []Problem {
	var problems []Problem

	for _, name := range sortedBoard(game) {
		if _, ok := l.Territories[name]; !ok {
			problems = append(problems, Problem{
				Kind: MissingTerritory, A: name,
				Detail: "declared in the .gdf but has no geometry",
			})
		}
	}
	for _, name := range sortedLayout(l) {
		terr, ok := game.Board[name]
		if !ok {
			problems = append(problems, Problem{
				Kind: UnknownTerritory, A: name,
				Detail: "has geometry but is not a territory on this board",
			})
			continue
		}
		want := "land"
		if terr.Terrain == models.Water {
			want = "sea"
		}
		if l.Territories[name].Kind != want {
			problems = append(problems, Problem{
				Kind: KindMismatch, A: name,
				Detail: fmt.Sprintf("layout says %q, board says %q",
					l.Territories[name].Kind, want),
			})
		}
	}
	return problems
}

func (l *Layout) checkGeometry(game *models.Game) []Problem {
	var problems []Problem

	for _, name := range sortedLayout(l) {
		t := l.Territories[name]

		if len(t.Polygons) == 0 {
			problems = append(problems, Problem{
				Kind: BadGeometry, A: name, Detail: "no polygons",
			})
			continue
		}
		for i, poly := range t.Polygons {
			if len(poly) == 0 || len(poly[0]) < 3 {
				problems = append(problems, Problem{
					Kind: BadGeometry, A: name,
					Detail: fmt.Sprintf("polygon %d has fewer than 3 points", i),
				})
				continue
			}
			if a := ringArea(poly[0]); a < minRingArea {
				problems = append(problems, Problem{
					Kind: BadGeometry, A: name,
					Detail: fmt.Sprintf("polygon %d has area %.3f, below %.2f", i, a, minRingArea),
				})
			}
		}

		// Anchors must be inside. The Playwright click test drives clicks at the
		// label anchor, so an anchor outside its region silently invalidates it.
		if !t.Contains(t.Label.X, t.Label.Y) {
			problems = append(problems, Problem{
				Kind: AnchorOutside, A: name,
				Detail: fmt.Sprintf("label anchor (%.1f, %.1f) is not inside the region",
					t.Label.X, t.Label.Y),
			})
		}
		if !t.Contains(t.Marker.X, t.Marker.Y) {
			problems = append(problems, Problem{
				Kind: AnchorOutside, A: name,
				Detail: fmt.Sprintf("marker anchor (%.1f, %.1f) is not inside the region",
					t.Marker.X, t.Marker.Y),
			})
		}
	}
	return problems
}

// checkAdjacency compares shared borders against the .gdf graph.
//
// Islands are exempt from the "declared adjacency must touch" rule: a land
// territory whose only neighbours are sea sits *inside* a sea polygon and shares
// no boundary with it. Fifteen territories on this board are like that, so
// without the exemption the check reports fifteen false failures and gets
// switched off, which is worse than not having it.
func (l *Layout) checkAdjacency(game *models.Game) []Problem {
	var problems []Problem

	names := sortedLayout(l)
	present := make(map[string]bool, len(names))
	for _, n := range names {
		if _, ok := game.Board[n]; ok {
			present[n] = true
		}
	}

	declared := make(map[[2]string]bool)
	for name, terr := range game.Board {
		for _, nb := range terr.ConnectedTo {
			declared[key(name, nb.Name)] = true
		}
	}

	for i, a := range names {
		if !present[a] {
			continue
		}
		for _, b := range names[i+1:] {
			if !present[b] {
				continue
			}
			ta, tb := l.Territories[a], l.Territories[b]
			if !ta.bounds().inflate(Lmax).overlaps(tb.bounds().inflate(Lmax)) {
				if declared[key(a, b)] && !l.Linked(a, b) && !isIsland(game, a, b) {
					problems = append(problems, Problem{
						Kind: MissingContact, A: a, B: b,
						Detail: "adjacent in the .gdf but the regions are far apart",
					})
				}
				continue
			}

			shared := SharedBorder(ta, tb, sampleStep, contactEps)
			switch {
			case declared[key(a, b)]:
				if shared < Lmin && !l.Linked(a, b) && !isIsland(game, a, b) {
					problems = append(problems, Problem{
						Kind: MissingContact, A: a, B: b,
						Detail: fmt.Sprintf("adjacent in the .gdf, shared border only %.2f (need %.2f)",
							shared, Lmin),
					})
				}
			default:
				// An island sits inside one sea zone but unavoidably abuts the
				// zones around it, so contact there says nothing about whether
				// the board should connect them. Reporting it buries the real
				// findings under noise -- Madagascar alone would raise three.
				if shared >= Lmax && !isIsland(game, a, b) {
					problems = append(problems, Problem{
						Kind: UndeclaredTouch, A: a, B: b,
						Detail: fmt.Sprintf("regions share %.2f of border but are not adjacent in the .gdf",
							shared),
					})
				}
			}
		}
	}
	return problems
}

// isIsland reports whether either territory is a land region surrounded by sea,
// in which case containment rather than a shared border expresses adjacency.
func isIsland(game *models.Game, a, b string) bool {
	return islandLike(game, a) || islandLike(game, b)
}

func islandLike(game *models.Game, name string) bool {
	terr, ok := game.Board[name]
	if !ok || terr.Terrain != models.Land {
		return false
	}
	for _, nb := range terr.ConnectedTo {
		if nb.Terrain != models.Water {
			return false
		}
	}
	return true
}

func key(a, b string) [2]string {
	if a < b {
		return [2]string{a, b}
	}
	return [2]string{b, a}
}

func sortedBoard(game *models.Game) []string {
	out := make([]string, 0, len(game.Board))
	for n := range game.Board {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func sortedLayout(l *Layout) []string {
	out := make([]string, 0, len(l.Territories))
	for n := range l.Territories {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}
