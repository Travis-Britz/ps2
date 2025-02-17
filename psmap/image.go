package psmap

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/Travis-Britz/ps2"
	"github.com/llgcode/draw2d/draw2dimg"
)

// FactionDrawColors are the base colors used for filling regions by owning faction in map drawing functions.
// These colors will be darkened for cut off territory and be shifted for opacity.
var FactionDrawColors = [5]color.RGBA{
	{0x00, 0x00, 0x00, 0x00}, // ps2.None
	{0x44, 0x0e, 0x62, 0xff}, // ps2.VS
	{0x00, 0x4b, 0x80, 0xff}, // ps2.NC
	{0x9e, 0x0b, 0x0f, 0xff}, // ps2.TR
	{0x80, 0x80, 0x80, 0xff}, // ps2.NSO - I think the Forgotten Fleet Carrier event can capture territories as NSO?
}

// Draw will draw map regions onto img.
// Note that the full resolution planetside map is 8192x8192 for the main continents,
// which would use 350MB of memory at minimum.
// Draw will scale to the dimensions of img.
// Pass a smaller img if memory or processing time is a concern.
// Note that the coordinate system is shifted from that returned by Census.
// Census consideres 0,0 to be the center of the map.
// This function expects 0,0 to be the upper left corner of img and will shift census coordinates appropriately.
// Map data should be given using the Census coordinates.
func Draw(img draw.Image, data Map, mapstate owner) error {
	if img.Bounds().Dx() != img.Bounds().Dy() {
		return fmt.Errorf("psmap.Draw: image bounds must be square; given: %v", img.Bounds())
	}
	if img.Bounds().Empty() {
		return errors.New("psmap.Draw: image cannot be empty")
	}
	if (img.Bounds().Min != image.Point{}) {
		// The draw2dimg package behaves in unexpected ways when img does not start at 0,0.
		// Rather than confound users of this function,
		// return an error instead.
		return errors.New("psmap.Draw: image bounds must start at 0,0")
	}

	summary, err := Summarize(data, mapstate)
	if err != nil {
		return fmt.Errorf("psmap.Draw: summary failed: %w", err)
	}

	// scale is the ratio of the full continent size to the destination image size
	scale := float64(img.Bounds().Dx()) / float64(data.Size)

	// transform takes a Point p and adjusts the points from the census coordinate system to our image's coordinates with 0,0 on the top left.
	// the resulting point is then scaled from the original continent dimensions to the image dimensions.
	transform := func(p Point) (x float64, y float64) {
		x, y = p.Point()

		// first shift the point so that 0,0 is the upper left of the coordinate system rather than the center
		// by adding half the continent size
		x += float64(data.Size / 2)
		y += float64(data.Size / 2)

		// then multiply the coordinate by the scale factor of the image bounds relative to what the full continent size is
		x *= scale
		y *= scale
		return x, y
	}

	gc := draw2dimg.NewGraphicContext(img)
	// gc.Translate(float64(img.Bounds().Min.X), float64(img.Bounds().Min.Y))
	for _, region := range data.Regions {

		// Set some properties
		gc.SetStrokeColor(color.White)
		gc.SetLineWidth(4 * scale)

		faction := mapstate.Owner(region.RegionID)
		fc := FactionDrawColors[faction]
		if summary.Cutoff[region.RegionID] {
			// darken cut off regions
			fc.R /= 2
			fc.G /= 2
			fc.B /= 2
		}
		if fc.A != 0 { // prevent divide by zero
			// set opacity

			newA := uint8(255 * 0.4) // 40% opacity
			fc.R = uint8(uint16(fc.R) * uint16(newA) / uint16(fc.A))
			fc.G = uint8(uint16(fc.G) * uint16(newA) / uint16(fc.A))
			fc.B = uint8(uint16(fc.B) * uint16(newA) / uint16(fc.A))
			fc.A = newA
		}

		gc.SetFillColor(fc)

		// Draw a closed shape
		gc.BeginPath() // Initialize a new path
		outline := Outline(region.Hexes, data.HexSize)
		for i, point := range outline {
			if i == 0 {
				gc.MoveTo(transform(point)) // Move to a position to start the new path
			} else {
				gc.LineTo(transform(point))
			}
		}
		gc.Close()
		gc.FillStroke()
	}
	return nil
}

