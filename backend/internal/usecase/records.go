package usecase

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
)

type AddCustomFieldInput struct {
	Actor                    UserOutput
	Module, Key, Label, Type string
	Options                  []string
	Sources                  []string
	Columns                  []Field
}

type RecordUseCase struct {
	repo       Repository
	modules    ModuleCatalogRepository
	ownerships OwnershipHistoryRepository
}

func NewRecord(repo Repository, modules ModuleCatalogRepository, ownerships ...OwnershipHistoryRepository) *RecordUseCase {
	service := &RecordUseCase{repo: repo, modules: modules}
	if len(ownerships) > 0 {
		service.ownerships = ownerships[0]
	}
	return service
}

type ListRecordsInput struct {
	Actor                        UserOutput
	Module, Query, Scope, Status string
	Page, Size                   int
}
type RecordInput struct {
	Actor      UserOutput
	Module, ID string
	Data       map[string]any
	Version    int
}
type ActionInput struct {
	Actor              UserOutput
	Module, ID, Action string
	Reason             string
	TargetStage        string
	Version            int
	FreezeDays         int
	OwnerID            *string
	ItemIndexes        []int
	MergeIDs           []string
}
type FollowupInput struct {
	Actor                             UserOutput
	Module, RecordID, Content, Method string
	NextDate                          *time.Time
}
type FollowupCommentInput struct {
	Actor                                 UserOutput
	Module, RecordID, FollowupID, Content string
}
type UpdateFollowupCommentInput struct {
	FollowupCommentInput
	CommentID string
}
type DeleteFollowupCommentInput struct {
	Actor                            UserOutput
	Module, RecordID, FollowupID, ID string
}
type ListOwnershipHistoryInput struct {
	Actor      UserOutput
	Module, ID string
}
type RecordOutput struct {
	ID         string            `json:"id"`
	Module     string            `json:"module"`
	Data       map[string]any    `json:"data"`
	References map[string]string `json:"references"`
	OwnerID    *string           `json:"owner_id"`
	OwnerName  string            `json:"owner_name"`
	Version    int               `json:"version"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}
type DetailOutput struct {
	RecordOutput
	Followups []FollowupOutput `json:"followups"`
	Approvals []ApprovalOutput `json:"approvals"`
	Related   []RecordOutput   `json:"related"`
}
type FollowupOutput struct {
	ID        string                  `json:"id"`
	Content   string                  `json:"content"`
	Method    string                  `json:"method"`
	UserName  string                  `json:"user_name"`
	NextDate  *time.Time              `json:"next_date,omitempty"`
	CreatedAt time.Time               `json:"created_at"`
	Comments  []FollowupCommentOutput `json:"comments"`
}
type FollowupCommentOutput struct {
	ID         string    `json:"id"`
	FollowupID string    `json:"followup_id"`
	UserID     string    `json:"user_id"`
	UserName   string    `json:"user_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
type ListFollowupCommentsOutput struct {
	Items []FollowupCommentOutput `json:"items"`
}
type OwnershipHistoryOutput struct {
	ID                string    `json:"id"`
	PreviousOwnerID   string    `json:"previous_owner_id"`
	PreviousOwnerName string    `json:"previous_owner_name"`
	OwnerID           string    `json:"owner_id"`
	OwnerName         string    `json:"owner_name"`
	OperatorID        string    `json:"operator_id"`
	OperatorName      string    `json:"operator_name"`
	Action            string    `json:"action"`
	CreatedAt         time.Time `json:"created_at"`
}
type ListOwnershipHistoryOutput struct {
	Items []OwnershipHistoryOutput `json:"items"`
}
type ListRecordsOutput struct {
	Items []RecordOutput `json:"items"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}
type ExportRecordsInput struct {
	Actor                        UserOutput
	Module, Query, Scope, Status string
}
type ExportRecordsOutput struct {
	Module Module
	Items  []RecordOutput
}
type ImportRecordsInput struct {
	Actor   UserOutput
	Module  string
	Records []map[string]any
}
type ImportRecordsOutput struct {
	Count int `json:"count"`
}
type ImportTemplateOutput struct {
	Module  Module
	Content []byte
}

var errImportPreview = errors.New("import preview complete")

type SearchInput struct {
	Actor UserOutput
	Query string
	Limit int
}
type SearchOutput struct {
	Items []RecordOutput `json:"items"`
	Total int            `json:"total"`
}

type DashboardInput struct{ Actor UserOutput }
type ReportInput struct {
	Actor           UserOutput
	Module, GroupBy string
	Metric, Field   string
}
type ReportRow struct {
	Group string `json:"group"`
	Value string `json:"value"`
}
type ReportOutput struct {
	Rows []ReportRow `json:"rows"`
}
type DashboardActivity struct {
	ID        string    `json:"id"`
	Module    string    `json:"module"`
	Name      string    `json:"name"`
	RecordID  string    `json:"record_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	Method    string    `json:"method"`
	CreatedAt time.Time `json:"created_at"`
}
type DashboardOutput struct {
	Customers        int                 `json:"customers"`
	Leads            int                 `json:"leads"`
	Opportunities    int                 `json:"opportunities"`
	Pipeline         string              `json:"pipeline"`
	WeightedPipeline string              `json:"weighted_pipeline"`
	ContractAmount   string              `json:"contract_amount"`
	ReceivedAmount   string              `json:"received_amount"`
	Receivable       string              `json:"receivable"`
	ConversionRate   float64             `json:"conversion_rate"`
	Funnel           []map[string]any    `json:"funnel"`
	Months           []map[string]string `json:"months"`
	Activities       []any               `json:"activities"`
	Tasks            []RecordOutput      `json:"tasks"`
	OverduePlans     []any               `json:"overdue_plans"`
	Counts           map[string]int      `json:"counts"`
}

func (u *RecordUseCase) Dashboard(ctx context.Context, input DashboardInput) (DashboardOutput, error) {
	if err := requirePermission(input.Actor, "DASHBOARD:READ"); err != nil {
		return DashboardOutput{}, err
	}
	output := DashboardOutput{Pipeline: "0", WeightedPipeline: "0", ContractAmount: "0", ReceivedAmount: "0", Receivable: "0", Funnel: []map[string]any{}, Months: []map[string]string{}, Activities: []any{}, Tasks: []RecordOutput{}, OverduePlans: []any{}, Counts: map[string]int{}}
	pipeline, weighted, contracts, payments := decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero
	stages := []string{"初步沟通", "需求确认", "方案报价", "商务谈判", "赢单", "输单"}
	if opportunityModule, moduleErr := u.moduleWithCustomFields(ctx, "opportunities"); moduleErr == nil {
		for _, field := range opportunityModule.Fields {
			if field.Key == "stage" && len(field.Options) > 0 {
				stages = append([]string(nil), field.Options...)
				break
			}
		}
	}
	funnel := make(map[string]struct {
		count  int
		amount decimal.Decimal
	}, len(stages))
	for _, stage := range stages {
		funnel[stage] = struct {
			count  int
			amount decimal.Decimal
		}{amount: decimal.Zero}
	}
	monthValues := make(map[string]map[string]decimal.Decimal, 6)
	monthStart := time.Now().UTC().AddDate(0, -5, 0)
	for offset := 0; offset < 6; offset++ {
		month := time.Date(monthStart.Year(), monthStart.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		monthValues[month] = map[string]decimal.Decimal{"contracts": decimal.Zero, "payments": decimal.Zero}
	}
	definitions, err := u.enabledModules(ctx)
	if err != nil {
		return DashboardOutput{}, err
	}
	for _, definition := range definitions {
		key := definition.Key
		if !allowed(input.Actor, key, false) {
			continue
		}
		rows, err := u.List(ctx, ListRecordsInput{Actor: input.Actor, Module: key, Page: 1, Size: 100})
		if err != nil {
			return DashboardOutput{}, err
		}
		output.Counts[key] = rows.Total
		switch key {
		case "customers":
			output.Customers = rows.Total
		case "leads":
			output.Leads = rows.Total
		case "opportunities":
			output.Opportunities = rows.Total
		case "tasks":
			for _, task := range rows.Items {
				if fmt.Sprint(task.Data["status"]) != "已完成" {
					output.Tasks = append(output.Tasks, task)
				}
			}
			if len(output.Tasks) > 10 {
				output.Tasks = output.Tasks[:10]
			}
		}
		if definition.Key == "opportunities" {
			for _, item := range rows.Items {
				amount, err := decimal.NewFromString(fmt.Sprint(item.Data["amount"]))
				if err != nil {
					continue
				}
				pipeline = pipeline.Add(amount)
				probability, _ := decimal.NewFromString(fmt.Sprint(item.Data["probability"]))
				weighted = weighted.Add(amount.Mul(probability).Div(decimal.NewFromInt(100)))
				stage := fmt.Sprint(item.Data["stage"])
				if value, ok := funnel[stage]; ok {
					value.count++
					value.amount = value.amount.Add(amount)
					funnel[stage] = value
				}
			}
		}
		if definition.Key == "contracts" || definition.Key == "payments" {
			for _, item := range rows.Items {
				if fmt.Sprint(item.Data["approval_status"]) != string(entity.ApprovalApproved) {
					continue
				}
				amount, err := decimal.NewFromString(fmt.Sprint(item.Data["amount"]))
				if err != nil {
					continue
				}
				dateField, monthKey := "signed_date", "contracts"
				if definition.Key == "payments" {
					dateField, monthKey = "received_date", "payments"
				}
				if definition.Key == "contracts" {
					contracts = contracts.Add(amount)
				} else {
					payments = payments.Add(amount)
				}
				if date, err := time.Parse("2006-01-02", fmt.Sprint(item.Data[dateField])); err == nil {
					if values, ok := monthValues[date.Format("2006-01")]; ok {
						values[monthKey] = values[monthKey].Add(amount)
					}
				}
			}
		}
		if key == "customers" || key == "leads" || key == "opportunities" {
			for _, item := range rows.Items {
				followups, err := u.repo.ListFollowups(ctx, item.ID)
				if err != nil {
					return DashboardOutput{}, err
				}
				for _, followup := range followups {
					user, err := u.repo.FindUserByID(ctx, followup.UserID)
					if err != nil {
						return DashboardOutput{}, err
					}
					output.Activities = append(output.Activities, DashboardActivity{ID: followup.ID, Module: key, Name: fmt.Sprint(item.Data["name"]), RecordID: item.ID, UserName: user.Name, Content: followup.Content, Method: followup.Method, CreatedAt: followup.CreatedAt})
				}
			}
		}
	}
	sort.Slice(output.Activities, func(left, right int) bool {
		return output.Activities[left].(DashboardActivity).CreatedAt.After(output.Activities[right].(DashboardActivity).CreatedAt)
	})
	if len(output.Activities) > 20 {
		output.Activities = output.Activities[:20]
	}
	plans := ListRecordsOutput{Items: []RecordOutput{}}
	if allowed(input.Actor, "payment_plans", false) {
		plans, err = u.List(ctx, ListRecordsInput{Actor: input.Actor, Module: "payment_plans", Page: 1, Size: 100})
	}
	if err != nil {
		return DashboardOutput{}, err
	}
	paymentsForPlans := ListRecordsOutput{Items: []RecordOutput{}}
	if allowed(input.Actor, "payments", false) {
		paymentsForPlans, err = u.List(ctx, ListRecordsInput{Actor: input.Actor, Module: "payments", Page: 1, Size: 100})
	}
	if err != nil {
		return DashboardOutput{}, err
	}
	paidByPlan := map[string]decimal.Decimal{}
	for _, payment := range paymentsForPlans.Items {
		if fmt.Sprint(payment.Data["approval_status"]) != string(entity.ApprovalApproved) {
			continue
		}
		amount, err := decimal.NewFromString(fmt.Sprint(payment.Data["amount"]))
		if err == nil {
			planID := fmt.Sprint(payment.Data["plan_id"])
			paidByPlan[planID] = paidByPlan[planID].Add(amount)
		}
	}
	today := time.Now().UTC().Format("2006-01-02")
	for _, plan := range plans.Items {
		dueDate := fmt.Sprint(plan.Data["due_date"])
		amount, err := decimal.NewFromString(fmt.Sprint(plan.Data["amount"]))
		remaining := amount.Sub(paidByPlan[plan.ID])
		if err == nil && dueDate < today && remaining.IsPositive() {
			output.OverduePlans = append(output.OverduePlans, map[string]any{"id": plan.ID, "name": fmt.Sprint(plan.Data["name"]), "remaining": remaining.StringFixed(2), "due_date": dueDate})
		}
	}
	output.Pipeline, output.WeightedPipeline = pipeline.String(), weighted.String()
	output.ContractAmount, output.ReceivedAmount = contracts.String(), payments.String()
	receivable := contracts.Sub(payments)
	if receivable.IsNegative() {
		receivable = decimal.Zero
	}
	output.Receivable = receivable.String()
	if output.Leads > 0 {
		converted := 0
		leads := ListRecordsOutput{Items: []RecordOutput{}}
		if allowed(input.Actor, "leads", false) {
			leads, err = u.List(ctx, ListRecordsInput{Actor: input.Actor, Module: "leads", Page: 1, Size: 100})
		}
		if err != nil {
			return DashboardOutput{}, err
		}
		for _, lead := range leads.Items {
			if fmt.Sprint(lead.Data["status"]) == "已转化" {
				converted++
			}
		}
		output.ConversionRate = float64(converted) * 100 / float64(output.Leads)
	}
	for _, stage := range stages {
		value := funnel[stage]
		output.Funnel = append(output.Funnel, map[string]any{"stage": stage, "count": value.count, "amount": value.amount.String()})
	}
	for offset := 0; offset < 6; offset++ {
		month := time.Date(monthStart.Year(), monthStart.Month()+time.Month(offset), 1, 0, 0, 0, 0, time.UTC).Format("2006-01")
		values := monthValues[month]
		output.Months = append(output.Months, map[string]string{"month": month, "contracts": values["contracts"].String(), "payments": values["payments"].String()})
	}
	return output, nil
}

func (u *RecordUseCase) Report(ctx context.Context, input ReportInput) (ReportOutput, error) {
	if err := requirePermission(input.Actor, "DASHBOARD:READ"); err != nil {
		return ReportOutput{}, err
	}
	if input.Actor.Role == entity.RoleSales && input.Actor.RoleID == "" {
		return ReportOutput{}, ErrForbidden
	}
	definition, err := u.moduleWithCustomFields(ctx, input.Module)
	if err != nil {
		return ReportOutput{}, err
	}
	groupFound, metricFound := false, input.Metric == "count"
	for _, field := range definition.Fields {
		if field.Key == input.GroupBy && fieldReadable(input.Actor, definition.Key, field.Key) && field.Type != "items" && field.Type != "subtable" {
			groupFound = true
		}
		if input.Metric == "sum" && field.Key == input.Field && fieldReadable(input.Actor, definition.Key, field.Key) && (field.Type == "money" || field.Type == "number" || field.Type == "formula") {
			metricFound = true
		}
	}
	if !groupFound || !metricFound || (input.Metric != "count" && input.Metric != "sum") {
		return ReportOutput{}, ErrValidation
	}
	records, err := u.repo.ListRecords(ctx, definition.Key)
	if err != nil {
		return ReportOutput{}, err
	}
	values := map[string]decimal.Decimal{}
	for _, record := range records {
		if !visible(input.Actor, definition, record, false) {
			continue
		}
		group := strings.TrimSpace(fmt.Sprint(record.Data[input.GroupBy]))
		if group == "" || group == "<nil>" {
			group = "未填写"
		}
		if input.Metric == "count" {
			values[group] = values[group].Add(decimal.NewFromInt(1))
			continue
		}
		value, err := decimal.NewFromString(fmt.Sprint(record.Data[input.Field]))
		if err == nil {
			values[group] = values[group].Add(value)
		}
	}
	groups := make([]string, 0, len(values))
	for group := range values {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	rows := make([]ReportRow, 0, len(groups))
	for _, group := range groups {
		rows = append(rows, ReportRow{Group: group, Value: values[group].StringFixed(2)})
	}
	return ReportOutput{Rows: rows}, nil
}

func (u *RecordUseCase) AddCustomField(ctx context.Context, input AddCustomFieldInput) error {
	if err := requirePermission(input.Actor, "CUSTOM_FORM:ADD"); err != nil {
		return err
	}
	if _, err := u.enabledModule(ctx, input.Module); err != nil {
		return err
	}
	if !regexp.MustCompile(`^custom_[a-z][a-z0-9_]{0,35}$`).MatchString(input.Key) || strings.TrimSpace(input.Label) == "" {
		return ErrValidation
	}
	if !slices.Contains([]string{"text", "textarea", "number", "date", "email", "select", "formula", "subtable"}, input.Type) {
		return ErrValidation
	}
	existing, err := u.repo.ListCustomFields(ctx, input.Module)
	if err != nil {
		return err
	}
	activeCount := 0
	for _, field := range existing {
		if field.Definition["archived"] != true {
			activeCount++
		}
	}
	if activeCount >= 60 {
		return entity.NewBusinessError("validation", "每个模块最多 60 个字段")
	}
	for _, field := range existing {
		if field.Key == input.Key {
			return entity.NewBusinessError("conflict", "字段标识已存在")
		}
	}
	if input.Type == "select" && len(input.Options) == 0 {
		return ErrValidation
	}
	if input.Type == "formula" {
		if len(input.Sources) == 0 {
			return ErrValidation
		}
		definition, err := u.moduleWithCustomFields(ctx, input.Module)
		if err != nil {
			return err
		}
		valid := map[string]bool{}
		for _, field := range definition.Fields {
			if field.Type == "number" || field.Type == "money" {
				valid[field.Key] = true
			}
		}
		seen := map[string]bool{}
		for _, source := range input.Sources {
			if !valid[source] || seen[source] {
				return ErrValidation
			}
			seen[source] = true
		}
	}
	if input.Type == "subtable" && len(input.Columns) == 0 {
		return ErrValidation
	}
	if input.Type == "subtable" {
		seen := map[string]bool{}
		for _, column := range input.Columns {
			if !regexp.MustCompile(`^[a-z][a-z0-9_]{0,35}$`).MatchString(column.Key) || strings.TrimSpace(column.Label) == "" || seen[column.Key] || !slices.Contains([]string{"text", "number", "money", "date", "select"}, column.Type) || (column.Type == "select" && len(column.Options) == 0) {
				return ErrValidation
			}
			seen[column.Key] = true
		}
	}
	return u.repo.CreateCustomField(ctx, entity.CustomField{Module: input.Module, Key: input.Key, Definition: map[string]any{"key": input.Key, "label": strings.TrimSpace(input.Label), "type": input.Type, "options": input.Options, "sources": input.Sources, "columns": input.Columns, "required": false}})
}
func (u *RecordUseCase) RenameCustomField(ctx context.Context, actor UserOutput, moduleKey, key, label string) error {
	if !strings.HasPrefix(key, "custom_") {
		return u.RenameStandardField(ctx, actor, moduleKey, key, label)
	}
	if err := requirePermission(actor, "CUSTOM_FORM:UPDATE"); err != nil {
		return err
	}
	if _, err := u.enabledModule(ctx, moduleKey); err != nil {
		return err
	}
	label = strings.TrimSpace(label)
	if !regexp.MustCompile(`^custom_[a-z][a-z0-9_]{0,35}$`).MatchString(key) || label == "" || len([]rune(label)) > 40 {
		return ErrValidation
	}
	fields, err := u.repo.ListCustomFields(ctx, moduleKey)
	if err != nil {
		return err
	}
	for _, field := range fields {
		if field.Key == key && field.Definition["archived"] != true {
			field.Definition["label"] = label
			return u.repo.UpdateCustomField(ctx, field)
		}
	}
	return ErrNotFound
}
func (u *RecordUseCase) DeleteCustomField(ctx context.Context, actor UserOutput, moduleKey, key string) error {
	if !strings.HasPrefix(key, "custom_") {
		return u.DisableStandardField(ctx, actor, moduleKey, key)
	}
	if err := requirePermission(actor, "CUSTOM_FORM:DELETE"); err != nil {
		return err
	}
	if _, err := u.enabledModule(ctx, moduleKey); err != nil {
		return err
	}
	if !regexp.MustCompile(`^custom_[a-z][a-z0-9_]{0,35}$`).MatchString(key) {
		return ErrValidation
	}
	fields, err := u.repo.ListCustomFields(ctx, moduleKey)
	if err != nil {
		return err
	}
	var target *entity.CustomField
	for index := range fields {
		if fields[index].Key == key && fields[index].Definition["archived"] != true {
			target = &fields[index]
		}
	}
	if target == nil {
		return ErrNotFound
	}
	for index := range fields {
		if fields[index].Definition["archived"] == true || fields[index].Definition["type"] != "formula" {
			continue
		}
		switch sources := fields[index].Definition["sources"].(type) {
		case []any:
			for _, source := range sources {
				if source == key {
					return entity.NewBusinessError("conflict", "字段被公式引用，请先处理公式字段")
				}
			}
		case []string:
			if slices.Contains(sources, key) {
				return entity.NewBusinessError("conflict", "字段被公式引用，请先处理公式字段")
			}
		}
	}
	flows, err := u.repo.ListApprovalFlows(ctx, moduleKey)
	if err != nil {
		return err
	}
	for _, flow := range flows {
		version, err := u.repo.FindCurrentApprovalFlowVersion(ctx, flow.ID)
		if err != nil {
			return err
		}
		for _, step := range version.Steps {
			for _, condition := range step.Conditions {
				if condition.Field == key {
					return entity.NewBusinessError("conflict", "字段被审批流程引用，请先调整审批条件")
				}
			}
		}
	}
	target.Definition["archived"] = true
	return u.repo.UpdateCustomField(ctx, *target)
}

func standardFieldCanDisable(moduleKey string, field Field) bool {
	if field.Required || field.ReadOnly || field.Type == "items" || field.Type == "formula" || field.Type == "subtable" {
		return false
	}
	switch moduleKey + ":" + field.Key {
	case "leads:phone", "leads:notes", "contracts:quote_id", "payments:plan_id":
		return false
	}
	return true
}

func (u *RecordUseCase) RenameStandardField(ctx context.Context, actor UserOutput, moduleKey, key, label string) error {
	if err := requirePermission(actor, "CUSTOM_FORM:UPDATE"); err != nil {
		return err
	}
	definition, err := u.enabledModule(ctx, moduleKey)
	if err != nil {
		return err
	}
	if _, builtIn := Modules[moduleKey]; !builtIn {
		return ErrNotFound
	}
	label = strings.TrimSpace(label)
	if label == "" || len([]rune(label)) > 40 {
		return ErrValidation
	}
	found := false
	for _, field := range definition.Fields {
		if field.Key == key {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	settings, err := u.repo.ListStandardFieldSettings(ctx, moduleKey)
	if err != nil {
		return err
	}
	setting := entity.StandardFieldSetting{Module: moduleKey, Key: key}
	for _, current := range settings {
		if current.Key == key {
			setting = current
			break
		}
	}
	if setting.Archived {
		return ErrNotFound
	}
	setting.Label = label
	return u.repo.SaveStandardFieldSetting(ctx, setting)
}

func (u *RecordUseCase) DisableStandardField(ctx context.Context, actor UserOutput, moduleKey, key string) error {
	if err := requirePermission(actor, "CUSTOM_FORM:DELETE"); err != nil {
		return err
	}
	definition, err := u.enabledModule(ctx, moduleKey)
	if err != nil {
		return err
	}
	if _, builtIn := Modules[moduleKey]; !builtIn {
		return ErrNotFound
	}
	var target *Field
	for index := range definition.Fields {
		if definition.Fields[index].Key == key {
			target = &definition.Fields[index]
			break
		}
	}
	if target == nil {
		return ErrNotFound
	}
	if !standardFieldCanDisable(moduleKey, *target) {
		return entity.NewBusinessError("conflict", "该标准字段参与必填或业务处理，不能停用")
	}
	settings, err := u.repo.ListStandardFieldSettings(ctx, moduleKey)
	if err != nil {
		return err
	}
	setting := entity.StandardFieldSetting{Module: moduleKey, Key: key}
	for _, current := range settings {
		if current.Key == key {
			setting = current
			break
		}
	}
	if setting.Archived {
		return ErrNotFound
	}
	customFields, err := u.repo.ListCustomFields(ctx, moduleKey)
	if err != nil {
		return err
	}
	for _, field := range customFields {
		if field.Definition["archived"] == true || field.Definition["type"] != "formula" {
			continue
		}
		for _, source := range fieldSources(field.Definition["sources"]) {
			if source == key {
				return entity.NewBusinessError("conflict", "字段被公式引用，请先处理公式字段")
			}
		}
	}
	flows, err := u.repo.ListApprovalFlows(ctx, moduleKey)
	if err != nil {
		return err
	}
	for _, flow := range flows {
		version, err := u.repo.FindCurrentApprovalFlowVersion(ctx, flow.ID)
		if err != nil {
			return err
		}
		for _, step := range version.Steps {
			for _, condition := range step.Conditions {
				if condition.Field == key {
					return entity.NewBusinessError("conflict", "字段被审批流程引用，请先调整审批条件")
				}
			}
		}
	}
	modules, err := u.modules.ListModules(ctx)
	if err != nil {
		return err
	}
	for _, module := range modules {
		if module.Key == moduleKey && regexp.MustCompile(`\{\{\s*`+regexp.QuoteMeta(key)+`\s*\}\}`).MatchString(module.DocumentTemplate) {
			return entity.NewBusinessError("conflict", "字段被文档模板引用，请先调整模板")
		}
	}
	setting.Archived = true
	return u.repo.SaveStandardFieldSetting(ctx, setting)
}

func fieldSources(value any) []string {
	switch values := value.(type) {
	case []string:
		return values
	case []any:
		result := make([]string, len(values))
		for index, item := range values {
			result[index] = fmt.Sprint(item)
		}
		return result
	}
	return nil
}

func (u *RecordUseCase) Catalog(ctx context.Context, actor UserOutput) ([]Module, error) {
	enabled, err := u.enabledModuleKeys(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Module, 0, len(enabled))
	for _, key := range enabled {
		if !HasModulePermission(actor, key, "READ", false) && !HasModulePermission(actor, key, "READ", true) {
			continue
		}
		item, err := u.moduleWithCustomFields(ctx, key)
		if err != nil {
			return nil, err
		}
		result = append(result, restrictedModule(actor, item))
	}
	return result, nil
}

func restrictedModule(actor UserOutput, module Module) Module {
	if actor.RoleID == "" || actor.FieldRestrictions == nil {
		return module
	}
	fields := make([]Field, 0, len(module.Fields))
	for _, field := range module.Fields {
		if fieldReadable(actor, module.Key, field.Key) {
			field.ReadOnly = !fieldWritable(actor, module.Key, field.Key)
			fields = append(fields, field)
		}
	}
	module.Fields = fields
	return module
}

func fieldReadable(actor UserOutput, moduleKey, fieldKey string) bool {
	if actor.RoleID == "" {
		return true
	}
	for _, key := range actor.FieldRestrictions[moduleKey].Hidden {
		if key == fieldKey {
			return false
		}
	}
	return true
}

func fieldWritable(actor UserOutput, moduleKey, fieldKey string) bool {
	if !fieldReadable(actor, moduleKey, fieldKey) {
		return false
	}
	for _, key := range actor.FieldRestrictions[moduleKey].ReadOnly {
		if key == fieldKey {
			return false
		}
	}
	return true
}

func filteredData(actor UserOutput, moduleKey string, data map[string]any) map[string]any {
	filtered := make(map[string]any, len(data))
	for key, value := range data {
		if fieldReadable(actor, moduleKey, key) {
			filtered[key] = value
		}
	}
	return filtered
}

func validateFieldWrite(actor UserOutput, module Module, data map[string]any) error {
	fields := make(map[string]Field, len(module.Fields))
	for _, field := range module.Fields {
		fields[field.Key] = field
	}
	for key := range data {
		field, found := fields[key]
		if found && (field.Type == "formula" || field.ReadOnly || (actor.RoleID != "" && !fieldWritable(actor, module.Key, key))) {
			return ErrForbidden
		}
	}
	return nil
}

func (u *RecordUseCase) moduleWithCustomFields(ctx context.Context, key string) (Module, error) {
	item, err := u.enabledModule(ctx, key)
	if err != nil {
		return Module{}, err
	}
	settings, err := u.repo.ListStandardFieldSettings(ctx, item.Key)
	if err != nil {
		return Module{}, err
	}
	byKey := make(map[string]entity.StandardFieldSetting, len(settings))
	for _, setting := range settings {
		byKey[setting.Key] = setting
	}
	visible := make([]Field, 0, len(item.Fields))
	_, builtIn := Modules[item.Key]
	for _, field := range item.Fields {
		if setting, found := byKey[field.Key]; found {
			if setting.Archived {
				continue
			}
			if setting.Label != "" {
				field.Label = setting.Label
			}
		}
		field.Standard = builtIn
		field.Disableable = builtIn && standardFieldCanDisable(item.Key, field)
		visible = append(visible, field)
	}
	item.Fields = visible
	fields, err := u.repo.ListCustomFields(ctx, item.Key)
	if err != nil {
		return Module{}, err
	}
	for _, field := range fields {
		if field.Definition["archived"] == true {
			continue
		}
		options := make([]string, 0)
		switch values := field.Definition["options"].(type) {
		case []any:
			for _, value := range values {
				options = append(options, fmt.Sprint(value))
			}
		case []string:
			options = append(options, values...)
		}
		sources := []string{}
		if values, ok := field.Definition["sources"].([]any); ok {
			for _, value := range values {
				sources = append(sources, fmt.Sprint(value))
			}
		}
		columns := []Field{}
		if raw, err := json.Marshal(field.Definition["columns"]); err == nil {
			_ = json.Unmarshal(raw, &columns)
		}
		item.Fields = append(item.Fields, Field{Key: field.Key, Label: fmt.Sprint(field.Definition["label"]), Type: fmt.Sprint(field.Definition["type"]), Required: false, Custom: true, Options: options, Sources: sources, Columns: columns})
	}
	definitions, err := u.modules.ListModules(ctx)
	if err != nil {
		return Module{}, err
	}
	for _, definition := range definitions {
		if definition.Key == key {
			item = applyModuleLayout(item, definition.Layout)
			item.DocumentTemplate = definition.DocumentTemplate
			if (key == "opportunities" || key == "contracts") && (len(definition.Stages) > 0 || len(definition.SalesStages) > 0) {
				stageNames := definition.SalesStages
				if len(definition.Stages) > 0 {
					stageNames = make([]string, len(definition.Stages))
					for index, stage := range definition.Stages {
						stageNames[index] = stage.Name
					}
				}
				for index := range item.Fields {
					if item.Fields[index].Key == "stage" {
						item.Fields[index].Options = append([]string(nil), stageNames...)
					}
				}
			}
			return item, nil
		}
	}
	return item, nil
}

func (u *RecordUseCase) validateStageTransition(ctx context.Context, moduleKey, origin, target string) error {
	if origin == target || strings.TrimSpace(target) == "" {
		return ErrValidation
	}
	definitions, err := u.modules.ListModules(ctx)
	if err != nil {
		return err
	}
	for _, definition := range definitions {
		if definition.Key != moduleKey {
			continue
		}
		if len(definition.Stages) == 0 {
			validStages := definition.SalesStages
			if len(validStages) == 0 {
				if fallback, found := Modules[moduleKey]; found {
					for _, field := range fallback.Fields {
						if field.Key == "stage" {
							validStages = field.Options
							break
						}
					}
				}
			}
			if slices.Contains(validStages, target) {
				return nil
			}
			return ErrValidation
		}
		positions := make(map[string]int, len(definition.Stages))
		types := make(map[string]string, len(definition.Stages))
		for index, stage := range definition.Stages {
			positions[stage.Name], types[stage.Name] = index, stage.Type
		}
		originPosition, originFound := positions[origin]
		targetPosition, targetFound := positions[target]
		if !originFound || !targetFound {
			return ErrValidation
		}
		if definition.CirculationType == "ADVANCED" {
			for _, transition := range definition.StageTransitions {
				if transition.Origin == origin && transition.Target == target {
					return nil
				}
			}
			return entity.NewBusinessError("conflict", "当前阶段不允许流转到目标阶段")
		}
		if types[origin] == "END" && !definition.EndRollback {
			return entity.NewBusinessError("conflict", "完结阶段不允许回退")
		}
		if types[origin] == "AFOOT" && types[target] == "AFOOT" && targetPosition < originPosition && !definition.AfootRollback {
			return entity.NewBusinessError("conflict", "进行中阶段不允许回退")
		}
		return nil
	}
	return ErrValidation
}

func applyModuleLayout(module Module, layout []entity.FieldLayout) Module {
	if len(layout) == 0 {
		return module
	}
	positions := make(map[string]entity.FieldLayout, len(layout))
	for _, field := range layout {
		positions[field.Key] = field
	}
	sort.SliceStable(module.Fields, func(left, right int) bool {
		leftLayout, leftFound := positions[module.Fields[left].Key]
		rightLayout, rightFound := positions[module.Fields[right].Key]
		if leftFound && rightFound {
			return leftLayout.Sort < rightLayout.Sort
		}
		return leftFound
	})
	for index := range module.Fields {
		if field, found := positions[module.Fields[index].Key]; found {
			module.Fields[index].Group = field.Group
		}
	}
	return module
}

func (u *RecordUseCase) enabledModule(ctx context.Context, key string) (Module, error) {
	definitions, err := u.modules.ListModules(ctx)
	if err != nil {
		return Module{}, err
	}
	for _, definition := range definitions {
		if definition.Key != key {
			continue
		}
		if !definition.Enabled {
			return Module{}, entity.NewBusinessError("not_found", "模块已停用")
		}
		if item, found := Modules[key]; found {
			return item, nil
		}
		if definition.Custom {
			return moduleFromDefinition(definition), nil
		}
	}
	return Module{}, entity.NewBusinessError("not_found", "模块不存在")
}

func (u *RecordUseCase) enabledModuleKeys(ctx context.Context) ([]string, error) {
	definitions, err := u.modules.ListModules(ctx)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		if definition.Enabled && (definition.Custom || Modules[definition.Key].Key != "") {
			keys = append(keys, definition.Key)
		}
	}
	sort.Strings(keys)
	return keys, nil
}

func (u *RecordUseCase) enabledModules(ctx context.Context) ([]Module, error) {
	keys, err := u.enabledModuleKeys(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Module, 0, len(keys))
	for _, key := range keys {
		item, err := u.enabledModule(ctx, key)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func moduleFromDefinition(definition entity.ModuleDefinition) Module {
	fields := make([]Field, len(definition.Fields))
	for index, field := range definition.Fields {
		fields[index] = Field{Key: field.Key, Label: field.Name, Type: field.Type, Required: field.Required, Unique: field.Unique, ReadOnly: field.ReadOnly, Options: append([]string(nil), field.Options...)}
	}
	group := definition.Group
	if group == "" {
		group = "自定义业务"
	}
	return Module{Key: definition.Key, Label: definition.Name, Group: group, Description: definition.Description, Shared: definition.Shared, Approvable: definition.Approvable, Fields: fields, DocumentTemplate: definition.DocumentTemplate}
}

func module(key string) (Module, error) {
	value, ok := Modules[key]
	if !ok {
		return Module{}, entity.NewBusinessError("not_found", "模块不存在")
	}
	return value, nil
}
func isManager(actor UserOutput) bool {
	return actor.Role == entity.RoleAdmin || actor.Role == entity.RoleManager
}
func allowed(actor UserOutput, moduleKey string, write bool) bool {
	action := "READ"
	if write {
		action = "UPDATE"
	}
	return HasModulePermission(actor, moduleKey, action, false)
}
func visible(actor UserOutput, m Module, r entity.Record, write bool) bool {
	if !allowed(actor, m.Key, write) {
		return false
	}
	return withinScope(actor, m, r, write)
}
func withinScope(actor UserOutput, m Module, r entity.Record, write bool) bool {
	if actor.RoleID != "" && (actor.OrganizationID == "" || r.OrganizationID != actor.OrganizationID) {
		return false
	}
	if actor.RoleID == "" && (actor.Role != entity.RoleSales || m.Shared) {
		return true
	}
	if m.Shared {
		return true
	}
	if actor.RoleID != "" {
		if r.OwnerID == nil {
			return !write
		}
		scope := actor.DataScope
		if scope == "" {
			scope = "self"
		}
		switch scope {
		case "all":
			return true
		case "department":
			return actor.Department != "" && r.OwnerDepartment == actor.Department
		case "department_and_children", "specified_departments":
			return slices.Contains(actor.DepartmentIDs, r.OwnerDepartment)
		default:
			return *r.OwnerID == actor.ID
		}
	}
	if write {
		return r.OwnerID != nil && *r.OwnerID == actor.ID
	}
	return r.OwnerID == nil || *r.OwnerID == actor.ID
}
func withinPoolScope(actor UserOutput, record entity.Record) bool {
	return actor.RoleID == "" || (actor.OrganizationID != "" && record.OrganizationID == actor.OrganizationID)
}
func (u *RecordUseCase) validateReferences(ctx context.Context, actor UserOutput, definition Module, data map[string]any) error {
	if actor.RoleID == "" {
		return nil
	}
	for _, field := range definition.Fields {
		if field.Type != "reference" {
			continue
		}
		id, _ := data[field.Key].(string)
		if id == "" {
			continue
		}
		referenced, err := u.repo.FindRecord(ctx, field.Module, id)
		if err != nil || !visible(actor, Modules[field.Module], referenced, false) {
			return ErrForbidden
		}
	}
	if items, ok := data["items"].([]any); ok {
		for _, value := range items {
			item, ok := value.(map[string]any)
			if !ok {
				return ErrValidation
			}
			productID, _ := item["product_id"].(string)
			product, err := u.repo.FindRecord(ctx, "products", productID)
			if err != nil || !visible(actor, Modules["products"], product, false) {
				return ErrForbidden
			}
		}
	}
	return nil
}
func (u *RecordUseCase) output(ctx context.Context, actor UserOutput, record entity.Record) (RecordOutput, error) {
	name := "公海"
	if record.OwnerID != nil {
		user, err := u.repo.FindUserByID(ctx, *record.OwnerID)
		if err == nil {
			name = user.Name
		}
	}
	references := map[string]string{}
	if definition, err := module(record.Module); err == nil {
		for _, field := range definition.Fields {
			if field.Type != "reference" || !fieldReadable(actor, record.Module, field.Key) {
				continue
			}
			id := strings.TrimSpace(fmt.Sprint(record.Data[field.Key]))
			if id == "" {
				continue
			}
			if target, err := u.repo.FindRecord(ctx, field.Module, id); err == nil && target.OrganizationID == record.OrganizationID && visible(actor, Modules[field.Module], target, false) {
				references[field.Key] = fmt.Sprint(target.Data["name"])
			}
		}
	}
	return RecordOutput{ID: record.ID, Module: record.Module, Data: filteredData(actor, record.Module, record.Data), References: references, OwnerID: record.OwnerID, OwnerName: name, Version: record.Version, CreatedAt: record.CreatedAt, UpdatedAt: record.UpdatedAt}, nil
}
func validate(module Module, data map[string]any) (map[string]any, error) {
	if data == nil {
		return nil, ErrValidation
	}
	allowed := map[string]Field{}
	for _, field := range module.Fields {
		allowed[field.Key] = field
	}
	for key := range data {
		field, ok := allowed[key]
		if !ok || field.Type == "formula" {
			return nil, entity.NewBusinessError("validation", "包含未知或只读字段："+key)
		}
	}
	clean := make(map[string]any, len(module.Fields))
	for _, field := range module.Fields {
		if field.Type == "formula" {
			continue
		}
		value := data[field.Key]
		if value == nil || value == "" {
			if field.Required {
				return nil, entity.NewBusinessError("validation", field.Label+"不能为空")
			}
			clean[field.Key] = ""
			continue
		}
		switch field.Type {
		case "money":
			number, err := decimal.NewFromString(fmt.Sprint(value))
			if err != nil || number.IsNegative() || number.GreaterThan(decimal.RequireFromString("999999999999.99")) {
				return nil, entity.NewBusinessError("validation", field.Label+"必须是有效的非负金额，最多两位小数")
			}
			clean[field.Key] = number.StringFixed(2)
		case "number":
			number, err := decimal.NewFromString(fmt.Sprint(value))
			if err != nil {
				return nil, entity.NewBusinessError("validation", field.Label+"格式错误")
			}
			minimum, maximum := decimal.Zero, decimal.NewFromInt(100)
			if field.Min != nil {
				minimum = decimal.NewFromFloat(*field.Min)
			}
			if field.Max != nil {
				maximum = decimal.NewFromFloat(*field.Max)
			}
			if number.LessThan(minimum) || number.GreaterThan(maximum) {
				return nil, entity.NewBusinessError("validation", field.Label+"超出允许范围")
			}
			clean[field.Key], _ = number.Float64()
		case "select":
			text, ok := value.(string)
			if !ok || !slices.Contains(field.Options, text) {
				return nil, entity.NewBusinessError("validation", field.Label+"选项无效")
			}
			clean[field.Key] = text
		case "date":
			text, ok := value.(string)
			if !ok {
				return nil, ErrValidation
			}
			if _, err := time.Parse("2006-01-02", text); err != nil {
				return nil, entity.NewBusinessError("validation", field.Label+"日期格式应为 YYYY-MM-DD")
			}
			clean[field.Key] = text
		case "email":
			text, ok := value.(string)
			if !ok || !regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`).MatchString(text) {
				return nil, entity.NewBusinessError("validation", "邮箱格式无效")
			}
			clean[field.Key] = strings.TrimSpace(text)
		case "items":
			items, ok := value.([]any)
			if !ok || len(items) == 0 || len(items) > 100 {
				return nil, entity.NewBusinessError("validation", "请添加 1–100 条产品明细")
			}
			normalized, amount, err := normalizeLineItems(items)
			if err != nil {
				return nil, err
			}
			clean[field.Key], clean["amount"] = normalized, amount.StringFixed(2)
		case "subtable":
			rows, ok := value.([]any)
			if !ok || len(rows) > 100 {
				return nil, entity.NewBusinessError("validation", field.Label+"格式错误")
			}
			normalized := make([]any, 0, len(rows))
			for _, row := range rows {
				values, ok := row.(map[string]any)
				if !ok {
					return nil, entity.NewBusinessError("validation", field.Label+"行数据格式错误")
				}
				cleanRow, err := validate(Module{Fields: field.Columns}, values)
				if err != nil {
					return nil, err
				}
				normalized = append(normalized, cleanRow)
			}
			clean[field.Key] = normalized
		default:
			text, ok := value.(string)
			if !ok {
				return nil, entity.NewBusinessError("validation", field.Label+"格式错误")
			}
			text = strings.TrimSpace(text)
			if field.Required && text == "" {
				return nil, entity.NewBusinessError("validation", field.Label+"不能为空")
			}
			clean[field.Key] = text
		}
	}
	for _, field := range module.Fields {
		if field.Type != "formula" {
			continue
		}
		total := decimal.Zero
		for _, source := range field.Sources {
			value, found := clean[source]
			if !found || value == "" {
				return nil, entity.NewBusinessError("validation", field.Label+"的来源字段不能为空")
			}
			number, err := decimal.NewFromString(fmt.Sprint(value))
			if err != nil {
				return nil, entity.NewBusinessError("validation", field.Label+"的来源字段必须是数值")
			}
			total = total.Add(number)
		}
		clean[field.Key] = total.StringFixed(2)
	}
	if module.Approvable {
		clean["approval_status"] = string(entity.ApprovalDraft)
	}
	if module.Key == "products" && (clean["reserved_stock"] == nil || clean["reserved_stock"] == "") {
		clean["reserved_stock"] = float64(0)
	}
	if module.Key == "opportunities" {
		if stage := fmt.Sprint(clean["stage"]); stage == "赢单" {
			clean["probability"] = float64(100)
		} else if stage == "输单" {
			clean["probability"] = float64(0)
		}
	}
	return clean, nil
}

func normalizeLineItems(items []any) ([]any, decimal.Decimal, error) {
	normalized := make([]any, 0, len(items))
	total := decimal.Zero
	maximum := decimal.RequireFromString("999999999999.99")
	for _, value := range items {
		item, ok := value.(map[string]any)
		if !ok {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "产品明细格式错误")
		}
		productID, ok := item["product_id"].(string)
		if !ok || strings.TrimSpace(productID) == "" {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "产品明细必须选择产品")
		}
		quantity, err := decimal.NewFromString(fmt.Sprint(item["quantity"]))
		if err != nil || !quantity.IsPositive() || quantity.GreaterThan(maximum) {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "产品数量必须是有效的正数")
		}
		priceValue, exists := item["unit_price"]
		if !exists {
			priceValue = item["price"]
		}
		unitPrice, err := decimal.NewFromString(fmt.Sprint(priceValue))
		if err != nil || !unitPrice.IsPositive() || unitPrice.GreaterThan(maximum) {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "产品单价必须是有效的正数")
		}
		discountRate, err := optionalRate(item["discount_rate"])
		if err != nil {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "折扣率必须是 0–100 的数字")
		}
		taxRate, err := optionalRate(item["tax_rate"])
		if err != nil {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "税率必须是 0–100 的数字")
		}
		baseAmount := quantity.Mul(unitPrice).Round(2)
		discountAmount := baseAmount.Mul(discountRate).Div(decimal.NewFromInt(100)).Round(2)
		preTaxAmount := baseAmount.Sub(discountAmount)
		taxAmount := preTaxAmount.Mul(taxRate).Div(decimal.NewFromInt(100)).Round(2)
		subtotal := preTaxAmount.Add(taxAmount).Round(2)
		total = total.Add(subtotal)
		if total.GreaterThan(maximum) {
			return nil, decimal.Zero, entity.NewBusinessError("validation", "产品明细合计金额超出允许范围")
		}
		clean := map[string]any{
			"product_id":      strings.TrimSpace(productID),
			"quantity":        quantity.String(),
			"unit_price":      unitPrice.StringFixed(2),
			"discount_rate":   discountRate.StringFixed(2),
			"discount_amount": discountAmount.StringFixed(2),
			"tax_rate":        taxRate.StringFixed(2),
			"tax_amount":      taxAmount.StringFixed(2),
			"pre_tax_amount":  preTaxAmount.StringFixed(2),
			"subtotal":        subtotal.StringFixed(2),
		}
		if name, ok := item["name"].(string); ok && strings.TrimSpace(name) != "" {
			clean["name"] = strings.TrimSpace(name)
		}
		if specification, ok := item["specification"].(string); ok && strings.TrimSpace(specification) != "" {
			clean["specification"] = strings.TrimSpace(specification)
		}
		normalized = append(normalized, clean)
	}
	return normalized, total, nil
}

func optionalRate(value any) (decimal.Decimal, error) {
	if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
		return decimal.Zero, nil
	}
	rate, err := decimal.NewFromString(fmt.Sprint(value))
	if err != nil || rate.IsNegative() || rate.GreaterThan(decimal.NewFromInt(100)) {
		return decimal.Zero, ErrValidation
	}
	return rate, nil
}

func lineItemQuantities(value any) (map[string]decimal.Decimal, error) {
	quantities := map[string]decimal.Decimal{}
	if value == nil {
		return quantities, nil
	}
	items, ok := value.([]any)
	if !ok {
		return nil, ErrValidation
	}
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, ErrValidation
		}
		productID := strings.TrimSpace(fmt.Sprint(item["product_id"]))
		quantity, err := decimal.NewFromString(fmt.Sprint(item["quantity"]))
		if productID == "" || err != nil || !quantity.IsPositive() {
			return nil, ErrValidation
		}
		quantities[productID] = quantities[productID].Add(quantity)
	}
	return quantities, nil
}

func (u *RecordUseCase) adjustOrderInventory(ctx context.Context, previous, current any, delivered bool) error {
	previousQuantities, err := lineItemQuantities(previous)
	if err != nil {
		return err
	}
	currentQuantities, err := lineItemQuantities(current)
	if err != nil {
		return err
	}
	productIDs := map[string]bool{}
	for productID := range previousQuantities {
		productIDs[productID] = true
	}
	for productID := range currentQuantities {
		productIDs[productID] = true
	}
	for productID := range productIDs {
		product, err := u.repo.FindRecord(ctx, "products", productID)
		if err != nil {
			return entity.NewBusinessError("validation", "订单产品不存在")
		}
		available, err := decimal.NewFromString(fmt.Sprint(product.Data["stock"]))
		if err != nil {
			available = decimal.Zero
		}
		reserved, err := decimal.NewFromString(fmt.Sprint(product.Data["reserved_stock"]))
		if err != nil {
			reserved = decimal.Zero
		}
		if delivered {
			quantity := previousQuantities[productID]
			if reserved.GreaterThanOrEqual(quantity) {
				reserved = reserved.Sub(quantity)
			} else if available.GreaterThanOrEqual(quantity) {
				available = available.Sub(quantity)
			} else {
				return entity.NewBusinessError("conflict", "产品库存不足，无法完成交付")
			}
		} else {
			delta := currentQuantities[productID].Sub(previousQuantities[productID])
			if delta.IsPositive() && available.LessThan(delta) {
				return entity.NewBusinessError("conflict", "产品可用库存不足")
			}
			available = available.Sub(delta)
			reserved = reserved.Add(delta)
			if reserved.IsNegative() {
				return entity.NewBusinessError("conflict", "产品预占库存异常")
			}
		}
		product.Data["stock"], _ = available.Float64()
		product.Data["reserved_stock"], _ = reserved.Float64()
		product.UpdatedAt = time.Now().UTC()
		if err := u.repo.UpdateRecord(ctx, product, product.Version); err != nil {
			return err
		}
	}
	return nil
}

func (u *RecordUseCase) List(ctx context.Context, input ListRecordsInput) (ListRecordsOutput, error) {
	_, rows, err := u.filteredRecords(ctx, input.Actor, input.Module, input.Query, input.Scope, input.Status)
	if err != nil {
		return ListRecordsOutput{}, err
	}
	items := u.recordOutputs(ctx, input.Actor, rows)
	total := len(items)
	if input.Page < 1 {
		input.Page = 1
	}
	if input.Size < 1 || input.Size > 100 {
		input.Size = 20
	}
	start := (input.Page - 1) * input.Size
	if start >= total {
		return ListRecordsOutput{Items: []RecordOutput{}, Total: total, Page: input.Page, Size: input.Size}, nil
	}
	end := min(start+input.Size, total)
	return ListRecordsOutput{Items: items[start:end], Total: total, Page: input.Page, Size: input.Size}, nil
}

func (u *RecordUseCase) Export(ctx context.Context, input ExportRecordsInput) (ExportRecordsOutput, error) {
	if !HasModulePermission(input.Actor, input.Module, "EXPORT", input.Scope == "pool") {
		return ExportRecordsOutput{}, ErrForbidden
	}
	m, rows, err := u.filteredRecords(ctx, input.Actor, input.Module, input.Query, input.Scope, input.Status)
	if err != nil {
		return ExportRecordsOutput{}, err
	}
	return ExportRecordsOutput{Module: m, Items: u.recordOutputs(ctx, input.Actor, rows)}, nil
}

func (u *RecordUseCase) filteredRecords(ctx context.Context, actor UserOutput, moduleKey, query, scope, status string) (Module, []entity.Record, error) {
	m, err := u.moduleWithCustomFields(ctx, moduleKey)
	if err != nil {
		return Module{}, nil, err
	}
	if !HasModulePermission(actor, m.Key, "READ", scope == "pool") {
		return Module{}, nil, ErrForbidden
	}
	rows, err := u.repo.ListRecords(ctx, m.Key)
	if err != nil {
		return Module{}, nil, err
	}
	items := make([]entity.Record, 0, len(rows))
	for _, row := range rows {
		poolModule := m.Key == "customers" || m.Key == "leads"
		if !withinScope(actor, m, row, false) || (scope == "mine" && (row.OwnerID == nil || *row.OwnerID != actor.ID)) || (scope == "pool" && (row.OwnerID != nil || !poolModule)) || (scope != "pool" && poolModule && row.OwnerID == nil) {
			continue
		}
		if query != "" && !strings.Contains(strings.ToLower(fmt.Sprint(filteredData(actor, m.Key, row.Data))), strings.ToLower(query)) {
			continue
		}
		visibleData := filteredData(actor, m.Key, row.Data)
		if status != "" && fmt.Sprint(visibleData["status"]) != status && fmt.Sprint(visibleData["stage"]) != status && fmt.Sprint(visibleData["approval_status"]) != status {
			continue
		}
		items = append(items, row)
	}
	return m, items, nil
}

func (u *RecordUseCase) recordOutputs(ctx context.Context, actor UserOutput, rows []entity.Record) []RecordOutput {
	items := make([]RecordOutput, 0, len(rows))
	for _, row := range rows {
		out, _ := u.output(ctx, actor, row)
		items = append(items, out)
	}
	return items
}
func (u *RecordUseCase) Search(ctx context.Context, input SearchInput) (SearchOutput, error) {
	query := strings.TrimSpace(input.Query)
	if query == "" {
		return SearchOutput{}, ErrValidation
	}
	if input.Limit < 1 || input.Limit > 100 {
		input.Limit = 20
	}
	items := make([]RecordOutput, 0, input.Limit)
	keys, err := u.enabledModuleKeys(ctx)
	if err != nil {
		return SearchOutput{}, err
	}
	for _, key := range keys {
		if !allowed(input.Actor, key, false) {
			continue
		}
		result, err := u.List(ctx, ListRecordsInput{Actor: input.Actor, Module: key, Query: query, Page: 1, Size: input.Limit})
		if err != nil {
			return SearchOutput{}, err
		}
		items = append(items, result.Items...)
		if len(items) >= input.Limit {
			return SearchOutput{Items: items[:input.Limit], Total: len(items)}, nil
		}
	}
	return SearchOutput{Items: items, Total: len(items)}, nil
}
func (u *RecordUseCase) Get(ctx context.Context, input RecordInput) (RecordOutput, error) {
	m, err := u.enabledModule(ctx, input.Module)
	if err != nil {
		return RecordOutput{}, err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.ID)
	if err != nil {
		return RecordOutput{}, ErrNotFound
	}
	canRead := HasModulePermission(input.Actor, m.Key, "READ", false)
	if record.OwnerID == nil {
		canRead = canRead || HasModulePermission(input.Actor, m.Key, "READ", true)
	}
	if !canRead || !withinScope(input.Actor, m, record, false) {
		return RecordOutput{}, ErrNotFound
	}
	return u.output(ctx, input.Actor, record)
}
func (u *RecordUseCase) Detail(ctx context.Context, input RecordInput) (DetailOutput, error) {
	record, err := u.Get(ctx, input)
	if err != nil {
		return DetailOutput{}, err
	}
	followups, err := u.repo.ListFollowups(ctx, record.ID)
	if err != nil {
		return DetailOutput{}, err
	}
	approvals, err := u.repo.ListApprovals(ctx)
	if err != nil {
		return DetailOutput{}, err
	}
	own := make([]ApprovalOutput, 0)
	for _, item := range approvals {
		if item.RecordID == record.ID {
			output, err := approvalOutput(ctx, u.repo, item, entity.Record{ID: record.ID, Module: record.Module, Data: record.Data})
			if err != nil {
				return DetailOutput{}, err
			}
			own = append(own, output)
		}
	}
	followupOutput := make([]FollowupOutput, 0, len(followups))
	for _, item := range followups {
		user, _ := u.repo.FindUserByID(ctx, item.UserID)
		comments, err := u.listFollowupComments(ctx, item.ID)
		if err != nil {
			return DetailOutput{}, err
		}
		followupOutput = append(followupOutput, FollowupOutput{ID: item.ID, Content: item.Content, Method: item.Method, UserName: user.Name, NextDate: item.NextDate, CreatedAt: item.CreatedAt, Comments: comments})
	}
	related := make([]RecordOutput, 0)
	keys, err := u.enabledModuleKeys(ctx)
	if err != nil {
		return DetailOutput{}, err
	}
	for _, key := range keys {
		items, err := u.repo.ListRecords(ctx, key)
		if err != nil {
			return DetailOutput{}, err
		}
		for _, item := range items {
			if fmt.Sprint(item.Data["customer_id"]) == record.ID && visible(input.Actor, Modules[key], item, false) {
				value, _ := u.output(ctx, input.Actor, item)
				related = append(related, value)
			}
		}
	}
	return DetailOutput{RecordOutput: record, Followups: followupOutput, Approvals: own, Related: related}, nil
}
func (u *RecordUseCase) Create(ctx context.Context, input RecordInput) (RecordOutput, error) {
	return u.create(ctx, input, "ADD")
}

func (u *RecordUseCase) create(ctx context.Context, input RecordInput, permissionAction string) (RecordOutput, error) {
	m, err := u.moduleWithCustomFields(ctx, input.Module)
	if err != nil {
		return RecordOutput{}, err
	}
	if !HasModulePermission(input.Actor, m.Key, permissionAction, false) {
		return RecordOutput{}, ErrForbidden
	}
	if m.Key == "payments" && permissionAction == "ADD" && !HasModulePermission(input.Actor, "contracts", "PAYMENT", false) {
		return RecordOutput{}, ErrForbidden
	}
	if m.Shared && input.Actor.Role == entity.RoleSales && input.Actor.RoleID == "" {
		return RecordOutput{}, entity.NewBusinessError("forbidden", "只有管理员和经理可以维护产品目录")
	}
	if err := validateFieldWrite(input.Actor, m, input.Data); err != nil {
		return RecordOutput{}, err
	}
	data, err := validate(m, input.Data)
	if err != nil {
		return RecordOutput{}, err
	}
	if err := u.validateReferences(ctx, input.Actor, m, data); err != nil {
		return RecordOutput{}, err
	}
	if err := u.validateUniqueFields(ctx, m, data, ""); err != nil {
		return RecordOutput{}, err
	}
	if err := u.validateFinancialLimit(ctx, m.Key, data, ""); err != nil {
		return RecordOutput{}, err
	}
	owner := input.Actor.ID
	record := entity.Record{ID: entity.NewID(), Module: m.Key, OrganizationID: input.Actor.OrganizationID, Data: data, OwnerID: &owner, OwnerDepartment: input.Actor.Department, Version: 1, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if m.Key == "orders" {
		err = u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
			if err := u.adjustOrderInventory(transactionContext, nil, data["items"], false); err != nil {
				return err
			}
			return u.repo.CreateRecord(transactionContext, record)
		})
	} else {
		err = u.repo.CreateRecord(ctx, record)
	}
	if err != nil {
		return RecordOutput{}, err
	}
	_ = u.audit(ctx, input.Actor, "创建", m.Key, record.ID, fmt.Sprint(data["name"]))
	return u.output(ctx, input.Actor, record)
}

func (u *RecordUseCase) Import(ctx context.Context, input ImportRecordsInput) (ImportRecordsOutput, error) {
	return u.importRecords(ctx, input, false)
}

func (u *RecordUseCase) PreviewImport(ctx context.Context, input ImportRecordsInput) (ImportRecordsOutput, error) {
	return u.importRecords(ctx, input, true)
}

func (u *RecordUseCase) ImportTemplate(ctx context.Context, actor UserOutput, moduleKey string) (ImportTemplateOutput, error) {
	module, err := u.moduleWithCustomFields(ctx, moduleKey)
	if err != nil {
		return ImportTemplateOutput{}, err
	}
	if !HasModulePermission(actor, module.Key, "IMPORT", false) || (module.Shared && actor.Role == entity.RoleSales && actor.RoleID == "") {
		return ImportTemplateOutput{}, ErrForbidden
	}
	book := excelize.NewFile()
	defer book.Close()
	values := make([]interface{}, 0, len(module.Fields))
	for _, field := range restrictedModule(actor, module).Fields {
		if !fieldWritable(actor, module.Key, field.Key) {
			continue
		}
		values = append(values, field.Key)
	}
	if err := book.SetSheetRow(book.GetSheetName(0), "A1", &values); err != nil {
		return ImportTemplateOutput{}, err
	}
	content, err := book.WriteToBuffer()
	if err != nil {
		return ImportTemplateOutput{}, err
	}
	return ImportTemplateOutput{Module: module, Content: content.Bytes()}, nil
}

func (u *RecordUseCase) importRecords(ctx context.Context, input ImportRecordsInput, preview bool) (ImportRecordsOutput, error) {
	if len(input.Records) == 0 || len(input.Records) > 500 {
		return ImportRecordsOutput{}, ErrValidation
	}
	if _, err := u.enabledModule(ctx, input.Module); err != nil {
		return ImportRecordsOutput{}, err
	}
	if !HasModulePermission(input.Actor, input.Module, "IMPORT", false) {
		return ImportRecordsOutput{}, ErrForbidden
	}
	if err := u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
		for _, data := range input.Records {
			if _, err := u.create(transactionContext, RecordInput{Actor: input.Actor, Module: input.Module, Data: data}, "IMPORT"); err != nil {
				return err
			}
		}
		if preview {
			return errImportPreview
		}
		return nil
	}); err != nil && !(preview && errors.Is(err, errImportPreview)) {
		return ImportRecordsOutput{}, err
	}
	return ImportRecordsOutput{Count: len(input.Records)}, nil
}

func (u *RecordUseCase) ImportCSV(ctx context.Context, actor UserOutput, module string, content []byte) (ImportRecordsOutput, error) {
	records, err := u.parseCSV(ctx, module, content)
	if err != nil {
		return ImportRecordsOutput{}, err
	}
	return u.Import(ctx, ImportRecordsInput{Actor: actor, Module: module, Records: records})
}

func (u *RecordUseCase) PreviewCSV(ctx context.Context, actor UserOutput, module string, content []byte) (ImportRecordsOutput, error) {
	records, err := u.parseCSV(ctx, module, content)
	if err != nil {
		return ImportRecordsOutput{}, err
	}
	return u.PreviewImport(ctx, ImportRecordsInput{Actor: actor, Module: module, Records: records})
}

func (u *RecordUseCase) ImportXLSX(ctx context.Context, actor UserOutput, module string, content []byte) (ImportRecordsOutput, error) {
	records, err := u.parseXLSX(ctx, module, content)
	if err != nil {
		return ImportRecordsOutput{}, err
	}
	return u.Import(ctx, ImportRecordsInput{Actor: actor, Module: module, Records: records})
}

func (u *RecordUseCase) PreviewXLSX(ctx context.Context, actor UserOutput, module string, content []byte) (ImportRecordsOutput, error) {
	records, err := u.parseXLSX(ctx, module, content)
	if err != nil {
		return ImportRecordsOutput{}, err
	}
	return u.PreviewImport(ctx, ImportRecordsInput{Actor: actor, Module: module, Records: records})
}

func (u *RecordUseCase) parseXLSX(ctx context.Context, module string, content []byte) ([]map[string]any, error) {
	book, err := excelize.OpenReader(bytes.NewReader(content))
	if err != nil {
		return nil, entity.NewBusinessError("validation", "XLSX 文件格式错误")
	}
	defer book.Close()
	sheets := book.GetSheetList()
	if len(sheets) == 0 {
		return nil, entity.NewBusinessError("validation", "XLSX 文件缺少工作表")
	}
	rows, err := book.GetRows(sheets[0])
	if err != nil {
		return nil, entity.NewBusinessError("validation", "XLSX 工作表读取失败")
	}
	buffer := bytes.Buffer{}
	writer := csv.NewWriter(&buffer)
	if err := writer.WriteAll(rows); err != nil {
		return nil, err
	}
	return u.parseCSV(ctx, module, buffer.Bytes())
}

func (u *RecordUseCase) parseCSV(ctx context.Context, module string, content []byte) ([]map[string]any, error) {
	definition, err := u.moduleWithCustomFields(ctx, module)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(content))
	reader.TrimLeadingSpace = true
	header, err := reader.Read()
	if err != nil || len(header) == 0 {
		return nil, entity.NewBusinessError("validation", "CSV 文件缺少表头")
	}
	fields := make(map[string]Field, len(definition.Fields))
	for _, field := range definition.Fields {
		fields[field.Key] = field
		fields[field.Label] = field
	}
	columns := make([]Field, len(header))
	for index, value := range header {
		key := strings.TrimPrefix(strings.TrimSpace(value), "\ufeff")
		if key == "ID" || key == "负责人" {
			continue
		}
		field, ok := fields[key]
		if !ok {
			return nil, entity.NewBusinessError("validation", "CSV 包含未知字段："+key)
		}
		columns[index] = field
	}
	records := make([]map[string]any, 0)
	for line := 2; ; line++ {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, entity.NewBusinessError("validation", fmt.Sprintf("CSV 第 %d 行格式错误", line))
		}
		if len(row) != len(columns) {
			return nil, entity.NewBusinessError("validation", fmt.Sprintf("CSV 第 %d 行列数与表头不一致", line))
		}
		data := make(map[string]any)
		for index, value := range row {
			field := columns[index]
			if field.Key == "" || value == "" {
				continue
			}
			if field.Type == "number" {
				number, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return nil, entity.NewBusinessError("validation", fmt.Sprintf("CSV 第 %d 行%s格式错误", line, field.Label))
				}
				data[field.Key] = number
				continue
			}
			if field.Type == "items" {
				var items []any
				if err := json.Unmarshal([]byte(value), &items); err != nil {
					return nil, entity.NewBusinessError("validation", fmt.Sprintf("CSV 第 %d 行%s格式错误", line, field.Label))
				}
				data[field.Key] = items
				continue
			}
			data[field.Key] = value
		}
		records = append(records, data)
	}
	return records, nil
}
func (u *RecordUseCase) Update(ctx context.Context, input RecordInput) (RecordOutput, error) {
	m, err := u.moduleWithCustomFields(ctx, input.Module)
	if err != nil {
		return RecordOutput{}, err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.ID)
	if err != nil {
		return RecordOutput{}, ErrNotFound
	}
	pool := record.OwnerID == nil && (m.Key == "customers" || m.Key == "leads")
	if !HasModulePermission(input.Actor, m.Key, "UPDATE", pool) || (pool && !withinPoolScope(input.Actor, record)) || (!pool && !withinScope(input.Actor, m, record, true)) {
		return RecordOutput{}, ErrForbidden
	}
	if record.Version != input.Version {
		return RecordOutput{}, ErrConflict
	}
	if status := fmt.Sprint(record.Data["approval_status"]); status == string(entity.ApprovalPending) || status == string(entity.ApprovalApproved) {
		return RecordOutput{}, entity.NewBusinessError("conflict", "审批中或已通过的记录不能修改")
	}
	if err := validateFieldWrite(input.Actor, m, input.Data); err != nil {
		return RecordOutput{}, err
	}
	dataInput := make(map[string]any, len(input.Data)+len(record.Data))
	for key, value := range input.Data {
		dataInput[key] = value
	}
	archived, err := u.repo.ListCustomFields(ctx, m.Key)
	if err != nil {
		return RecordOutput{}, err
	}
	for _, field := range archived {
		if field.Definition["archived"] == true {
			delete(dataInput, field.Key)
		}
	}
	standardSettings, err := u.repo.ListStandardFieldSettings(ctx, m.Key)
	if err != nil {
		return RecordOutput{}, err
	}
	for _, setting := range standardSettings {
		if setting.Archived {
			delete(dataInput, setting.Key)
		}
	}
	for _, field := range m.Fields {
		if field.ReadOnly || !fieldWritable(input.Actor, m.Key, field.Key) {
			dataInput[field.Key] = record.Data[field.Key]
		}
	}
	data, err := validate(m, dataInput)
	if err != nil {
		return RecordOutput{}, err
	}
	for _, field := range archived {
		if field.Definition["archived"] == true {
			if value, exists := record.Data[field.Key]; exists {
				data[field.Key] = value
			}
		}
	}
	for _, setting := range standardSettings {
		if setting.Archived {
			if value, exists := record.Data[setting.Key]; exists {
				data[setting.Key] = value
			}
		}
	}
	if (m.Key == "opportunities" || m.Key == "contracts") && fmt.Sprint(record.Data["stage"]) != fmt.Sprint(data["stage"]) {
		if err := u.validateStageTransition(ctx, m.Key, fmt.Sprint(record.Data["stage"]), fmt.Sprint(data["stage"])); err != nil {
			return RecordOutput{}, err
		}
	}
	if m.Key == "opportunities" && fmt.Sprint(record.Data["stage"]) == "赢单" && fmt.Sprint(data["stage"]) == "输单" && !HasModulePermission(input.Actor, m.Key, "RESIGN", false) {
		return RecordOutput{}, ErrForbidden
	}
	if err := u.validateReferences(ctx, input.Actor, m, data); err != nil {
		return RecordOutput{}, err
	}
	if err := u.validateUniqueFields(ctx, m, data, record.ID); err != nil {
		return RecordOutput{}, err
	}
	if err := u.validateFinancialLimit(ctx, m.Key, data, record.ID); err != nil {
		return RecordOutput{}, err
	}
	previousData := record.Data
	record.Data, record.UpdatedAt = data, time.Now().UTC()
	if m.Key == "orders" {
		err = u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
			if err := u.adjustOrderInventory(transactionContext, previousData["items"], data["items"], false); err != nil {
				return err
			}
			return u.repo.UpdateRecord(transactionContext, record, input.Version)
		})
	} else {
		err = u.repo.UpdateRecord(ctx, record, input.Version)
	}
	if err != nil {
		return RecordOutput{}, err
	}
	_ = u.audit(ctx, input.Actor, "更新", m.Key, record.ID, recordDataDiff(previousData, data))
	record.Version++
	return u.output(ctx, input.Actor, record)
}

func (u *RecordUseCase) validateUniqueFields(ctx context.Context, module Module, data map[string]any, excludeID string) error {
	uniqueFields := make([]Field, 0)
	for _, field := range module.Fields {
		if field.Unique && strings.TrimSpace(fmt.Sprint(data[field.Key])) != "" {
			uniqueFields = append(uniqueFields, field)
		}
	}
	if len(uniqueFields) == 0 {
		return nil
	}
	items, err := u.repo.ListRecords(ctx, module.Key)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == excludeID {
			continue
		}
		for _, field := range uniqueFields {
			if fmt.Sprint(item.Data[field.Key]) == fmt.Sprint(data[field.Key]) {
				return entity.NewBusinessError("conflict", field.Label+"已存在相同值")
			}
		}
	}
	return nil
}

func recordDataDiff(previous, current map[string]any) string {
	changes := map[string]map[string]any{}
	keys := map[string]bool{}
	for key := range previous {
		keys[key] = true
	}
	for key := range current {
		keys[key] = true
	}
	for key := range keys {
		if reflect.DeepEqual(previous[key], current[key]) {
			continue
		}
		changes[key] = map[string]any{"before": previous[key], "after": current[key]}
	}
	content, err := json.Marshal(changes)
	if err != nil {
		return "字段已更新"
	}
	return string(content)
}

func (u *RecordUseCase) validateFinancialLimit(ctx context.Context, module string, data map[string]any, excludeID string) error {
	if module != "invoices" && module != "payments" {
		return nil
	}
	amount, err := decimal.NewFromString(fmt.Sprint(data["amount"]))
	if err != nil {
		return ErrValidation
	}
	contractID := fmt.Sprint(data["contract_id"])
	contract, err := u.repo.FindRecord(ctx, "contracts", contractID)
	if err != nil {
		return entity.NewBusinessError("validation", "关联合同不存在")
	}
	limit, err := decimal.NewFromString(fmt.Sprint(contract.Data["amount"]))
	if err != nil {
		return ErrValidation
	}
	items, err := u.repo.ListRecords(ctx, module)
	if err != nil {
		return err
	}
	total := amount
	for _, item := range items {
		if item.ID == excludeID || fmt.Sprint(item.Data["contract_id"]) != contractID {
			continue
		}
		value, err := decimal.NewFromString(fmt.Sprint(item.Data["amount"]))
		if err == nil {
			total = total.Add(value)
		}
	}
	if total.GreaterThan(limit) {
		return entity.NewBusinessError("validation", "累计金额不能超过合同金额")
	}
	if module == "payments" && fmt.Sprint(data["plan_id"]) != "" {
		planID := fmt.Sprint(data["plan_id"])
		plan, err := u.repo.FindRecord(ctx, "payment_plans", planID)
		if err != nil {
			return entity.NewBusinessError("validation", "关联回款计划不存在")
		}
		planLimit, err := decimal.NewFromString(fmt.Sprint(plan.Data["amount"]))
		if err != nil {
			return ErrValidation
		}
		planTotal := amount
		for _, item := range items {
			if item.ID == excludeID || fmt.Sprint(item.Data["plan_id"]) != planID {
				continue
			}
			value, err := decimal.NewFromString(fmt.Sprint(item.Data["amount"]))
			if err == nil {
				planTotal = planTotal.Add(value)
			}
		}
		if planTotal.GreaterThan(planLimit) {
			return entity.NewBusinessError("validation", "到账金额不能超过回款计划金额")
		}
	}
	return nil
}
func (u *RecordUseCase) Delete(ctx context.Context, input RecordInput) error {
	m, err := u.enabledModule(ctx, input.Module)
	if err != nil {
		return err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.ID)
	if err != nil {
		return ErrNotFound
	}
	pool := record.OwnerID == nil && (m.Key == "customers" || m.Key == "leads")
	if !HasModulePermission(input.Actor, m.Key, "DELETE", pool) || (pool && !withinPoolScope(input.Actor, record)) || (!pool && !withinScope(input.Actor, m, record, true)) {
		return ErrForbidden
	}
	if record.Version != input.Version {
		return ErrConflict
	}
	if status := fmt.Sprint(record.Data["approval_status"]); status == string(entity.ApprovalPending) || status == string(entity.ApprovalApproved) {
		return entity.NewBusinessError("conflict", "审批中或已通过的记录不能删除")
	}
	if m.Key == "orders" {
		err = u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
			if err := u.adjustOrderInventory(transactionContext, record.Data["items"], nil, false); err != nil {
				return err
			}
			return u.repo.DeleteRecord(transactionContext, m.Key, input.ID, input.Version)
		})
	} else {
		err = u.repo.DeleteRecord(ctx, m.Key, input.ID, input.Version)
	}
	if err == nil {
		_ = u.audit(ctx, input.Actor, "删除", m.Key, record.ID, fmt.Sprint(record.Data["name"]))
	}
	return err
}
func (u *RecordUseCase) AddFollowup(ctx context.Context, input FollowupInput) error {
	m, err := u.enabledModule(ctx, input.Module)
	if err != nil {
		return err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.RecordID)
	if err != nil {
		return ErrNotFound
	}
	if !visible(input.Actor, m, record, true) {
		return ErrForbidden
	}
	if strings.TrimSpace(input.Content) == "" {
		return ErrValidation
	}
	now := time.Now().UTC()
	err = u.repo.WithinTransaction(ctx, func(tx context.Context) error {
		if err := u.repo.CreateFollowup(tx, entity.Followup{ID: entity.NewID(), RecordID: record.ID, UserID: input.Actor.ID, Content: strings.TrimSpace(input.Content), Method: input.Method, NextDate: input.NextDate, CreatedAt: now}); err != nil {
			return err
		}
		record.UpdatedAt = now
		return u.repo.UpdateRecord(tx, record, record.Version)
	})
	if err == nil {
		_ = u.audit(ctx, input.Actor, "添加跟进", m.Key, record.ID, "")
	}
	return err
}
func (u *RecordUseCase) CreateFollowupComment(ctx context.Context, input FollowupCommentInput) (FollowupCommentOutput, error) {
	_, _, followup, err := u.followupAccess(ctx, input, true)
	if err != nil {
		return FollowupCommentOutput{}, err
	}
	content := strings.TrimSpace(input.Content)
	if content == "" || len([]rune(content)) > 2000 {
		return FollowupCommentOutput{}, ErrValidation
	}
	now := time.Now().UTC()
	comment := entity.FollowupComment{ID: entity.NewID(), FollowupID: followup.ID, UserID: input.Actor.ID, Content: content, CreatedAt: now, UpdatedAt: now}
	if err = u.repo.CreateFollowupComment(ctx, comment); err != nil {
		return FollowupCommentOutput{}, err
	}
	return u.followupCommentOutput(ctx, comment)
}
func (u *RecordUseCase) UpdateFollowupComment(ctx context.Context, input UpdateFollowupCommentInput) (FollowupCommentOutput, error) {
	if _, _, followup, err := u.followupAccess(ctx, input.FollowupCommentInput, true); err != nil {
		return FollowupCommentOutput{}, err
	} else {
		comment, err := u.repo.FindFollowupComment(ctx, input.CommentID)
		if err != nil || comment.FollowupID != followup.ID {
			return FollowupCommentOutput{}, ErrNotFound
		}
		if comment.UserID != input.Actor.ID && !isManager(input.Actor) {
			return FollowupCommentOutput{}, ErrForbidden
		}
		content := strings.TrimSpace(input.Content)
		if content == "" || len([]rune(content)) > 2000 {
			return FollowupCommentOutput{}, ErrValidation
		}
		comment.Content, comment.UpdatedAt = content, time.Now().UTC()
		if err = u.repo.UpdateFollowupComment(ctx, comment); err != nil {
			return FollowupCommentOutput{}, err
		}
		return u.followupCommentOutput(ctx, comment)
	}
}
func (u *RecordUseCase) DeleteFollowupComment(ctx context.Context, input DeleteFollowupCommentInput) error {
	if _, _, followup, err := u.followupAccess(ctx, FollowupCommentInput{Actor: input.Actor, Module: input.Module, RecordID: input.RecordID, FollowupID: input.FollowupID}, true); err != nil {
		return err
	} else {
		comment, err := u.repo.FindFollowupComment(ctx, input.ID)
		if err != nil || comment.FollowupID != followup.ID {
			return ErrNotFound
		}
		if comment.UserID != input.Actor.ID && !isManager(input.Actor) {
			return ErrForbidden
		}
		return u.repo.DeleteFollowupComment(ctx, comment.ID)
	}
}
func (u *RecordUseCase) ListFollowupComments(ctx context.Context, input FollowupCommentInput) (ListFollowupCommentsOutput, error) {
	if _, _, _, err := u.followupAccess(ctx, input, false); err != nil {
		return ListFollowupCommentsOutput{}, err
	}
	items, err := u.listFollowupComments(ctx, input.FollowupID)
	if err != nil {
		return ListFollowupCommentsOutput{}, err
	}
	return ListFollowupCommentsOutput{Items: items}, nil
}
func (u *RecordUseCase) followupAccess(ctx context.Context, input FollowupCommentInput, write bool) (Module, entity.Record, entity.Followup, error) {
	m, err := u.enabledModule(ctx, input.Module)
	if err != nil {
		return Module{}, entity.Record{}, entity.Followup{}, err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.RecordID)
	if err != nil || !visible(input.Actor, m, record, write) {
		return Module{}, entity.Record{}, entity.Followup{}, ErrNotFound
	}
	followup, err := u.repo.FindFollowup(ctx, input.FollowupID)
	if err != nil || followup.RecordID != record.ID {
		return Module{}, entity.Record{}, entity.Followup{}, ErrNotFound
	}
	return m, record, followup, nil
}
func (u *RecordUseCase) listFollowupComments(ctx context.Context, followupID string) ([]FollowupCommentOutput, error) {
	comments, err := u.repo.ListFollowupComments(ctx, followupID)
	if err != nil {
		return nil, err
	}
	items := make([]FollowupCommentOutput, 0, len(comments))
	for _, comment := range comments {
		output, err := u.followupCommentOutput(ctx, comment)
		if err != nil {
			return nil, err
		}
		items = append(items, output)
	}
	return items, nil
}
func (u *RecordUseCase) followupCommentOutput(ctx context.Context, comment entity.FollowupComment) (FollowupCommentOutput, error) {
	user, err := u.repo.FindUserByID(ctx, comment.UserID)
	if err != nil {
		return FollowupCommentOutput{}, err
	}
	return FollowupCommentOutput{ID: comment.ID, FollowupID: comment.FollowupID, UserID: comment.UserID, UserName: user.Name, Content: comment.Content, CreatedAt: comment.CreatedAt, UpdatedAt: comment.UpdatedAt}, nil
}
func (u *RecordUseCase) Action(ctx context.Context, input ActionInput) (RecordOutput, error) {
	m, err := module(input.Module)
	if err != nil {
		return RecordOutput{}, err
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.ID)
	if err != nil {
		return RecordOutput{}, ErrNotFound
	}
	if input.Actor.RoleID != "" && record.OrganizationID != input.Actor.OrganizationID {
		return RecordOutput{}, ErrForbidden
	}
	if record.Version != input.Version {
		return RecordOutput{}, ErrConflict
	}
	poolFrozen := normalizePoolFreeze(&record)
	previousOwnerID := ""
	if record.OwnerID != nil {
		previousOwnerID = *record.OwnerID
	}
	var approvalToCreate *entity.Approval
	switch input.Action {
	case "claim":
		if !HasModulePermission(input.Actor, m.Key, "PICK", true) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "customers" && m.Key != "leads" {
			return RecordOutput{}, entity.NewBusinessError("validation", "该模块不支持领取")
		}
		if record.OwnerID != nil {
			return RecordOutput{}, entity.NewBusinessError("conflict", "该记录已被领取")
		}
		if poolFrozen {
			return RecordOutput{}, entity.NewBusinessError("conflict", "该公海记录已冻结，不能领取")
		}
		record.OwnerID = &input.Actor.ID
		record.OwnerDepartment = input.Actor.Department
	case "release":
		if !HasModulePermission(input.Actor, m.Key, "RECYCLE", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "customers" && m.Key != "leads" {
			return RecordOutput{}, entity.NewBusinessError("validation", "该模块不支持放入公海")
		}
		if !withinScope(input.Actor, m, record, true) {
			return RecordOutput{}, ErrForbidden
		}
		record.OwnerID = nil
		record.OwnerDepartment = ""
	case "assign":
		pool := record.OwnerID == nil && (m.Key == "customers" || m.Key == "leads")
		action := "TRANSFER"
		if pool {
			action = "ASSIGN"
		}
		if !HasModulePermission(input.Actor, m.Key, action, pool) {
			return RecordOutput{}, ErrForbidden
		}
		if pool && poolFrozen {
			return RecordOutput{}, entity.NewBusinessError("conflict", "该公海记录已冻结，不能分配")
		}
		if input.OwnerID == nil {
			return RecordOutput{}, ErrValidation
		}
		owner, err := u.repo.FindUserByID(ctx, *input.OwnerID)
		if err != nil || !owner.Active || (record.OrganizationID != "" && owner.OrganizationID != record.OrganizationID) {
			return RecordOutput{}, ErrValidation
		}
		record.OwnerID = input.OwnerID
		record.OwnerDepartment = owner.Department
	case "freeze":
		if !HasModulePermission(input.Actor, m.Key, "FREEZE", true) {
			return RecordOutput{}, ErrForbidden
		}
		if (m.Key != "customers" && m.Key != "leads") || record.OwnerID != nil || strings.TrimSpace(input.Reason) == "" || len([]rune(strings.TrimSpace(input.Reason))) > 500 || input.FreezeDays < 0 || input.FreezeDays > 365 {
			return RecordOutput{}, ErrValidation
		}
		if poolFrozen {
			return RecordOutput{}, entity.NewBusinessError("conflict", "该公海记录已冻结")
		}
		record.Data["frozen"] = true
		record.Data["freeze_reason"] = strings.TrimSpace(input.Reason)
		if input.FreezeDays > 0 {
			record.Data["unfreeze_at"] = time.Now().UTC().AddDate(0, 0, input.FreezeDays).Format(time.RFC3339)
		} else {
			delete(record.Data, "unfreeze_at")
		}
	case "unfreeze":
		if !HasModulePermission(input.Actor, m.Key, "FREEZE", true) {
			return RecordOutput{}, ErrForbidden
		}
		if (m.Key != "customers" && m.Key != "leads") || record.OwnerID != nil || !poolFrozen {
			return RecordOutput{}, ErrValidation
		}
		delete(record.Data, "frozen")
		delete(record.Data, "freeze_reason")
		delete(record.Data, "unfreeze_at")
	case "void":
		if !HasModulePermission(input.Actor, m.Key, "VOIDED", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "quotes" || record.Data["voided"] == true || fmt.Sprint(record.Data["approval_status"]) != string(entity.ApprovalApproved) {
			return RecordOutput{}, ErrValidation
		}
		contracts, err := u.repo.ListRecords(ctx, "contracts")
		if err != nil {
			return RecordOutput{}, err
		}
		for _, contract := range contracts {
			if fmt.Sprint(contract.Data["quote_id"]) == record.ID {
				return RecordOutput{}, entity.NewBusinessError("conflict", "报价已关联合同，不能作废")
			}
		}
		record.Data["voided"] = true
		record.Data["voided_at"] = time.Now().UTC().Format(time.RFC3339)
	case "merge":
		if !HasModulePermission(input.Actor, m.Key, "MERGE", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "customers" || record.OwnerID == nil || !withinScope(input.Actor, m, record, true) {
			return RecordOutput{}, ErrForbidden
		}
		return u.mergeCustomers(ctx, input, record)
	case "stage":
		if !HasModulePermission(input.Actor, m.Key, "STAGE", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "contracts" || !withinScope(input.Actor, m, record, true) || fmt.Sprint(record.Data["approval_status"]) != string(entity.ApprovalApproved) {
			return RecordOutput{}, ErrValidation
		}
		if err := u.validateStageTransition(ctx, m.Key, fmt.Sprint(record.Data["stage"]), input.TargetStage); err != nil {
			return RecordOutput{}, err
		}
		record.Data["stage"] = input.TargetStage
	case "convert":
		if !HasModulePermission(input.Actor, "leads", "UPDATE", false) || !HasModulePermission(input.Actor, "customers", "ADD", false) || !HasModulePermission(input.Actor, "contacts", "ADD", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "leads" {
			return RecordOutput{}, entity.NewBusinessError("validation", "仅线索支持转化")
		}
		if !withinScope(input.Actor, m, record, true) || record.OwnerID == nil {
			return RecordOutput{}, entity.NewBusinessError("validation", "请先领取线索")
		}
		if fmt.Sprint(record.Data["status"]) == "无效" {
			return RecordOutput{}, entity.NewBusinessError("validation", "无效线索不能转化")
		}
		customerID := entity.NewID()
		now := time.Now().UTC()
		customer := entity.Record{ID: customerID, Module: "customers", OrganizationID: record.OrganizationID, OwnerID: record.OwnerID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: map[string]any{"name": fmt.Sprint(record.Data["company"]), "industry": fmt.Sprint(record.Data["industry"]), "source": fmt.Sprint(record.Data["source"]), "level": "B · 潜力", "status": "潜在客户", "phone": fmt.Sprint(record.Data["phone"]), "website": "", "address": "", "notes": fmt.Sprint(record.Data["notes"])}}
		if err := u.repo.CreateRecord(ctx, customer); err != nil {
			return RecordOutput{}, err
		}
		contact := entity.Record{ID: entity.NewID(), Module: "contacts", OrganizationID: record.OrganizationID, OwnerID: record.OwnerID, Version: 1, CreatedAt: now, UpdatedAt: now, Data: map[string]any{"name": fmt.Sprint(record.Data["contact"]), "customer_id": customerID, "title": "", "phone": fmt.Sprint(record.Data["phone"]), "email": fmt.Sprint(record.Data["email"]), "role": "其他", "notes": ""}}
		if err := u.repo.CreateRecord(ctx, contact); err != nil {
			return RecordOutput{}, err
		}
		followups, err := u.repo.ListFollowups(ctx, record.ID)
		if err != nil {
			return RecordOutput{}, err
		}
		for _, followup := range followups {
			followup.ID = entity.NewID()
			followup.RecordID = customerID
			if err := u.repo.CreateFollowup(ctx, followup); err != nil {
				return RecordOutput{}, err
			}
		}
		record.Data["converted_customer_id"] = customerID
		record.Data["status"] = "已转化"
	case "submit":
		if !HasModulePermission(input.Actor, m.Key, "UPDATE", false) {
			return RecordOutput{}, ErrForbidden
		}
		if !m.Approvable {
			return RecordOutput{}, entity.NewBusinessError("validation", "该模块无需审批")
		}
		if fmt.Sprint(record.Data["approval_status"]) == string(entity.ApprovalPending) {
			return RecordOutput{}, entity.NewBusinessError("conflict", "记录正在审批")
		}
		approval := entity.Approval{ID: entity.NewID(), RecordID: record.ID, RequestedBy: input.Actor.ID, Status: entity.ApprovalPending, CreatedAt: time.Now().UTC()}
		flows, err := u.repo.ListApprovalFlows(ctx, m.Key)
		if err != nil {
			return RecordOutput{}, err
		}
		for _, flow := range flows {
			if !flow.Enabled {
				continue
			}
			version, err := u.repo.FindApprovalFlowVersion(ctx, flow.ID, flow.Version)
			if err != nil || len(version.Steps) == 0 {
				return RecordOutput{}, ErrValidation
			}
			approval.FlowID = flow.ID
			approval.FlowVersion = version.Version
			approval.CurrentStep = nextApplicableApprovalStep(version.Steps, record, 0)
			if approval.CurrentStep == 0 {
				return RecordOutput{}, ErrValidation
			}
			break
		}
		record.Data["approval_status"] = string(entity.ApprovalPending)
		approvalToCreate = &approval
	case "deliver":
		if !HasModulePermission(input.Actor, m.Key, "UPDATE", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "orders" || fmt.Sprint(record.Data["approval_status"]) != "approved" {
			return RecordOutput{}, entity.NewBusinessError("validation", "只有已通过审批的订单可以交付")
		}
		if fmt.Sprint(record.Data["delivery_status"]) == "已完成" {
			return RecordOutput{}, entity.NewBusinessError("conflict", "订单已交付")
		}
		record.Data["delivery_status"] = "已完成"
	case "split":
		if !HasModulePermission(input.Actor, m.Key, "UPDATE", false) || !HasModulePermission(input.Actor, m.Key, "ADD", false) {
			return RecordOutput{}, ErrForbidden
		}
		if m.Key != "orders" || fmt.Sprint(record.Data["approval_status"]) != string(entity.ApprovalDraft) {
			return RecordOutput{}, entity.NewBusinessError("validation", "只有草稿订单可以拆分")
		}
		items, ok := record.Data["items"].([]any)
		if !ok || len(input.ItemIndexes) == 0 || len(input.ItemIndexes) >= len(items) {
			return RecordOutput{}, entity.NewBusinessError("validation", "请选择部分产品明细进行拆分")
		}
		selected, remaining, seen := []any{}, []any{}, map[int]bool{}
		for _, index := range input.ItemIndexes {
			if index < 0 || index >= len(items) || seen[index] {
				return RecordOutput{}, ErrValidation
			}
			seen[index] = true
		}
		for index, item := range items {
			if seen[index] {
				selected = append(selected, item)
			} else {
				remaining = append(remaining, item)
			}
		}
		selected, selectedTotal, _ := normalizeLineItems(selected)
		remaining, remainingTotal, _ := normalizeLineItems(remaining)
		now := time.Now().UTC()
		childData := make(map[string]any, len(record.Data))
		for key, value := range record.Data {
			childData[key] = value
		}
		childData["name"] = fmt.Sprint(record.Data["name"]) + "（拆分）"
		childData["code"] = fmt.Sprintf("%s-S%d", record.Data["code"], record.Version)
		childData["items"], childData["amount"] = selected, selectedTotal.StringFixed(2)
		child := entity.Record{ID: entity.NewID(), Module: "orders", OrganizationID: record.OrganizationID, Data: childData, OwnerID: record.OwnerID, OwnerDepartment: record.OwnerDepartment, Version: 1, CreatedAt: now, UpdatedAt: now}
		record.Data["items"], record.Data["amount"], record.UpdatedAt = remaining, remainingTotal.StringFixed(2), now
		if err := u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
			if err := u.repo.UpdateRecord(transactionContext, record, input.Version); err != nil {
				return err
			}
			return u.repo.CreateRecord(transactionContext, child)
		}); err != nil {
			return RecordOutput{}, err
		}
		record.Version++
		_ = u.audit(ctx, input.Actor, "split", m.Key, record.ID, fmt.Sprint(childData["code"]))
		return u.output(ctx, input.Actor, record)
	default:
		return RecordOutput{}, entity.NewBusinessError("not_found", "操作不存在")
	}
	if input.Action == "deliver" {
		record.UpdatedAt = time.Now().UTC()
		if err := u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
			if err := u.adjustOrderInventory(transactionContext, record.Data["items"], nil, true); err != nil {
				return err
			}
			return u.repo.UpdateRecord(transactionContext, record, input.Version)
		}); err != nil {
			return RecordOutput{}, err
		}
		record.Version++
		_ = u.audit(ctx, input.Actor, input.Action, m.Key, record.ID, fmt.Sprint(record.Data["name"]))
		return u.output(ctx, input.Actor, record)
	}
	if m.Key == "customers" && (input.Action == "claim" || input.Action == "release" || input.Action == "assign") {
		keys, err := u.enabledModuleKeys(ctx)
		if err != nil {
			return RecordOutput{}, err
		}
		for _, key := range keys {
			if key == "customers" || !allowed(input.Actor, key, true) {
				continue
			}
			related, err := u.repo.ListRecords(ctx, key)
			if err != nil {
				return RecordOutput{}, err
			}
			for _, item := range related {
				if fmt.Sprint(item.Data["customer_id"]) != record.ID || item.OrganizationID != record.OrganizationID {
					continue
				}
				item.OwnerID = record.OwnerID
				item.UpdatedAt = time.Now().UTC()
				if err := u.repo.UpdateRecord(ctx, item, item.Version); err != nil {
					return RecordOutput{}, err
				}
			}
		}
	}
	record.UpdatedAt = time.Now().UTC()
	if approvalToCreate != nil {
		if err := u.repo.SubmitApproval(ctx, *approvalToCreate, record, input.Version); err != nil {
			return RecordOutput{}, err
		}
	} else {
		if err := u.repo.UpdateRecord(ctx, record, input.Version); err != nil {
			return RecordOutput{}, err
		}
	}
	record.Version++
	ownerID := ""
	if record.OwnerID != nil {
		ownerID = *record.OwnerID
	}
	if u.ownerships != nil && (input.Action == "claim" || input.Action == "release" || input.Action == "assign") && previousOwnerID != ownerID {
		if err := u.ownerships.CreateOwnershipHistory(ctx, entity.OwnershipHistory{ID: entity.NewID(), Module: m.Key, RecordID: record.ID, PreviousOwnerID: previousOwnerID, OwnerID: ownerID, OperatorID: input.Actor.ID, Action: input.Action, CreatedAt: record.UpdatedAt}); err != nil {
			return RecordOutput{}, err
		}
	}
	_ = u.audit(ctx, input.Actor, input.Action, m.Key, record.ID, fmt.Sprint(record.Data["name"]))
	return u.output(ctx, input.Actor, record)
}

func (u *RecordUseCase) mergeCustomers(ctx context.Context, input ActionInput, target entity.Record) (RecordOutput, error) {
	if len(input.MergeIDs) == 0 || len(input.MergeIDs) > 20 {
		return RecordOutput{}, ErrValidation
	}
	keys, err := u.enabledModuleKeys(ctx)
	if err != nil {
		return RecordOutput{}, err
	}
	sources := make([]entity.Record, 0, len(input.MergeIDs))
	seen := map[string]bool{target.ID: true}
	for _, sourceID := range input.MergeIDs {
		sourceID = strings.TrimSpace(sourceID)
		if sourceID == "" || seen[sourceID] {
			return RecordOutput{}, ErrValidation
		}
		seen[sourceID] = true
		source, err := u.repo.FindRecord(ctx, "customers", sourceID)
		if err != nil || source.OrganizationID != target.OrganizationID || source.OwnerID == nil || !withinScope(input.Actor, Modules["customers"], source, true) {
			return RecordOutput{}, ErrForbidden
		}
		sources = append(sources, source)
	}
	err = u.repo.WithinTransaction(ctx, func(transactionContext context.Context) error {
		for _, key := range keys {
			if key == "customers" {
				continue
			}
			items, err := u.repo.ListRecords(transactionContext, key)
			if err != nil {
				return err
			}
			for _, item := range items {
				if item.OrganizationID != target.OrganizationID || !seen[fmt.Sprint(item.Data["customer_id"])] {
					continue
				}
				item.Data["customer_id"] = target.ID
				item.UpdatedAt = time.Now().UTC()
				if err := u.repo.UpdateRecord(transactionContext, item, item.Version); err != nil {
					return err
				}
			}
		}
		for _, source := range sources {
			if err := u.repo.MoveRecordActivities(transactionContext, "customers", source.ID, target.ID); err != nil {
				return err
			}
			if err := u.repo.DeleteRecord(transactionContext, "customers", source.ID, source.Version); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return RecordOutput{}, err
	}
	_ = u.audit(ctx, input.Actor, "merge", "customers", target.ID, strings.Join(input.MergeIDs, ","))
	return u.output(ctx, input.Actor, target)
}

func normalizePoolFreeze(record *entity.Record) bool {
	if record.Data["frozen"] != true {
		return false
	}
	unfreezeAt := strings.TrimSpace(fmt.Sprint(record.Data["unfreeze_at"]))
	if unfreezeAt == "" || unfreezeAt == "<nil>" {
		return true
	}
	value, err := time.Parse(time.RFC3339, unfreezeAt)
	if err != nil || value.After(time.Now().UTC()) {
		return true
	}
	delete(record.Data, "frozen")
	delete(record.Data, "freeze_reason")
	delete(record.Data, "unfreeze_at")
	return false
}

func (u *RecordUseCase) ListOwnershipHistory(ctx context.Context, input ListOwnershipHistoryInput) (ListOwnershipHistoryOutput, error) {
	if u.ownerships == nil {
		return ListOwnershipHistoryOutput{Items: []OwnershipHistoryOutput{}}, nil
	}
	m, err := module(input.Module)
	if err != nil {
		return ListOwnershipHistoryOutput{}, err
	}
	if m.Key != "customers" && m.Key != "leads" {
		return ListOwnershipHistoryOutput{}, ErrNotFound
	}
	record, err := u.repo.FindRecord(ctx, m.Key, input.ID)
	if err != nil {
		return ListOwnershipHistoryOutput{}, ErrNotFound
	}
	if !visible(input.Actor, m, record, false) {
		return ListOwnershipHistoryOutput{}, ErrForbidden
	}
	items, err := u.ownerships.ListOwnershipHistory(ctx, m.Key, record.ID)
	if err != nil {
		return ListOwnershipHistoryOutput{}, err
	}
	output := make([]OwnershipHistoryOutput, 0, len(items))
	for _, item := range items {
		row := OwnershipHistoryOutput{ID: item.ID, PreviousOwnerID: item.PreviousOwnerID, OwnerID: item.OwnerID, OperatorID: item.OperatorID, Action: item.Action, CreatedAt: item.CreatedAt}
		if item.PreviousOwnerID != "" {
			user, err := u.repo.FindUserByID(ctx, item.PreviousOwnerID)
			if err != nil {
				return ListOwnershipHistoryOutput{}, err
			}
			row.PreviousOwnerName = user.Name
		}
		if item.OwnerID != "" {
			user, err := u.repo.FindUserByID(ctx, item.OwnerID)
			if err != nil {
				return ListOwnershipHistoryOutput{}, err
			}
			row.OwnerName = user.Name
		}
		user, err := u.repo.FindUserByID(ctx, item.OperatorID)
		if err != nil {
			return ListOwnershipHistoryOutput{}, err
		}
		row.OperatorName = user.Name
		output = append(output, row)
	}
	return ListOwnershipHistoryOutput{Items: output}, nil
}

func (u *RecordUseCase) audit(ctx context.Context, actor UserOutput, action, module, recordID, detail string) error {
	return u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: actor.ID, Action: action, Module: module, RecordID: &recordID, Detail: detail, CreatedAt: time.Now().UTC()})
}
