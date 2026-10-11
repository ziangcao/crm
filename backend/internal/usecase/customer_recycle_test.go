package usecase

import (
	"context"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
)

type customerRecycleRepositoryStub struct {
	items []entity.Record
}

func (s customerRecycleRepositoryStub) ListIdleOwnedCustomers(_ context.Context, _ time.Time) ([]entity.Record, error) {
	return s.items, nil
}
func (customerRecycleRepositoryStub) ListUsers(context.Context) ([]entity.User, error) {
	return nil, nil
}

func TestCustomerRecycleSkipsDisabledAndEmptyRuns(t *testing.T) {
	service := NewCustomerRecycle(customerRecycleRepositoryStub{}, nil)
	disabled, err := service.Run(context.Background(), RunCustomerRecycleInput{IdleDays: 0})
	if err != nil || disabled.Candidates != 0 || disabled.Recycled != 0 {
		t.Fatalf("unexpected disabled run: %#v, err=%v", disabled, err)
	}
	empty, err := service.Run(context.Background(), RunCustomerRecycleInput{IdleDays: 30, Now: time.Now().UTC()})
	if err != nil || empty.Candidates != 0 || empty.Recycled != 0 {
		t.Fatalf("unexpected empty run: %#v, err=%v", empty, err)
	}
}
