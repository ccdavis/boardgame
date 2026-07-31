package layout

import "math"

type box struct{ minX, minY, maxX, maxY float64 }

func (b box) inflate(d float64) box {
	return box{b.minX - d, b.minY - d, b.maxX + d, b.maxY + d}
}

func (b box) overlaps(o box) bool {
	return !(b.maxX < o.minX || o.maxX < b.minX || b.maxY < o.minY || o.maxY < b.minY)
}

func ringBox(r Ring) box {
	out := box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, p := range r {
		out.minX = math.Min(out.minX, p[0])
		out.minY = math.Min(out.minY, p[1])
		out.maxX = math.Max(out.maxX, p[0])
		out.maxY = math.Max(out.maxY, p[1])
	}
	return out
}

func (t *Territory) bounds() box {
	out := box{math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)}
	for _, poly := range t.Polygons {
		if len(poly) == 0 {
			continue
		}
		b := ringBox(poly[0])
		out.minX = math.Min(out.minX, b.minX)
		out.minY = math.Min(out.minY, b.minY)
		out.maxX = math.Max(out.maxX, b.maxX)
		out.maxY = math.Max(out.maxY, b.maxY)
	}
	return out
}

// signedArea is twice the signed area of a ring; sign gives winding order.
func signedArea(r Ring) float64 {
	var sum float64
	for i := range r {
		j := (i + 1) % len(r)
		sum += r[i][0]*r[j][1] - r[j][0]*r[i][1]
	}
	return sum / 2
}

func ringArea(r Ring) float64 { return math.Abs(signedArea(r)) }

// Area is the total area of a territory, holes subtracted.
func (t *Territory) Area() float64 {
	var total float64
	for _, poly := range t.Polygons {
		for i, ring := range poly {
			if i == 0 {
				total += ringArea(ring)
			} else {
				total -= ringArea(ring)
			}
		}
	}
	return total
}

// pointInRing is a standard ray-crossing test.
func pointInRing(x, y float64, r Ring) bool {
	inside := false
	for i := range r {
		j := (i + 1) % len(r)
		xi, yi := r[i][0], r[i][1]
		xj, yj := r[j][0], r[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
	}
	return inside
}

// Contains reports whether a point lies inside the territory, respecting holes.
func (t *Territory) Contains(x, y float64) bool {
	for _, poly := range t.Polygons {
		if len(poly) == 0 || !pointInRing(x, y, poly[0]) {
			continue
		}
		inHole := false
		for _, hole := range poly[1:] {
			if pointInRing(x, y, hole) {
				inHole = true
				break
			}
		}
		if !inHole {
			return true
		}
	}
	return false
}

// pointSegmentDistance is the distance from p to segment ab.
func pointSegmentDistance(px, py, ax, ay, bx, by float64) float64 {
	dx, dy := bx-ax, by-ay
	if dx == 0 && dy == 0 {
		return math.Hypot(px-ax, py-ay)
	}
	t := ((px-ax)*dx + (py-ay)*dy) / (dx*dx + dy*dy)
	t = math.Max(0, math.Min(1, t))
	return math.Hypot(px-(ax+t*dx), py-(ay+t*dy))
}

// distanceToBoundary is the shortest distance from a point to any edge.
func (t *Territory) distanceToBoundary(x, y float64) float64 {
	best := math.Inf(1)
	for _, poly := range t.Polygons {
		for _, ring := range poly {
			for i := range ring {
				j := (i + 1) % len(ring)
				d := pointSegmentDistance(x, y, ring[i][0], ring[i][1], ring[j][0], ring[j][1])
				if d < best {
					best = d
				}
			}
		}
	}
	return best
}

// SharedBorder estimates the length of boundary that a and b hold in common.
//
// Each edge of a is walked at a fixed step and a sample counts when it lies
// within eps of b's boundary. Multiplying surviving samples by the step gives a
// length in layout units. That is enough to separate a real shared border from
// two regions meeting at a corner, which is all the adjacency check needs.
func SharedBorder(a, b *Territory, step, eps float64) float64 {
	if !a.bounds().inflate(eps).overlaps(b.bounds().inflate(eps)) {
		return 0
	}

	var length float64
	for _, poly := range a.Polygons {
		for _, ring := range poly {
			for i := range ring {
				j := (i + 1) % len(ring)
				x0, y0 := ring[i][0], ring[i][1]
				x1, y1 := ring[j][0], ring[j][1]
				segLen := math.Hypot(x1-x0, y1-y0)
				if segLen == 0 {
					continue
				}
				steps := int(segLen/step) + 1
				for s := 0; s <= steps; s++ {
					f := float64(s) / float64(steps)
					px, py := x0+f*(x1-x0), y0+f*(y1-y0)
					if b.distanceToBoundary(px, py) <= eps {
						length += segLen / float64(steps)
					}
				}
			}
		}
	}
	return length
}
