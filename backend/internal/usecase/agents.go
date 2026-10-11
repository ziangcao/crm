package usecase

import (
	"context"
	"slices"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

var AgentToolNames = []string{"record.search", "record.list", "record.get", "followup-plan.list"}

type AgentRepository interface {
	ListAgents(context.Context, string) ([]entity.AgentDefinition, error)
	FindAgent(context.Context, string) (entity.AgentDefinition, error)
	SaveAgent(context.Context, entity.AgentDefinition) error
	DeleteAgent(context.Context, string, string) error
}

type AgentUseCase struct{ repository AgentRepository }

type AgentInput struct {
	Actor            UserOutput
	ID, Name, Prompt string
	ToolNames        []string
	Enabled          bool
}

type AgentOutput struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Prompt         string    `json:"prompt"`
	ToolNames      []string  `json:"tool_names"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func NewAgent(repository AgentRepository) *AgentUseCase { return &AgentUseCase{repository: repository} }

func (u *AgentUseCase) List(ctx context.Context, actor UserOutput) ([]AgentOutput, error) {
	if err := requirePermission(actor, "AGENT:READ"); err != nil {
		return nil, err
	}
	if err := requireOrganization(actor); err != nil {
		return nil, err
	}
	items, err := u.repository.ListAgents(ctx, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	output := make([]AgentOutput, len(items))
	for index, item := range items {
		output[index] = agentOutput(item)
	}
	return output, nil
}

func (u *AgentUseCase) Save(ctx context.Context, input AgentInput) (AgentOutput, error) {
	action := "ADD"
	if input.ID != "" {
		action = "UPDATE"
	}
	if err := requirePermission(input.Actor, "AGENT:"+action); err != nil {
		return AgentOutput{}, err
	}
	if err := requireOrganization(input.Actor); err != nil {
		return AgentOutput{}, err
	}
	name, prompt := strings.TrimSpace(input.Name), strings.TrimSpace(input.Prompt)
	if name == "" || len([]rune(name)) > 100 || prompt == "" || len([]rune(prompt)) > 10000 || len(input.ToolNames) == 0 {
		return AgentOutput{}, ErrValidation
	}
	seen := map[string]bool{}
	for _, toolName := range input.ToolNames {
		if seen[toolName] || !slices.Contains(AgentToolNames, toolName) {
			return AgentOutput{}, ErrValidation
		}
		seen[toolName] = true
	}
	now := time.Now().UTC()
	item := entity.AgentDefinition{ID: input.ID, OrganizationID: input.Actor.OrganizationID, Name: name, Prompt: prompt, ToolNames: append([]string(nil), input.ToolNames...), Enabled: input.Enabled, UpdatedAt: now}
	if item.ID == "" {
		item.ID, item.CreatedAt = entity.NewID(), now
	} else {
		existing, err := u.repository.FindAgent(ctx, item.ID)
		if err != nil {
			return AgentOutput{}, ErrNotFound
		}
		if existing.OrganizationID != input.Actor.OrganizationID {
			return AgentOutput{}, ErrForbidden
		}
		item.CreatedAt = existing.CreatedAt
	}
	if err := u.repository.SaveAgent(ctx, item); err != nil {
		return AgentOutput{}, err
	}
	return agentOutput(item), nil
}

func (u *AgentUseCase) Delete(ctx context.Context, actor UserOutput, id string) error {
	if err := requirePermission(actor, "AGENT:DELETE"); err != nil {
		return err
	}
	if err := requireOrganization(actor); err != nil {
		return err
	}
	if id == "" {
		return ErrValidation
	}
	item, err := u.repository.FindAgent(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if item.OrganizationID != actor.OrganizationID {
		return ErrForbidden
	}
	return u.repository.DeleteAgent(ctx, id, actor.OrganizationID)
}

func agentOutput(item entity.AgentDefinition) AgentOutput {
	return AgentOutput{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Prompt: item.Prompt, ToolNames: item.ToolNames, Enabled: item.Enabled, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
