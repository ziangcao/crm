package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
	"github.com/shopspring/decimal"
)

type ApprovalFlowUseCase struct{ repository ApprovalFlowRepository }

func NewApprovalFlow(repository ApprovalFlowRepository) *ApprovalFlowUseCase {
	return &ApprovalFlowUseCase{repository: repository}
}

type ApprovalFlowOutput struct {
	ID        string                    `json:"id"`
	Module    string                    `json:"module"`
	Name      string                    `json:"name"`
	Enabled   bool                      `json:"enabled"`
	Version   int                       `json:"version"`
	Steps     []entity.ApprovalFlowStep `json:"steps,omitempty"`
	CreatedAt time.Time                 `json:"created_at"`
	UpdatedAt time.Time                 `json:"updated_at"`
}

type ListApprovalFlowsInput struct {
	Actor  UserOutput
	Module string
}

type ListApprovalFlowsOutput struct {
	Items []ApprovalFlowOutput `json:"items"`
}

type SaveApprovalFlowInput struct {
	Actor   UserOutput
	ID      string
	Module  string
	Name    string
	Enabled bool
	Steps   []entity.ApprovalFlowStep
}

type SaveApprovalFlowOutput struct {
	Flow ApprovalFlowOutput `json:"flow"`
}

type SetApprovalFlowEnabledInput struct {
	Actor   UserOutput
	ID      string
	Enabled bool
}

type SetApprovalFlowEnabledOutput struct {
	Flow ApprovalFlowOutput `json:"flow"`
}

func (u *ApprovalFlowUseCase) List(ctx context.Context, input ListApprovalFlowsInput) (ListApprovalFlowsOutput, error) {
	if err := requirePermission(input.Actor, "PROCESS_SETTING:READ"); err != nil {
		return ListApprovalFlowsOutput{}, err
	}
	if input.Module != "" {
		if _, err := module(input.Module); err != nil {
			return ListApprovalFlowsOutput{}, err
		}
	}
	flows, err := u.repository.ListApprovalFlows(ctx, input.Module)
	if err != nil {
		return ListApprovalFlowsOutput{}, err
	}
	items := make([]ApprovalFlowOutput, 0, len(flows))
	for _, flow := range flows {
		version, err := u.repository.FindCurrentApprovalFlowVersion(ctx, flow.ID)
		if err != nil {
			return ListApprovalFlowsOutput{}, err
		}
		items = append(items, outputApprovalFlow(flow, version))
	}
	return ListApprovalFlowsOutput{Items: items}, nil
}

func (u *ApprovalFlowUseCase) Save(ctx context.Context, input SaveApprovalFlowInput) (SaveApprovalFlowOutput, error) {
	action := "ADD"
	if input.ID != "" {
		action = "UPDATE"
	}
	if err := requirePermission(input.Actor, "PROCESS_SETTING:"+action); err != nil {
		return SaveApprovalFlowOutput{}, err
	}
	if strings.TrimSpace(input.Name) == "" {
		return SaveApprovalFlowOutput{}, ErrValidation
	}
	definition, err := module(input.Module)
	if err != nil {
		return SaveApprovalFlowOutput{}, err
	}
	if !definition.Approvable {
		return SaveApprovalFlowOutput{}, ErrValidation
	}
	if err := validateApprovalFlowSteps(input.Steps); err != nil {
		return SaveApprovalFlowOutput{}, err
	}
	if err := u.validateApprovalFlowConditions(ctx, definition, input.Steps); err != nil {
		return SaveApprovalFlowOutput{}, err
	}
	if err := u.validateApprovers(ctx, input.Steps); err != nil {
		return SaveApprovalFlowOutput{}, err
	}

	var saved entity.ApprovalFlow
	var version entity.ApprovalFlowVersion
	err = u.repository.WithinTransaction(ctx, func(transactionContext context.Context) error {
		now := time.Now().UTC()
		flow := entity.ApprovalFlow{ID: input.ID, Module: input.Module, Name: strings.TrimSpace(input.Name), Enabled: input.Enabled}
		if input.ID == "" {
			flow.ID = entity.NewID()
			flow.Version = 1
			flow.CreatedAt = now
		} else {
			existing, err := u.flowByID(transactionContext, input.ID)
			if err != nil {
				return err
			}
			if existing.Module != input.Module {
				return ErrValidation
			}
			flow.Version = existing.Version + 1
			flow.CreatedAt = existing.CreatedAt
		}
		flow.UpdatedAt = now
		version = entity.ApprovalFlowVersion{ID: entity.NewID(), FlowID: flow.ID, Version: flow.Version, Steps: input.Steps, CreatedAt: now}
		if err := u.repository.SaveApprovalFlow(transactionContext, flow); err != nil {
			return err
		}
		if err := u.repository.SaveApprovalFlowVersion(transactionContext, version); err != nil {
			return err
		}
		if flow.Enabled {
			if err := u.disableOtherFlows(transactionContext, flow); err != nil {
				return err
			}
		}
		saved = flow
		return nil
	})
	if err != nil {
		return SaveApprovalFlowOutput{}, err
	}
	return SaveApprovalFlowOutput{Flow: outputApprovalFlow(saved, version)}, nil
}

