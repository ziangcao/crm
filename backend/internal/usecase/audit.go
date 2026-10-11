package usecase

import (
	"context"
	"time"
)

type AuditUseCase struct{ repo Repository }

func NewAudit(repo Repository) *AuditUseCase { return &AuditUseCase{repo: repo} }

type ListAuditInput struct {
	Actor UserOutput
	Page  int
}
type ListAuditOutput struct {
	Items []AuditOutput `json:"items"`
	Total int           `json:"total"`
}
type AuditOutput struct {
	ID        string    `json:"id"`
	UserName  string    `json:"user_name"`
	Action    string    `json:"action"`
	Module    string    `json:"module"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (u *AuditUseCase) List(ctx context.Context, input ListAuditInput) (ListAuditOutput, error) {
	if err := requirePermission(input.Actor, "OPERATION_LOG:READ"); err != nil {
		return ListAuditOutput{}, err
	}
	if input.Page < 1 {
		input.Page = 1
	}
	items, total, err := u.repo.ListAudit(ctx, input.Page, 30)
	if err != nil {
		return ListAuditOutput{}, err
	}
	output := make([]AuditOutput, 0, len(items))
	for _, item := range items {
		user, err := u.repo.FindUserByID(ctx, item.UserID)
		if err != nil {
			return ListAuditOutput{}, err
		}
		output = append(output, AuditOutput{ID: item.ID, UserName: user.Name, Action: item.Action, Module: item.Module, Detail: item.Detail, CreatedAt: item.CreatedAt})
	}
	return ListAuditOutput{Items: output, Total: total}, nil
}
