package psmap_test

import (
	"image"
	"testing"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/psmap"
)

func TestBounds(t *testing.T) {

	tt := map[string]struct {
		ExpectedBounds image.Rectangle
		ImageBounds    image.Rectangle
		Region         ps2.RegionID
		Data           psmap.Map
		ExpectedError  bool
	}{
		"Region with single tile at 0,0 with image:map scale of 1:1 and bounds matching the game coordinates": {
			// "HexSize" is the distance from the center point of a hex tile to the top corner.
			// hexagon height: 2 * size
			// hexagon width: sqrt(3) * size
			// (sqrt(3)*200) / 2 = 173
			ExpectedBounds: image.Rect(-173, -200, 173, 200),
			ImageBounds:    image.Rect(-4096, -4096, 4096, 4096),
			Region:         1,
			ExpectedError:  false,
			Data: psmap.Map{
				HexSize: 200,
				Size:    8192,
				Regions: []psmap.Region{
					{
						RegionID: 1,
						Hexes: []psmap.Hex{
							{X: 0, Y: 0},
						},
					},
				},
			},
		},
		"Region with single tile at 0,0 with image:map scale of 1:1 and bounds adjusted to 0,0 for Min": {
			// "HexSize" is the distance from the center point of a hex tile to the top corner.
			// hexagon height: 2 * size
			// hexagon width: sqrt(3) * size
			// (sqrt(3)*200) / 2 = 173
			ExpectedBounds: image.Rect(3923, 3896, 4296, 4296),
			ImageBounds:    image.Rect(0, 0, 8192, 8192),
			Region:         1,
			ExpectedError:  false,
			Data: psmap.Map{
				HexSize: 200,
				Size:    8192,
				Regions: []psmap.Region{
					{
						RegionID: 1,
						Hexes: []psmap.Hex{
							{X: 0, Y: 0},
						},
					},
				},
			},
		},
	}

	for testname, testcase := range tt {
		result, err := psmap.Bounds(testcase.ImageBounds, testcase.Data, getHexes(testcase.Data, testcase.Region))
		if err != nil && !testcase.ExpectedError {
			t.Errorf("%s: expected nil error; got %s", testname, err)
		}
		if err == nil && testcase.ExpectedError {
			t.Errorf("%s: expected case to return an error", testname)
		}

		if !result.Eq(testcase.ExpectedBounds) {
			t.Errorf("%s: expected bounds to be %s; got %s", testname, testcase.ExpectedBounds, result)
		}

	}

	// give a 1:1 scale bounds so the math is easier to test the base case
}

func getHexes(data psmap.Map, region ps2.RegionID) []psmap.Hex {
	for _, r := range data.Regions {
		if r.RegionID == region {
			return r.Hexes
		}
	}
	return nil
}