func (u *ApprovalFlowUseCase) SetEnabled(ctx context.Context, input SetApprovalFlowEnabledInput) (SetApprovalFlowEnabledOutput, error) {
	if err := requirePermission(input.Actor, "PROCESS_SETTING:UPDATE"); err != nil {
		return SetApprovalFlowEnabledOutput{}, err
	}
	var saved entity.ApprovalFlow
	var version entity.ApprovalFlowVersion
	err := u.repository.WithinTransaction(ctx, func(transactionContext context.Context) error {
		flow, err := u.flowByID(transactionContext, input.ID)
		if err != nil {
			return err
		}
		version, err = u.repository.FindCurrentApprovalFlowVersion(transactionContext, flow.ID)
		if err != nil {
			return ErrNotFound
		}
		flow.Enabled = input.Enabled
		flow.UpdatedAt = time.Now().UTC()
		if err := u.repository.SaveApprovalFlow(transactionContext, flow); err != nil {
			return err
		}
		if input.Enabled {
			if err := u.disableOtherFlows(transactionContext, flow); err != nil {
				return err
			}
		}
		saved = flow
		return nil
	})
	if err != nil {
		return SetApprovalFlowEnabledOutput{}, err
	}
	return SetApprovalFlowEnabledOutput{Flow: outputApprovalFlow(saved, version)}, nil
}

func (u *ApprovalFlowUseCase) flowByID(ctx context.Context, id string) (entity.ApprovalFlow, error) {
	flows, err := u.repository.ListApprovalFlows(ctx, "")
	if err != nil {
		return entity.ApprovalFlow{}, err
	}
	for _, flow := range flows {
		if flow.ID == id {
			return flow, nil
		}
	}
	return entity.ApprovalFlow{}, ErrNotFound
}

