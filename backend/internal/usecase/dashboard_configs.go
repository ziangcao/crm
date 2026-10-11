package usecase

import (
	"context"
	"slices"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

var DashboardWidgetKeys = []string{"stats", "performance", "funnel", "tasks", "activities", "overdue"}

type DashboardRepository interface {
	ListDashboards(context.Context, string) ([]entity.DashboardDefinition, error)
	FindDashboard(context.Context, string) (entity.DashboardDefinition, error)
	SaveDashboard(context.Context, entity.DashboardDefinition) error
	DeleteDashboard(context.Context, string, string) error
}

type DashboardConfigUseCase struct{ repository DashboardRepository }

type DashboardConfigInput struct {
	Actor     UserOutput
	ID, Name  string
	Widgets   []entity.DashboardWidget
	IsDefault bool
}

type DashboardConfigOutput struct {
	ID             string                   `json:"id"`
	OrganizationID string                   `json:"organization_id"`
	Name           string                   `json:"name"`
	Widgets        []entity.DashboardWidget `json:"widgets"`
	IsDefault      bool                     `json:"is_default"`
	CreatedAt      time.Time                `json:"created_at"`
	UpdatedAt      time.Time                `json:"updated_at"`
}

func NewDashboardConfig(repository DashboardRepository) *DashboardConfigUseCase {
	return &DashboardConfigUseCase{repository: repository}
}

func (u *DashboardConfigUseCase) List(ctx context.Context, actor UserOutput) ([]DashboardConfigOutput, error) {
	if err := requirePermission(actor, "DASHBOARD:READ"); err != nil {
		return nil, err
	}
	if err := requireOrganization(actor); err != nil {
		return nil, err
	}
	items, err := u.repository.ListDashboards(ctx, actor.OrganizationID)
	if err != nil {
		return nil, err
	}
	output := make([]DashboardConfigOutput, len(items))
	for index, item := range items {
		output[index] = dashboardConfigOutput(item)
	}
	return output, nil
}

func (u *DashboardConfigUseCase) Save(ctx context.Context, input DashboardConfigInput) (DashboardConfigOutput, error) {
	action := "ADD"
	if input.ID != "" {
		action = "UPDATE"
	}
	if err := requirePermission(input.Actor, "DASHBOARD:"+action); err != nil {
		return DashboardConfigOutput{}, err
	}
	if err := requireOrganization(input.Actor); err != nil {
		return DashboardConfigOutput{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 100 || len(input.Widgets) == 0 || len(input.Widgets) > len(DashboardWidgetKeys) {
		return DashboardConfigOutput{}, ErrValidation
	}
	seen := map[string]bool{}
	for _, widget := range input.Widgets {
		if seen[widget.Key] || !slices.Contains(DashboardWidgetKeys, widget.Key) || (widget.Width != 1 && widget.Width != 2) {
			return DashboardConfigOutput{}, ErrValidation
		}
		seen[widget.Key] = true
	}
	now := time.Now().UTC()
	item := entity.DashboardDefinition{ID: input.ID, OrganizationID: input.Actor.OrganizationID, Name: name, Widgets: append([]entity.DashboardWidget(nil), input.Widgets...), IsDefault: input.IsDefault, UpdatedAt: now}
	if item.ID == "" {
		item.ID, item.CreatedAt = entity.NewID(), now
	} else {
		existing, err := u.repository.FindDashboard(ctx, item.ID)
		if err != nil {
			return DashboardConfigOutput{}, ErrNotFound
		}
		if existing.OrganizationID != input.Actor.OrganizationID {
			return DashboardConfigOutput{}, ErrForbidden
		}
		item.CreatedAt = existing.CreatedAt
	}
	if err := u.repository.SaveDashboard(ctx, item); err != nil {
		return DashboardConfigOutput{}, err
	}
	return dashboardConfigOutput(item), nil
}

func (u *DashboardConfigUseCase) Delete(ctx context.Context, actor UserOutput, id string) error {
	if err := requirePermission(actor, "DASHBOARD:DELETE"); err != nil {
		return err
	}
	if err := requireOrganization(actor); err != nil {
		return err
	}
	item, err := u.repository.FindDashboard(ctx, id)
	if err != nil {
		return ErrNotFound
	}
	if item.OrganizationID != actor.OrganizationID {
		return ErrForbidden
	}
	return u.repository.DeleteDashboard(ctx, id, actor.OrganizationID)
}

func dashboardConfigOutput(item entity.DashboardDefinition) DashboardConfigOutput {
	return DashboardConfigOutput{ID: item.ID, OrganizationID: item.OrganizationID, Name: item.Name, Widgets: item.Widgets, IsDefault: item.IsDefault, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}
