package usecase

import (
	"context"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
)

type setupRepositoryStub struct {
	Repository
	organization entity.Organization
	user         entity.User
}

func (repository *setupRepositoryStub) WithinTransaction(ctx context.Context, action func(context.Context) error) error {
	return action(ctx)
}
func (*setupRepositoryStub) CountUsers(context.Context) (int, error) { return 0, nil }
func (repository *setupRepositoryStub) SaveOrganization(_ context.Context, organization entity.Organization) error {
	repository.organization = organization
	return nil
}
func (repository *setupRepositoryStub) CreateUser(_ context.Context, user entity.User) error {
	repository.user = user
	return nil
}
func (*setupRepositoryStub) CreateSession(context.Context, string, string, time.Time) error {
	return nil
}
func (*setupRepositoryStub) CreateAudit(context.Context, entity.Audit) error { return nil }

type setupPasswords struct{}

func (setupPasswords) Hash(string) (string, error)  { return "hash", nil }
func (setupPasswords) Compare(string, string) error { return nil }

type setupTokens struct{}

func (setupTokens) New() (string, error) { return "token", nil }

func TestSetupCreatesAndAssignsDefaultOrganization(t *testing.T) {
	repository := &setupRepositoryStub{}
	output, err := NewAuth(repository, setupPasswords{}, setupTokens{}, time.Hour).Setup(context.Background(), SetupInput{Username: "admin", Password: "Password123", Name: "管理员"})
	if err != nil {
		t.Fatal(err)
	}
	if repository.organization.ID == "" || repository.organization.Name != "默认组织" {
		t.Fatalf("default organization was not created: %#v", repository.organization)
	}
	if repository.user.OrganizationID != repository.organization.ID || output.User.OrganizationID != repository.organization.ID {
		t.Fatalf("administrator was not assigned to the default organization: %#v", repository.user)
	}
}
