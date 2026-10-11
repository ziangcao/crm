package usecase

import (
	"context"
	"cordialcrm/backend/internal/entity"
	"strings"
	"time"
)

type UserUseCase struct {
	repo      Repository
	passwords PasswordHasher
}

func NewUser(repo Repository, passwords PasswordHasher) *UserUseCase {
	return &UserUseCase{repo, passwords}
}

type CreateUserInput struct {
	Actor                                UserOutput
	Username, Password, Name, Department string
	Role                                 entity.Role
	OrganizationID, RoleID               string
}
type UpdateUserInput struct {
	Actor                  UserOutput
	ID, Name, Department   string
	Role                   entity.Role
	OrganizationID, RoleID string
	Active                 bool
	Password               *string
}
type ExternalUser struct{ Username, Name string }

func requireAdmin(actor UserOutput) error {
	if actor.Role != entity.RoleAdmin {
		return entity.NewBusinessError("forbidden", "需要管理员权限")
	}
	return nil
}
func validRole(role entity.Role) bool {
	return role == entity.RoleAdmin || role == entity.RoleManager || role == entity.RoleSales
}
func (u *UserUseCase) validateAssignment(ctx context.Context, role entity.Role, organizationID, roleID string) error {
	if (organizationID == "") != (roleID == "") {
		return ErrValidation
	}
	if roleID == "" {
		return nil
	}
	if role != entity.RoleSales {
		return entity.NewBusinessError("validation", "自定义角色成员必须使用销售基础角色")
	}
	assigned, err := u.repo.FindRoleByID(ctx, roleID)
	if err != nil || assigned.OrganizationID != organizationID || assigned.BuiltIn {
		return entity.NewBusinessError("validation", "角色不存在或不属于当前组织")
	}
	return nil
}
func (u *UserUseCase) validateDepartment(ctx context.Context, organizationID, departmentID string) error {
	if organizationID == "" || departmentID == "" {
		return nil
	}
	departments, err := u.repo.ListDepartments(ctx, organizationID)
	if err != nil {
		return err
	}
	for _, department := range departments {
		if department.ID == departmentID {
			return nil
		}
	}
	return entity.NewBusinessError("validation", "部门不存在或不属于当前组织")
}
func (u *UserUseCase) List(ctx context.Context, actor UserOutput) ([]UserOutput, error) {
	if actor.RoleID != "" && !HasPermission(actor, "SYS_ORGANIZATION:READ") && !HasPermission(actor, "SYSTEM_ROLE:ADD_USER") {
		return nil, ErrForbidden
	}
	if actor.Role == entity.RoleSales && actor.RoleID == "" {
		return []UserOutput{actor}, nil
	}
	users, err := u.repo.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]UserOutput, len(users))
	for i, user := range users {
		out[i] = publicUser(user)
	}
	return out, nil
}
func (u *UserUseCase) SyncExternal(ctx context.Context, actor UserOutput, source []ExternalUser) (int, error) {
	if err := requirePermission(actor, "SYS_ORGANIZATION:SYNC"); err != nil {
		return 0, err
	}
	users, err := u.repo.ListUsers(ctx)
	if err != nil {
		return 0, err
	}
	byUsername := make(map[string]entity.User, len(users))
	for _, user := range users {
		byUsername[user.Username] = user
	}
	count := 0
	for _, external := range source {
		external.Username, external.Name = strings.TrimSpace(external.Username), strings.TrimSpace(external.Name)
		if !validUsername(external.Username) || external.Name == "" {
			continue
		}
		if user, found := byUsername[external.Username]; found {
			if user.Name != external.Name {
				user.Name = external.Name
				if err := u.repo.UpdateUser(ctx, user); err != nil {
					return count, err
				}
			}
		} else {
			hash, err := u.passwords.Hash(entity.NewID() + "Aa1")
			if err != nil {
				return count, err
			}
			user := entity.User{ID: entity.NewID(), Username: external.Username, Name: external.Name, PasswordHash: hash, Role: entity.RoleSales, OrganizationID: actor.OrganizationID, Active: true, AuthVersion: 1, CreatedAt: time.Now().UTC()}
			if err := u.repo.CreateUser(ctx, user); err != nil {
				return count, err
			}
		}
		count++
	}
	return count, nil
}
func (u *UserUseCase) Create(ctx context.Context, input CreateUserInput) (UserOutput, error) {
	if err := requirePermission(input.Actor, "SYSTEM_ROLE:ADD_USER"); err != nil {
		return UserOutput{}, err
	}
	if !validUsername(input.Username) || !validPassword(input.Password) || strings.TrimSpace(input.Name) == "" || !validRole(input.Role) {
		return UserOutput{}, ErrValidation
	}
	if err := u.validateAssignment(ctx, input.Role, input.OrganizationID, input.RoleID); err != nil {
		return UserOutput{}, err
	}
	if err := u.validateDepartment(ctx, input.OrganizationID, strings.TrimSpace(input.Department)); err != nil {
		return UserOutput{}, err
	}
	hash, err := u.passwords.Hash(input.Password)
	if err != nil {
		return UserOutput{}, err
	}
	user := entity.User{ID: entity.NewID(), Username: input.Username, Name: strings.TrimSpace(input.Name), PasswordHash: hash, Role: input.Role, OrganizationID: input.OrganizationID, RoleID: input.RoleID, Department: strings.TrimSpace(input.Department), Active: true, AuthVersion: 1, CreatedAt: time.Now().UTC()}
	if err = u.repo.CreateUser(ctx, user); err != nil {
		return UserOutput{}, err
	}
	created, err := u.repo.FindUserByID(ctx, user.ID)
	if err != nil {
		return UserOutput{}, err
	}
	return publicUser(created), nil
}
func (u *UserUseCase) Update(ctx context.Context, input UpdateUserInput) (UserOutput, error) {
	permissionID := "SYSTEM_ROLE:ADD_USER"
	if !input.Active {
		permissionID = "SYSTEM_ROLE:REMOVE_USER"
	}
	if err := requirePermission(input.Actor, permissionID); err != nil {
		return UserOutput{}, err
	}
	if input.Password != nil {
		if err := requirePermission(input.Actor, "SYS_ORGANIZATION_USER:RESET_PASSWORD"); err != nil {
			return UserOutput{}, err
		}
	}
	if strings.TrimSpace(input.Name) == "" || !validRole(input.Role) {
		return UserOutput{}, ErrValidation
	}
	if err := u.validateAssignment(ctx, input.Role, input.OrganizationID, input.RoleID); err != nil {
		return UserOutput{}, err
	}
	if err := u.validateDepartment(ctx, input.OrganizationID, strings.TrimSpace(input.Department)); err != nil {
		return UserOutput{}, err
	}
	user, err := u.repo.FindUserByID(ctx, input.ID)
	if err != nil {
		return UserOutput{}, ErrNotFound
	}
	if user.ID == input.Actor.ID && (!input.Active || input.Role != entity.RoleAdmin) {
		return UserOutput{}, entity.NewBusinessError("validation", "不能停用自己或移除自己的管理员权限")
	}
	user.Name, user.Department, user.Role, user.Active = strings.TrimSpace(input.Name), strings.TrimSpace(input.Department), input.Role, input.Active
	user.OrganizationID, user.RoleID = input.OrganizationID, input.RoleID
	if input.Password != nil {
		if !validPassword(*input.Password) {
			return UserOutput{}, ErrValidation
		}
		user.PasswordHash, err = u.passwords.Hash(*input.Password)
		if err != nil {
			return UserOutput{}, err
		}
		user.AuthVersion++
	}
	if err = u.repo.UpdateUser(ctx, user); err != nil {
		return UserOutput{}, err
	}
	updated, err := u.repo.FindUserByID(ctx, user.ID)
	if err != nil {
		return UserOutput{}, err
	}
	return publicUser(updated), nil
}
