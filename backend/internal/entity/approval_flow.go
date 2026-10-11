package entity

import "time"

type ApprovalFlow struct {
	ID, Module, Name string
	Enabled          bool
	Version          int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type ApprovalFlowVersion struct {
	ID, FlowID string
	Version    int
	Steps      []ApprovalFlowStep
	CreatedAt  time.Time
}

type ApprovalFlowStep struct {
	Order         int
	ApproverRoles []Role
	ApproverIDs   []string
	MinAmount     string
	Conditions    []ApprovalCondition
}

type ApprovalCondition struct{ Field, Operator, Value string }
