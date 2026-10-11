package usecase

import (
	"context"

	"cordialcrm/backend/internal/entity"
)

type OrganizationMigrationUseCase struct {
	repository Repository
	systems    OrganizationRepository
}

type OrganizationRepository interface {
	ListOrganizations(context.Context) ([]entity.Organization, error)
}

type AssignOrganizationInput struct {
	OrganizationID string
	Apply          bool
}

type AssignOrganizationOutput struct {
	OrganizationID string `json:"organization_id"`
	Users          int    `json:"users"`
	Records        int    `json:"records"`
	Applied        bool   `json:"applied"`
}

func NewOrganizationMigration(repository Repository, systems OrganizationRepository) *OrganizationMigrationUseCase {
	return &OrganizationMigrationUseCase{repository: repository, systems: systems}
}

func (u *OrganizationMigrationUseCase) AssignUnambiguous(ctx context.Context, apply bool) (AssignOrganizationOutput, bool, error) {
	organizations, err := u.systems.ListOrganizations(ctx)
	if err != nil {
		return AssignOrganizationOutput{}, false, err
	}
	if len(organizations) != 1 {
		return AssignOrganizationOutput{}, false, nil
	}
	output, err := u.Assign(ctx, AssignOrganizationInput{OrganizationID: organizations[0].ID, Apply: apply})
	return output, true, err
}

func (u *OrganizationMigrationUseCase) Assign(ctx context.Context, input AssignOrganizationInput) (AssignOrganizationOutput, error) {
	if input.OrganizationID == "" {
		return AssignOrganizationOutput{}, ErrValidation
	}
	organizations, err := u.systems.ListOrganizations(ctx)
	if err != nil {
		return AssignOrganizationOutput{}, err
	}
	found := false
	for _, organization := range organizations {
		if organization.ID == input.OrganizationID {
			found = true
			break
		}
	}
	if !found {
		return AssignOrganizationOutput{}, ErrNotFound
	}
	users, err := u.repository.ListUsers(ctx)
	if err != nil {
		return AssignOrganizationOutput{}, err
	}
	legacyUsers := make([]entity.User, 0)
	for _, user := range users {
		if user.OrganizationID == "" {
			user.OrganizationID = input.OrganizationID
			legacyUsers = append(legacyUsers, user)
		}
	}
	legacyRecords := make([]entity.Record, 0)
	for moduleKey := range Modules {
		records, err := u.repository.ListRecords(ctx, moduleKey)
		if err != nil {
			return AssignOrganizationOutput{}, err
		}
		for _, record := range records {
			if record.OrganizationID == "" {
				record.OrganizationID = input.OrganizationID
				legacyRecords = append(legacyRecords, record)
			}
		}
	}
	output := AssignOrganizationOutput{OrganizationID: input.OrganizationID, Users: len(legacyUsers), Records: len(legacyRecords), Applied: input.Apply}
	if !input.Apply {
		return output, nil
	}
	if err := u.repository.WithinTransaction(ctx, func(transactionContext context.Context) error {
		for _, user := range legacyUsers {
			if err := u.repository.UpdateUser(transactionContext, user); err != nil {
				return err
			}
		}
		for _, record := range legacyRecords {
			if err := u.repository.UpdateRecord(transactionContext, record, record.Version); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return AssignOrganizationOutput{}, err
	}
	return output, nil
}
