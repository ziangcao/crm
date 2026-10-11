package usecase

import (
	"context"
	"testing"

	"cordialcrm/backend/internal/entity"
)

type organizationMigrationRepository struct {
	Repository
	organizations []entity.Organization
	users         []entity.User
	records       map[string][]entity.Record
}

func (repository *organizationMigrationRepository) WithinTransaction(ctx context.Context, action func(context.Context) error) error {
	return action(ctx)
}
func (repository *organizationMigrationRepository) ListOrganizations(context.Context) ([]entity.Organization, error) {
	return repository.organizations, nil
}
func (repository *organizationMigrationRepository) ListUsers(context.Context) ([]entity.User, error) {
	return append([]entity.User(nil), repository.users...), nil
}
func (repository *organizationMigrationRepository) UpdateUser(_ context.Context, updated entity.User) error {
	for index, user := range repository.users {
		if user.ID == updated.ID {
			repository.users[index] = updated
		}
	}
	return nil
}
func (repository *organizationMigrationRepository) ListRecords(_ context.Context, module string) ([]entity.Record, error) {
	return append([]entity.Record(nil), repository.records[module]...), nil
}
func (repository *organizationMigrationRepository) UpdateRecord(_ context.Context, updated entity.Record, _ int) error {
	for index, record := range repository.records[updated.Module] {
		if record.ID == updated.ID {
			repository.records[updated.Module][index] = updated
		}
	}
	return nil
}

func TestAssignOrganizationPreviewAndApply(t *testing.T) {
	repository := &organizationMigrationRepository{
		organizations: []entity.Organization{{ID: "org-a"}},
		users:         []entity.User{{ID: "legacy"}, {ID: "assigned", OrganizationID: "org-b"}},
		records: map[string][]entity.Record{
			"customers": {{ID: "legacy-record", Module: "customers", Version: 1}, {ID: "assigned-record", Module: "customers", OrganizationID: "org-b", Version: 1}},
		},
	}
	service := NewOrganizationMigration(repository, repository)
	preview, err := service.Assign(context.Background(), AssignOrganizationInput{OrganizationID: "org-a"})
	if err != nil || preview.Applied || preview.Users != 1 || preview.Records != 1 {
		t.Fatalf("unexpected preview: %#v, %v", preview, err)
	}
	if repository.users[0].OrganizationID != "" || repository.records["customers"][0].OrganizationID != "" {
		t.Fatal("preview changed persisted data")
	}
	result, err := service.Assign(context.Background(), AssignOrganizationInput{OrganizationID: "org-a", Apply: true})
	if err != nil || !result.Applied || repository.users[0].OrganizationID != "org-a" || repository.records["customers"][0].OrganizationID != "org-a" {
		t.Fatalf("unexpected applied result: %#v, %#v, %#v, %v", result, repository.users, repository.records, err)
	}
	if repository.users[1].OrganizationID != "org-b" || repository.records["customers"][1].OrganizationID != "org-b" {
		t.Fatal("migration changed assigned data")
	}
}

func TestAssignOrganizationRejectsUnknownOrganization(t *testing.T) {
	service := NewOrganizationMigration(&organizationMigrationRepository{}, &organizationMigrationRepository{})
	if _, err := service.Assign(context.Background(), AssignOrganizationInput{OrganizationID: "missing"}); err != ErrNotFound {
		t.Fatalf("expected missing organization error, got %v", err)
	}
}

func TestAssignUnambiguousOnlyMigratesSingleOrganization(t *testing.T) {
	repository := &organizationMigrationRepository{organizations: []entity.Organization{{ID: "org-a"}}, users: []entity.User{{ID: "legacy"}}, records: map[string][]entity.Record{}}
	result, unambiguous, err := NewOrganizationMigration(repository, repository).AssignUnambiguous(context.Background(), true)
	if err != nil || !unambiguous || result.Users != 1 || repository.users[0].OrganizationID != "org-a" {
		t.Fatalf("single organization migration failed: %#v, %v, %v", result, unambiguous, err)
	}
	repository.organizations = append(repository.organizations, entity.Organization{ID: "org-b"})
	repository.users = append(repository.users, entity.User{ID: "another-legacy"})
	_, unambiguous, err = NewOrganizationMigration(repository, repository).AssignUnambiguous(context.Background(), true)
	if err != nil || unambiguous || repository.users[1].OrganizationID != "" {
		t.Fatalf("ambiguous migration should be skipped: %v, %v", unambiguous, err)
	}
}
