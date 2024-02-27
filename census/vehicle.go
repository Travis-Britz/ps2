package census

import "github.com/Travis-Britz/ps2"

type Vehicle struct {
	VehicleID      ps2.VehicleID    `json:"vehicle_id,string"`
	Name           ps2.Localization `json:"name"`
	Description    ps2.Localization `json:"description"`
	Type           int              `json:"type_id,string"` // Type is things like "four wheeled ground vehicle", and generally not useful.
	TypeName       string           `json:"type_name"`
	Cost           int              `json:"cost,string"`
	CostResourceID ps2.ResourceID   `json:"cost_resource_id,string"`
	ImageSetID     ps2.ImageSetID   `json:"image_set_id,string"`
	ImageID        ps2.ImageID      `json:"image_id,string"`
	ImagePath      string           `json:"image_path"`
}

func (v Vehicle) ImageURL() string { return apiBase + v.ImagePath }

func (Vehicle) CollectionName() string { return "vehicle" }
