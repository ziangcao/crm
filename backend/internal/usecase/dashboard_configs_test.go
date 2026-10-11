package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type dashboardRepositoryStub struct{ items []entity.DashboardDefinition }

func (s *dashboardRepositoryStub) ListDashboards(_ context.Context, organizationID string) ([]entity.DashboardDefinition, error) {
	result := []entity.DashboardDefinition{}
	for _, item := range s.items {
		if item.OrganizationID == organizationID {
			result = append(result, item)
		}
	}
	return result, nil
}
func (s *dashboardRepositoryStub) FindDashboard(_ context.Context, id string) (entity.DashboardDefinition, error) {
	for _, item := range s.items {
		if item.ID == id {
			return item, nil
		}
	}
	return entity.DashboardDefinition{}, ErrNotFound
}
func (s *dashboardRepositoryStub) SaveDashboard(_ context.Context, item entity.DashboardDefinition) error {
	for index := range s.items {
		if s.items[index].ID == item.ID {
			s.items[index] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}
func (s *dashboardRepositoryStub) DeleteDashboard(_ context.Context, _, _ string) error { return nil }

func TestDashboardConfigValidatesLayoutAndOrganization(t *testing.T) {
	repository := &dashboardRepositoryStub{}
	service := NewDashboardConfig(repository)
	actor := UserOutput{OrganizationID: "org-1", RoleID: "role-1", PermissionIDs: []string{"DASHBOARD:READ", "DASHBOARD:ADD", "DASHBOARD:UPDATE", "DASHBOARD:DELETE"}}
	created, err := service.Save(context.Background(), DashboardConfigInput{Actor: actor, Name: "销售看板", Widgets: []entity.DashboardWidget{{Key: "stats", Width: 2}, {Key: "funnel", Width: 1}}, IsDefault: true})
	if err != nil || created.Name != "销售看板" {
		t.Fatalf("dashboard create failed: %#v err=%v", created, err)
	}
	if _, err := service.Save(context.Background(), DashboardConfigInput{Actor: actor, Name: "错误", Widgets: []entity.DashboardWidget{{Key: "unknown", Width: 1}}}); err != ErrValidation {
		t.Fatalf("invalid widget accepted: %v", err)
	}
	repository.items = append(repository.items, entity.DashboardDefinition{ID: "foreign", OrganizationID: "org-2"})
	if _, err := service.Save(context.Background(), DashboardConfigInput{Actor: actor, ID: "foreign", Name: "越界", Widgets: []entity.DashboardWidget{{Key: "stats", Width: 2}}}); err != ErrForbidden {
		t.Fatalf("cross-organization update accepted: %v", err)
	}
	unassigned := actor
	unassigned.OrganizationID = ""
	if _, err := service.List(context.Background(), unassigned); err == nil {
		t.Fatal("unassigned actor accessed organization dashboards")
	}
}