func (u *ApprovalFlowUseCase) disableOtherFlows(ctx context.Context, target entity.ApprovalFlow) error {
	flows, err := u.repository.ListApprovalFlows(ctx, target.Module)
	if err != nil {
		return err
	}
	for _, flow := range flows {
		if flow.ID != target.ID && flow.Enabled {
			flow.Enabled = false
			flow.UpdatedAt = target.UpdatedAt
			if err := u.repository.SaveApprovalFlow(ctx, flow); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateApprovalFlowSteps(steps []entity.ApprovalFlowStep) error {
	if len(steps) == 0 {
		return ErrValidation
	}
	validRoles := map[entity.Role]bool{entity.RoleAdmin: true, entity.RoleManager: true, entity.RoleSales: true}
	for index, step := range steps {
		if step.Order != index+1 || (len(step.ApproverRoles) == 0 && len(step.ApproverIDs) == 0) {
			return ErrValidation
		}
		if step.MinAmount != "" {
			amount, err := decimal.NewFromString(step.MinAmount)
			if err != nil || !amount.IsPositive() {
				return ErrValidation
			}
		}
		for _, role := range step.ApproverRoles {
			if !validRoles[role] {
				return ErrValidation
			}
		}
		seen := map[string]bool{}
		for _, userID := range step.ApproverIDs {
			if userID == "" || seen[userID] {
				return ErrValidation
			}
			seen[userID] = true
		}
		for _, condition := range step.Conditions {
			if condition.Field == "" || (condition.Operator != "eq" && condition.Operator != "neq" && condition.Operator != "contains" && condition.Operator != "empty") {
				return ErrValidation
			}
			if (condition.Operator == "empty" && strings.TrimSpace(condition.Value) != "") || (condition.Operator != "empty" && strings.TrimSpace(condition.Value) == "") {
				return ErrValidation
			}
		}
	}
	if steps[len(steps)-1].MinAmount != "" {
		return ErrValidation
	}
	return nil
}

func (u *ApprovalFlowUseCase) validateApprovers(ctx context.Context, steps []entity.ApprovalFlowStep) error {
	users, err := u.repository.ListUsers(ctx)
	if err != nil {
		return err
	}
	byID := make(map[string]entity.User, len(users))
	for _, user := range users {
		byID[user.ID] = user
	}
	for _, step := range steps {
		for _, userID := range step.ApproverIDs {
			user, found := byID[userID]
			if !found || !user.Active || (user.Role != entity.RoleAdmin && user.Role != entity.RoleManager) {
				return ErrValidation
			}
		}
	}
	return nil
}

func (u *ApprovalFlowUseCase) validateApprovalFlowConditions(ctx context.Context, definition Module, steps []entity.ApprovalFlowStep) error {
	fields := map[string]bool{"amount": true}
	settings, err := u.repository.ListStandardFieldSettings(ctx, definition.Key)
	if err != nil {
		return err
	}
	archived := map[string]bool{}
	for _, setting := range settings {
		archived[setting.Key] = setting.Archived
	}
	for _, field := range definition.Fields {
		if !archived[field.Key] {
			fields[field.Key] = true
		}
	}
	customFields, err := u.repository.ListCustomFields(ctx, definition.Key)
	if err != nil {
		return err
	}
	for _, field := range customFields {
		if field.Definition["archived"] != true {
			fields[field.Key] = true
		}
	}
	for _, step := range steps {
		for _, condition := range step.Conditions {
			if !fields[condition.Field] {
				return ErrValidation
			}
		}
	}
	return nil
}

func nextApplicableApprovalStep(steps []entity.ApprovalFlowStep, record entity.Record, after int) int {
	amount, amountError := decimal.NewFromString(strings.TrimSpace(fmt.Sprint(record.Data["amount"])))
	for index := after; index < len(steps); index++ {
		if !approvalStepMatches(steps[index], record) {
			continue
		}
		if steps[index].MinAmount == "" {
			return index + 1
		}
		minimum, err := decimal.NewFromString(steps[index].MinAmount)
		if err == nil && amountError == nil && amount.GreaterThanOrEqual(minimum) {
			return index + 1
		}
	}
	return 0
}

func approvalStepMatches(step entity.ApprovalFlowStep, record entity.Record) bool {
	for _, condition := range step.Conditions {
		value := strings.TrimSpace(fmt.Sprint(record.Data[condition.Field]))
		expected := strings.TrimSpace(condition.Value)
		switch condition.Operator {
		case "eq":
			if value != expected {
				return false
			}
		case "neq":
			if value == expected {
				return false
			}
		case "contains":
			if !strings.Contains(value, expected) {
				return false
			}
		case "empty":
			if value != "" {
				return false
			}
		}
	}
	return true
}

func outputApprovalFlow(flow entity.ApprovalFlow, version entity.ApprovalFlowVersion) ApprovalFlowOutput {
	return ApprovalFlowOutput{ID: flow.ID, Module: flow.Module, Name: flow.Name, Enabled: flow.Enabled, Version: flow.Version, Steps: version.Steps, CreatedAt: flow.CreatedAt, UpdatedAt: flow.UpdatedAt}
}
