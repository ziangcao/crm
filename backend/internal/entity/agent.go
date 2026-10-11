package entity

import "time"

type AgentDefinition struct {
	ID, OrganizationID, Name, Prompt string
	ToolNames                        []string
	Enabled                          bool
	CreatedAt, UpdatedAt             time.Time
}
