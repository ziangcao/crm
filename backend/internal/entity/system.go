package entity

import "time"

type Organization struct {
	ID, Name  string
	CreatedAt time.Time
}

type Department struct {
	ID, OrganizationID, ParentID, Name string
	Sort                               int
}

type Permission struct {
	ID, Code, Name, Module string
}

type SystemRole struct {
	ID, OrganizationID, Name string
	BuiltIn                  bool
	PermissionIDs            []string
	DataScope                string
	DepartmentIDs            []string
	FieldRestrictions        map[string]FieldRestriction
}

type FieldRestriction struct {
	Hidden   []string `json:"hidden"`
	ReadOnly []string `json:"read_only"`
}

type ModuleDefinition struct {
	ID, Key, Name, Icon, DocumentTemplate string
	Group, Description                    string
	Enabled                               bool
	Shared, Approvable, Custom            bool
	Fields                                []FieldDefinition
	Layout                                []FieldLayout
	SalesStages                           []string
	Stages                                []StageDefinition
	AfootRollback, EndRollback            bool
	CirculationType                       string
	StageTransitions                      []StageTransition
}

type StageDefinition struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Rate int    `json:"rate"`
}

type StageTransition struct {
	Origin string `json:"origin"`
	Target string `json:"target"`
}

type FieldLayout struct {
	Key   string `json:"key"`
	Group string `json:"group"`
	Sort  int    `json:"sort"`
}

type FieldDefinition struct {
	ID       string   `json:"id"`
	ModuleID string   `json:"module_id"`
	Key      string   `json:"key"`
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Unique   bool     `json:"unique"`
	ReadOnly bool     `json:"read_only"`
	Options  []string `json:"options"`
	Sort     int      `json:"sort"`
}