func DrawMask(mask draw.Image, data Map, hexes []Hex, scale float64, offset image.Point) error {
	// var minX, minY, maxX, maxY float64 = 9000, 9000, -9000, -9000

	// transform takes a Point p and adjusts the points from the census coordinate system to our image's coordinates with 0,0 on the top left.
	// the resulting point is then scaled from the original continent dimensions to the image dimensions.
	transform := func(x, y float64) (float64, float64) {
		// first shift the point so that 0,0 is the upper left of the coordinate system rather than the center
		// by adding half the continent size
		x += float64(data.Size / 2)
		y += float64(data.Size / 2)

		// then multiply the coordinate by the scale factor of the image bounds relative to what the full continent size is
		x *= scale
		y *= scale
		return x, y
	}

	gc := draw2dimg.NewGraphicContext(mask)

	// Set some properties
	gc.SetStrokeColor(color.Transparent)
	gc.SetLineWidth(2 * scale)
	gc.SetFillColor(color.Opaque)

	// Draw a closed shape
	gc.BeginPath() // Initialize a new path
	outline := Outline(hexes, data.HexSize)
	for i, point := range outline {
		x, y := point.Point()
		x, y = transform(x, y)
		// slog.Info("point", "x", x, "y", y)

		// adjust the outline to be relative to the crop offset
		x -= float64(offset.X)
		y -= float64(offset.Y)

		if i == 0 {
			gc.MoveTo(x, y) // Move to a position to start the new path
		} else {
			gc.LineTo(x, y)
		}
	}
	gc.Close()
	gc.FillStroke()

	return nil
}

// Bounds returns a rectangle describing the bounds of region inside terrain bounds.
func Bounds(terrainBounds image.Rectangle, data Map, hexes []Hex) (image.Rectangle, error) {

	if terrainBounds.Empty() {
		return image.Rectangle{}, errors.New("cannot use empty terrain bounds")
	}

	if terrainBounds.Dx() != terrainBounds.Dy() {
		return image.Rectangle{}, errors.New("terrain bounds must be square")
	}

	if len(hexes) == 0 {
		return image.Rectangle{}, errors.New("no hexes given")
	}

	var minX, minY, maxX, maxY float64 = 9000, 9000, -9000, -9000
	outline := Outline(hexes, data.HexSize)
	for _, p := range outline {
		x, y := p.Point()
		if x < minX {
			minX = x
		}
		if y < minY {
			minY = y
		}
		if x > maxX {
			maxX = x
		}
		if y > maxY {
			maxY = y
		}
	}

	// scale is the ratio of the full continent size to the supplied image size
	scale := float64(terrainBounds.Bounds().Dx()) / float64(data.Size)

	// transform takes a Point p and adjusts the points from the census coordinate system to our image's coordinates with 0,0 on the top left.
	// the resulting point is then scaled from the original continent dimensions to the image dimensions.
	transform := func(x, y float64) (float64, float64) {
		// first shift the point so that 0,0 is the upper left of the coordinate system rather than the center
		// by adding half the continent size
		// todo: this should account for the terrain image's actual coordinates
		x += float64(data.Size/2 + terrainBounds.Min.X)
		y += float64(data.Size/2 + terrainBounds.Min.Y)

		// then multiply the coordinate by the scale factor of the image bounds relative to what the full continent size is
		x *= scale
		y *= scale
		return x, y
	}

	minX, minY = transform(minX, minY)
	maxX, maxY = transform(maxX, maxY)

	rect := image.Rect(int(minX)-1, int(minY)-1, int(maxX)+1, int(maxY)+1)
	return rect, nil
}

func LocBounds(terrainBounds image.Rectangle, data Map, loc Loc) (image.Rectangle, error) {
	if terrainBounds.Empty() {
		return image.Rectangle{}, errors.New("cannot use empty terrain bounds")
	}

	if terrainBounds.Dx() != terrainBounds.Dy() {
		return image.Rectangle{}, errors.New("terrain bounds must be square")
	}
	scale := float64(terrainBounds.Bounds().Dx()) / float64(data.Size)
	// transform takes a Point p and adjusts the points from the census coordinate system to our image's coordinates with 0,0 on the top left.
	// the resulting point is then scaled from the original continent dimensions to the image dimensions.
	transform := func(x, y float64) (float64, float64) {
		// first shift the point so that 0,0 is the upper left of the coordinate system rather than the center
		// by adding half the continent size
		// todo: this should account for the terrain image's actual coordinates
		x += float64(data.Size/2 + terrainBounds.Min.X)
		y += float64(data.Size/2 + terrainBounds.Min.Y)

		// then multiply the coordinate by the scale factor of the image bounds relative to what the full continent size is
		x *= scale
		y *= scale
		return x, y
	}
	size := float64(2 * data.HexSize)
	x, y := loc.Point()
	minX, minY := transform(x-size, y-size)
	maxX, maxY := transform(x+size, y+size)

	rect := image.Rect(int(minX), int(minY), int(maxX), int(maxY))
	return rect, nil
}

