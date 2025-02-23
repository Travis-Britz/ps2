package psmap

import (
	"errors"
	"io"
	"text/template"

	"github.com/Travis-Britz/ps2"
	"github.com/Travis-Britz/ps2/census"
)

const svgTemplate = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 8192 8192">
<style>
	.NC {fill: #004b80ff;}
	.TR {fill: #9e0b0fff;}
	.VS {fill: #440e62ff;}
	.None {fill: #00000009;}
	.cutoff {filter: brightness(0.6);}
	polygon:hover {
		filter: brightness(1.5);
	}
	polygon {
		transition: 0.4s;
		stroke-width: 3px;
		stroke: white;
	}
</style>
{{if (ne .TerrainImageURL "")}}
	<image x="0" y="0" width="8192" height="8192" href="{{ .TerrainImageURL }}"/>
{{end}}
{{range .Regions }}
	<g id="region{{ .ID }}" class="">
		<polygon points="{{range .Outline}}{{.X}},{{.Y}} {{end}}" class="{{.Faction}}{{if .Cutoff}} cutoff{{end}}" style="fill:lime;stroke:purple;stroke-width:3"/>
	</g>
{{end}}
</svg>`

var svgTmpl = template.Must(template.New("mapsvg").Parse(svgTemplate))

func Svg(zone ps2.ZoneID, state State) io.WriterTo {
	svg := &svgZone{}

	return svg
}

func (svg svgZone) WriteTo(w io.Writer) (int64, error) {
	return 0, errors.New("not implemented")
	// counter := &counter{w: w}
	// err := svgTmpl.Execute(counter, svg)
	// return int64(counter.n), err
}

type counter struct {
	n int
	w io.Writer
}

func (c *counter) Write(p []byte) (n int, err error) {
	n, err = c.w.Write(p)
	c.n += n
	return
}

type svgMapregion struct {
	ID           ps2.RegionID
	Name         string
	FacilityID   ps2.FacilityID
	FacilityType ps2.FacilityTypeID
	FacilityX    float64
	FacilityY    float64
	Faction      ps2.FactionID
	Cutoff       bool
	Outline      []struct {
		X int64
		Y int64
	}
}
type svgZone struct {
	Regions         []svgMapregion
	Links           []Link
	TerrainImageURL string
}

// https://census.daybreakgames.com/get/ps2:v2/zone?zone_id=2&c:join=map_region^list:1^inject_at:regions^hide:zone_id(map_hex^list:1^inject_at:hexes^hide:zone_id'map_region_id)&c:lang=en

type nounmarshalzone census.Zone
type MapResult struct {
	nounmarshalzone
	MapRegions    []RegionResult        `json:"regions"`
	FacilityLinks []census.FacilityLink `json:"links"`
}

type RegionResult struct {
	census.MapRegion
	Hexes []census.MapHex `json:"hexes"`
}

type censusMapResult struct {
	ZoneList []MapResult `json:"zone_list"`
	Returned int         `json:"returned"`
}
