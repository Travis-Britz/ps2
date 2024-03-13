package psmap

import (
	"math"
)

const mapDimensions = 8192

type Point interface {
	// Point returns an x,y coordinate on a plane where top left is 0,0.
	// This correlates to the SVG coordinate system and most computer graphics.
	Point() (float64, float64)
}

// Outline generates a list of coordinates that define a polygon shaped like a map region.
//
// hexes are a slice of Hex tiles using the planetside hex grid system for X,Y.
// width is the width of the point-up hexagon.
// The tile X,Y and the width correspond to the values available from the Census API.
// The returned value is a list of X,Y graphics coordinates for drawing a polygon shaped like the outside of the map region.
// The final point does not return to the start.
// The <polygon> SVG element is self-closing.
//
// The coordinates have 0,0 located at the upper left,
// intentionally matching the SVG coordinate system.
//
// To draw the outline for Ti Alloys:
// https://census.daybreakgames.com/get/ps2:v2/map_hex?c:limit=5000&map_region_id=2419&c:show=x,y
// https://census.daybreakgames.com/get/ps2:v2/zone?c:lang=en&zone_id=2&c:show=hex_size
func Outline(hexes []Hex, width int) (path []Point) {
	if len(hexes) == 0 {
		return nil
	}
	size := widthToSize(width)
	region := make(map[Hex]bool)
	min_x := 999
	var leftmost Hex
	// build up a set of hexes in the region to quickly check whether a hex is part of the region
	for _, hex := range hexes {
		region[hex] = true
		// keep track of the leftmost hex tile we find while iterating through the list
		if hex.X < min_x {
			min_x = hex.X
			leftmost = hex
		}
	}

	// To get the outline of a region we walk along the outside edges,
	// treating it much like a graph structure where the nodes are the "Y" intersection of three hexagon corners,
	// and the edges of the graph are the flat hexagon edges between the corners.
	// Our starting point needs to be an outer edge of the region.
	// The choice of leftmost is arbitrary; it could have been top/right/bottom (with adjustments for the first move).
	// Our hex points are indexed counter-clockwise 0-5, with 0 being the top point.
	// By starting on the leftmost hex at point 1 (top of left face) we know we're on the outside of the region.
	// From there we simply walk around the outside of the region counter-clockwise.
	start := point{
		Hex:    leftmost,
		corner: 1,
		size:   size,
	}
	// We know the first move is always left because we start at the leftmost hex corner 1.
	left, right := fork(start)
	path = []Point{start, left}
	for current := left; current != start; {
		left, right = fork(current)
		// At every new node we try to turn right first.
		// If the region exists in our set then we procede down that edge.
		// Otherwise we turn left and stay on the current hex tile.
		// This has the convenient property that the last move will always end on the start point.
		if region[right.Hex] {
			current = right
		} else {
			current = left
		}
		path = append(path, current)
	}

	return path
}

func fork(current point) (left, right point) {
	// the neighbor points depend on which point of the hexagon we're currently on
	// we explore the possible options counter-clockwise.
	// we push points from neighboring hexes before points on the current hex
	// because each point can be represented in three different ways using our method of Hex(X,Y,n).
	// imagine the Y joint between three hexes - the center point can be represented as
	// point 3 on the top hex, point 5 on the left hex, or point 1 on the right hex.
	// with traversal being depth-first, by jumping to the next region first we don't have to worry
	// about deduplicating overlapping points because the duplicates will never be explored.

	// to phrase another way:
	// every point is the intersection of three edges.
	// while walking along the outer path, at each corner there are three possible directions:
	// turn right, turn left, turn back.
	// turning back is never correct for tracing an outline.
	// so for each point we try turning right first, then turning left.
	// a left turn is a corner on the current hex tile.

	switch current.corner {
	case 0:
		right = current
		right.UpLeft()
		right.corner = 5

		left = current
		left.corner = 1
	case 1:
		right = current
		right.Left()
		right.corner = 0

		left = current
		left.corner = 2
	case 2:
		right = current
		right.DownLeft()
		right.corner = 1

		left = current
		left.corner = 3
	case 3:
		right = current
		right.DownRight()
		right.corner = 2

		left = current
		left.corner = 4
	case 4:
		right = current
		right.Right()
		right.corner = 3

		left = current
		left.corner = 5
	case 5:
		right = current
		right.UpRight()
		right.corner = 4

		left = current
		left.corner = 0
	}
	return left, right
}

// widthToSize converts a size given by census (width/diameter describing the hex inner circle)
// to a point-up size describing the radius of the outer circle.
// https://www.redblobgames.com/grids/hexagons/#basics
func widthToSize(width int) float64 {
	// census lists the hex size as the width,
	// or diameter of the inner circle describing a point-up hex.

	// a hexagon can be split into a number of right triangles.
	// we're given the adjacent side and angle alpha.
	// solve for the hypotenuse to get the distance from the center to the top point of a hex.
	b := float64(width) / 2
	a := b * math.Tan((30*math.Pi)/180)
	c := math.Sqrt(a*a + b*b)
	return c
}

// point is a cartesian coordinate point represented by a hex grid tile and corner offset.
// three different hex corners may share the same point.
type point struct {
	Hex
	corner int
	size   float64
}

// Point implements [Point].
func (p point) Point() (x float64, y float64) {

	// dimensions of a hex
	width := math.Sqrt(3) * p.size
	height := 2 * p.size

	// center point of a hex
	center_x := width * (float64(p.Hex.X) + float64(p.Hex.Y)*0.5)
	center_y := float64(-1*p.Hex.Y)*height*0.75 - height/2

	// our corner indexing starts from the top corner of a hex and goes counter-clockwise.
	// the math might seem weird if you try to verify or recreate it because we're switching between different coordinate systems.
	// honestly I just flipped signs until the tiles lined up with a map image.
	angle_deg := 60*p.corner + 90
	angle_rad := (math.Pi / 180) * float64(angle_deg)

	// x,y coord of the hex corner
	x = center_x + p.size*math.Cos(angle_rad)
	y = center_y - p.size*math.Sin(angle_rad)

	// adjust the coordinate for the planetside map, which has 0,0 in the center of the map
	x = x + mapDimensions/2
	y = y + mapDimensions/2

	return x, y
}

// Hex represents the position of a single map hex tile.
// X,Y correspond to the game's tile grid as returned by the census /map_hex endpoint..
type Hex struct {
	X, Y int
}

// Left moves the tile left.
func (h *Hex) Left() {
	h.X--
}

// Right moves the tile right.
func (h *Hex) Right() {
	h.X++
}

// UpLeft moves the tile diagonally up and left.
func (h *Hex) UpLeft() {
	h.X--
	h.Y++
}

// DownLeft moves the tile diagonally down and to the left.
func (h *Hex) DownLeft() {
	h.Y--
}

// UpRight moves the tile diagonally up and to the right.
func (h *Hex) UpRight() {
	h.Y++
}

// DownRight moves the tile diagonally down adn to the right.
func (h *Hex) DownRight() {
	h.X++
	h.Y--
}