// GenerateMask return an [image.Image] for use as a mask in [draw.DrawMask].
// mask draw.Image, data Map, hexes []Hex, scale float64, offset image.Point

// scale: map image size divided by full resolution continent size (e.g. 512/8192)
func GenerateMask(bounds image.Rectangle, data Map, hexes []Hex, scale float64, offset image.Point, fill color.Color, outline color.Color) (image.Image, error) {
	// need scale and offset?

	// var minX, minY, maxX, maxY float64 = 9000, 9000, -9000, -9000

	// transform takes a Point p and adjusts the points from the census coordinate system to our image's coordinates with 0,0 on the top left.
	// the resulting point is then scaled from the original continent dimensions to the image dimensions.
	transform := func(x, y float64) (float64, float64) {
		// first shift the point so that 0,0 is the upper left of the coordinate system rather than the center
		// by adding half the continent size
		x += float64(data.Size / 2)
		y += float64(data.Size / 2)

		// then multiply the coordinate by the scale factor of the image bounds relative to what the full continent size is
		x *= scale
		y *= scale
		return x, y
	}

	mask := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	gc := draw2dimg.NewGraphicContext(mask)

	// Set some properties
	gc.SetStrokeColor(outline)
	gc.SetLineWidth(4 * scale)
	gc.SetFillColor(fill)

	// Draw a closed shape
	gc.BeginPath() // Initialize a new path
	hexOutline := Outline(hexes, data.HexSize)
	for i, point := range hexOutline {
		x, y := point.Point()
		x, y = transform(x, y)
		// slog.Info("point", "x", x, "y", y)

		// adjust the outline to be relative to the crop offset
		x -= float64(offset.X)
		y -= float64(offset.Y)

		if i == 0 {
			gc.MoveTo(x, y) // Move to a position to start the new path
		} else {
			gc.LineTo(x, y)
		}
	}
	gc.Close()
	gc.FillStroke()

	return mask, nil
}

// func CropRegion(terrain image.Image, data Map, region ps2.RegionID) (image.Image, image.Point, error) {
// 	scale := float64(terrain.Bounds().Dx()) / float64(data.Size)

// 	regionBounds, err := Bounds(terrain.Bounds(), data, region)
// 	if err != nil {
// 		return nil, image.Point{}, err
// 	}

// 	// slog.Info("demensions", "rect", rect)
// 	regionImage := image.NewRGBA(regionBounds)
// 	draw.Draw(regionImage, regionImage.Bounds(), terrain, regionBounds.Min, draw.Src)

// 	gc := draw2dimg.NewGraphicContext(regionImage)

// 	// Set some properties
// 	gc.SetStrokeColor(color.White)
// 	gc.SetLineWidth(5 * scale)
// 	gc.SetFillColor(color.Transparent)

// 	// Draw a closed shape
// 	gc.BeginPath() // Initialize a new path
// 	for i, point := range outline {
// 		x, y := point.Point()
// 		x, y = transform(x, y)
// 		// slog.Info("point", "x", x, "y", y)

// 		// adjust the outline to be relative to the crop offset
// 		x -= minX
// 		y -= minY

// 		if i == 0 {
// 			gc.MoveTo(x, y) // Move to a position to start the new path
// 		} else {
// 			gc.LineTo(x, y)
// 		}
// 	}
// 	gc.Close()
// 	gc.FillStroke()

// 	return regionImage, image.Point{X: int(minX), Y: int(minY)}, nil
// }

// renderOptions are potential options that could be passed for map rendering.
// it might be cleaner (and much easier to expand on later) to have separate drawing functions instead,
// e.g. DrawLattice(img image.Image,...), DrawFacilityNames(img.Image,...).
// type renderOptions struct {
// 	drawTerrain bool
// 	drawLattice bool
// 	drawFacilityName bool
// 	drawFacilityNameColor bool
// 	drawFacilityIcon bool
// 	drawRegionOutline bool
// 	drawContinentOutline bool // outline the entire continent
// }

// Size returns the full size map dimensions of the given continent,
// or an error if the size is unknown.
func Size(continent ps2.ContinentID) (int, error) {
	switch continent {
	case ps2.Indar, ps2.Hossin, ps2.Amerish, ps2.Esamir, ps2.Oshur, ps2.Desolation:
		return 8192, nil
	case ps2.Nexus, ps2.Koltyr:
		return 4096, nil
	case ps2.Tutorial:
		return 2048, nil
	default:
		return 0, errors.New("no data")
	}
}
