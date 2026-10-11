package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type assignedRoleRepository struct {
	Repository
	role        entity.SystemRole
	departments []entity.Department
}

func (repository assignedRoleRepository) ListDepartments(_ context.Context, organizationID string) ([]entity.Department, error) {
	items := make([]entity.Department, 0, len(repository.departments))
	for _, department := range repository.departments {
		if department.OrganizationID == organizationID {
			items = append(items, department)
		}
	}
	return items, nil
}

func (repository assignedRoleRepository) FindRoleByID(_ context.Context, id string) (entity.SystemRole, error) {
	if id == repository.role.ID {
		return repository.role, nil
	}
	return entity.SystemRole{}, ErrNotFound
}

func TestCustomRoleAssignmentRejectsForeignDepartment(t *testing.T) {
	service := NewUser(assignedRoleRepository{departments: []entity.Department{{ID: "east", OrganizationID: "org-a"}}}, nil)
	if err := service.validateDepartment(context.Background(), "org-a", "east"); err != nil {
		t.Fatalf("valid department rejected: %v", err)
	}
	if err := service.validateDepartment(context.Background(), "org-a", "west"); err == nil {
		t.Fatal("accepted foreign department")
	}
}

func TestCustomRoleAssignmentRejectsElevationAndCrossOrganization(t *testing.T) {
	service := NewUser(assignedRoleRepository{role: entity.SystemRole{ID: "role-a", OrganizationID: "org-a"}}, nil)
	for _, input := range []struct {
		role                   entity.Role
		organizationID, roleID string
	}{
		{entity.RoleAdmin, "org-a", "role-a"},
		{entity.RoleManager, "org-a", "role-a"},
		{entity.RoleSales, "org-b", "role-a"},
		{entity.RoleSales, "org-a", "missing"},
		{entity.RoleSales, "org-a", ""},
	} {
		if err := service.validateAssignment(context.Background(), input.role, input.organizationID, input.roleID); err == nil {
			t.Fatalf("accepted invalid assignment: %#v", input)
		}
	}
	if err := service.validateAssignment(context.Background(), entity.RoleSales, "org-a", "role-a"); err != nil {
		t.Fatalf("valid assignment rejected: %v", err)
	}
}
