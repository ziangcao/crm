package entity

import "time"

type DashboardDefinition struct {
	ID, OrganizationID, Name string
	Widgets                  []DashboardWidget
	IsDefault                bool
	CreatedAt, UpdatedAt     time.Time
}

type DashboardWidget struct {
	Key   string `json:"key"`
	Width int    `json:"width"`
}
