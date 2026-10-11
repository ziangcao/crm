package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type agentRepositoryStub struct {
	items   []entity.AgentDefinition
	deleted string
}

func (s *agentRepositoryStub) ListAgents(_ context.Context, organizationID string) ([]entity.AgentDefinition, error) {
	result := []entity.AgentDefinition{}
	for _, item := range s.items {
		if item.OrganizationID == organizationID {
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *agentRepositoryStub) FindAgent(_ context.Context, id string) (entity.AgentDefinition, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.AgentDefinition{}, ErrNotFound
}
func (s *agentRepositoryStub) SaveAgent(_ context.Context, item entity.AgentDefinition) error {
	for index := range s.items {
		if s.items[index].ID == item.ID {
			s.items[index] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}
func (s *agentRepositoryStub) DeleteAgent(_ context.Context, id, _ string) error {
	s.deleted = id
	return nil
}

func TestAgentCRUDValidatesPermissionsToolsAndOrganization(t *testing.T) {
	repository := &agentRepositoryStub{}
	service := NewAgent(repository)
	actor := UserOutput{OrganizationID: "org-1", RoleID: "role-1", PermissionIDs: []string{"AGENT:READ", "AGENT:ADD", "AGENT:UPDATE", "AGENT:DELETE"}}
	created, err := service.Save(context.Background(), AgentInput{Actor: actor, Name: "销售助手", Prompt: "帮助销售查询客户", ToolNames: []string{"record.search"}, Enabled: true})
	if err != nil || created.Name != "销售助手" || len(repository.items) != 1 {
		t.Fatalf("agent create failed: %#v err=%v", created, err)
	}
	if _, err := service.Save(context.Background(), AgentInput{Actor: actor, Name: "错误助手", Prompt: "test", ToolNames: []string{"unknown"}}); err != ErrValidation {
		t.Fatalf("invalid tool accepted: %v", err)
	}
	repository.items = append(repository.items, entity.AgentDefinition{ID: "foreign", OrganizationID: "org-2", Name: "其他", Prompt: "test", ToolNames: []string{"record.get"}})
	if _, err := service.Save(context.Background(), AgentInput{Actor: actor, ID: "foreign", Name: "越界", Prompt: "test", ToolNames: []string{"record.get"}}); err != ErrForbidden {
		t.Fatalf("cross-organization update accepted: %v", err)
	}
	if err := service.Delete(context.Background(), actor, created.ID); err != nil || repository.deleted != created.ID {
		t.Fatalf("agent delete failed: %v", err)
	}
	unassigned := actor
	unassigned.OrganizationID = ""
	if _, err := service.List(context.Background(), unassigned); err == nil {
		t.Fatal("unassigned actor accessed organization agents")
	}
}
