package usecase

import (
	"context"
	"time"

	"cordialcrm/backend/internal/entity"
)

type CustomerRecycleUseCase struct {
	repo    CustomerRecycleRepository
	records *RecordUseCase
	clock   func() time.Time
}

func NewCustomerRecycle(repo CustomerRecycleRepository, records *RecordUseCase) *CustomerRecycleUseCase {
	return &CustomerRecycleUseCase{repo: repo, records: records, clock: time.Now}
}

type RunCustomerRecycleInput struct {
	IdleDays int
	Now      time.Time
}
type RunCustomerRecycleOutput struct {
	Candidates int
	Recycled   int
}

func (u *CustomerRecycleUseCase) Run(ctx context.Context, input RunCustomerRecycleInput) (RunCustomerRecycleOutput, error) {
	if input.IdleDays <= 0 {
		return RunCustomerRecycleOutput{}, nil
	}
	now := input.Now
	if now.IsZero() {
		now = u.clock()
	}
	items, err := u.repo.ListIdleOwnedCustomers(ctx, now.UTC().AddDate(0, 0, -input.IdleDays))
	if err != nil {
		return RunCustomerRecycleOutput{}, err
	}
	output := RunCustomerRecycleOutput{Candidates: len(items)}
	if len(items) == 0 {
		return output, nil
	}
	users, err := u.repo.ListUsers(ctx)
	if err != nil {
		return RunCustomerRecycleOutput{}, err
	}
	var actor UserOutput
	for _, user := range users {
		if user.Active && user.Role == entity.RoleAdmin {
			actor = publicUser(user)
			break
		}
	}
	if actor.ID == "" {
		return RunCustomerRecycleOutput{}, entity.NewBusinessError("validation", "自动回收需要启用的管理员账号")
	}
	for _, item := range items {
		if _, err := u.records.Action(ctx, ActionInput{Actor: actor, Module: "customers", ID: item.ID, Action: "release", Version: item.Version}); err != nil {
			return RunCustomerRecycleOutput{}, err
		}
		output.Recycled++
	}
	return output, nil
}
