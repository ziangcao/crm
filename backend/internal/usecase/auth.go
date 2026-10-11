package usecase

import (
	"context"
	"regexp"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

type AuthUseCase struct {
	repo       Repository
	passwords  PasswordHasher
	tokens     TokenGenerator
	sessionTTL time.Duration
}

type setupOrganizationRepository interface {
	SaveOrganization(context.Context, entity.Organization) error
}

func NewAuth(repo Repository, passwords PasswordHasher, tokens TokenGenerator, sessionTTL time.Duration) *AuthUseCase {
	return &AuthUseCase{repo, passwords, tokens, sessionTTL}
}

type SetupInput struct {
	Username, Password, Name string
	Demo                     bool
}
type LoginInput struct{ Username, Password string }
type ChangePasswordInput struct{ UserID, CurrentPassword, NewPassword string }
type AuthOutput struct {
	User  UserOutput
	Token string
}
type UserOutput struct {
	ID                string                             `json:"id"`
	Username          string                             `json:"username"`
	Name              string                             `json:"name"`
	Department        string                             `json:"department"`
	Role              entity.Role                        `json:"role"`
	OrganizationID    string                             `json:"organization_id"`
	RoleID            string                             `json:"role_id"`
	PermissionIDs     []string                           `json:"permission_ids"`
	DataScope         string                             `json:"data_scope"`
	DepartmentIDs     []string                           `json:"department_ids"`
	FieldRestrictions map[string]entity.FieldRestriction `json:"field_restrictions"`
	Active            bool                               `json:"active"`
}

func publicUser(user entity.User) UserOutput {
	return UserOutput{ID: user.ID, Username: user.Username, Name: user.Name, Department: user.Department, Role: user.Role, OrganizationID: user.OrganizationID, RoleID: user.RoleID, PermissionIDs: user.PermissionIDs, DataScope: user.DataScope, DepartmentIDs: user.DepartmentIDs, FieldRestrictions: user.FieldRestrictions, Active: user.Active}
}
func validPassword(password string) bool {
	return len(password) >= 10 && regexp.MustCompile(`[A-Za-z]`).MatchString(password) && regexp.MustCompile(`\d`).MatchString(password)
}
func validUsername(username string) bool {
	return regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,40}$`).MatchString(username)
}

func (u *AuthUseCase) Setup(ctx context.Context, input SetupInput) (AuthOutput, error) {
	if !validUsername(input.Username) || !validPassword(input.Password) || strings.TrimSpace(input.Name) == "" {
		return AuthOutput{}, ErrValidation
	}
	var output AuthOutput
	err := u.repo.WithinTransaction(ctx, func(ctx context.Context) error {
		count, err := u.repo.CountUsers(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			return entity.NewBusinessError("conflict", "系统已初始化")
		}
		hash, err := u.passwords.Hash(input.Password)
		if err != nil {
			return err
		}
		organizationID := ""
		if organizations, ok := u.repo.(setupOrganizationRepository); ok {
			organization := entity.Organization{ID: entity.NewID(), Name: "默认组织", CreatedAt: time.Now().UTC()}
			if err := organizations.SaveOrganization(ctx, organization); err != nil {
				return err
			}
			organizationID = organization.ID
		}
		user := entity.User{ID: entity.NewID(), Username: input.Username, Name: strings.TrimSpace(input.Name), PasswordHash: hash, Role: entity.RoleAdmin, OrganizationID: organizationID, Active: true, AuthVersion: 1, CreatedAt: time.Now().UTC()}
		if err = u.repo.CreateUser(ctx, user); err != nil {
			return err
		}
		token, err := u.tokens.New()
		if err != nil {
			return err
		}
		if err = u.repo.CreateSession(ctx, token, user.ID, time.Now().Add(u.sessionTTL)); err != nil {
			return err
		}
		_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: user.ID, Action: "初始化", Module: "system", CreatedAt: time.Now().UTC()})
		output = AuthOutput{User: publicUser(user), Token: token}
		return nil
	})
	return output, err
}

func (u *AuthUseCase) SetupRequired(ctx context.Context) (bool, error) {
	count, err := u.repo.CountUsers(ctx)
	if err != nil {
		return false, err
	}
	return count == 0, nil
}

func (u *AuthUseCase) Login(ctx context.Context, input LoginInput) (AuthOutput, error) {
	if strings.TrimSpace(input.Username) == "" || input.Password == "" {
		return AuthOutput{}, ErrValidation
	}
	var output AuthOutput
	err := u.repo.WithinTransaction(ctx, func(ctx context.Context) error {
		blocked, err := u.repo.RegisterLoginFailure(ctx, input.Username, time.Now().UTC())
		if err != nil {
			return err
		}
		if blocked {
			return entity.NewBusinessError("rate_limited", "尝试次数过多，请 15 分钟后再试")
		}
		user, err := u.repo.FindUserByUsername(ctx, input.Username)
		if err != nil || !user.Active || u.passwords.Compare(user.PasswordHash, input.Password) != nil {
			return entity.NewBusinessError("unauthorized", "用户名或密码错误")
		}
		if err = u.repo.ClearLoginFailures(ctx, input.Username); err != nil {
			return err
		}
		token, err := u.tokens.New()
		if err != nil {
			return err
		}
		if err = u.repo.CreateSession(ctx, token, user.ID, time.Now().Add(u.sessionTTL)); err != nil {
			return err
		}
		_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: user.ID, Action: "登录", Module: "system", CreatedAt: time.Now().UTC()})
		output = AuthOutput{User: publicUser(user), Token: token}
		return nil
	})
	return output, err
}
func (u *AuthUseCase) LoginExternal(ctx context.Context, username string) (AuthOutput, error) {
	user, err := u.repo.FindUserByUsername(ctx, strings.TrimSpace(username))
	if err != nil || !user.Active {
		return AuthOutput{}, entity.NewBusinessError("unauthorized", "企业微信账号尚未同步或已停用")
	}
	token, err := u.tokens.New()
	if err != nil {
		return AuthOutput{}, err
	}
	if err := u.repo.CreateSession(ctx, token, user.ID, time.Now().Add(u.sessionTTL)); err != nil {
		return AuthOutput{}, err
	}
	_ = u.repo.CreateAudit(ctx, entity.Audit{ID: entity.NewID(), UserID: user.ID, Action: "企业微信登录", Module: "system", CreatedAt: time.Now().UTC()})
	return AuthOutput{User: publicUser(user), Token: token}, nil
}
func (u *AuthUseCase) CurrentUser(ctx context.Context, token string) (UserOutput, error) {
	if token == "" {
		return UserOutput{}, ErrUnauthorized
	}
	user, err := u.repo.FindSessionUser(ctx, token, time.Now().UTC())
	if err != nil || !user.Active {
		return UserOutput{}, ErrUnauthorized
	}
	return publicUser(user), nil
}
func (u *AuthUseCase) Logout(ctx context.Context, token string) error {
	return u.repo.DeleteSession(ctx, token)
}
func (u *AuthUseCase) ChangePassword(ctx context.Context, input ChangePasswordInput) (AuthOutput, error) {
	if !validPassword(input.NewPassword) {
		return AuthOutput{}, ErrValidation
	}
	var output AuthOutput
	err := u.repo.WithinTransaction(ctx, func(ctx context.Context) error {
		user, err := u.repo.FindUserByID(ctx, input.UserID)
		if err != nil {
			return err
		}
		if u.passwords.Compare(user.PasswordHash, input.CurrentPassword) != nil {
			return entity.NewBusinessError("validation", "原密码错误")
		}
		hash, err := u.passwords.Hash(input.NewPassword)
		if err != nil {
			return err
		}
		user.PasswordHash = hash
		user.AuthVersion++
		if err = u.repo.UpdateUser(ctx, user); err != nil {
			return err
		}
		if err = u.repo.DeleteUserSessions(ctx, user.ID); err != nil {
			return err
		}
		token, err := u.tokens.New()
		if err != nil {
			return err
		}
		if err = u.repo.CreateSession(ctx, token, user.ID, time.Now().Add(u.sessionTTL)); err != nil {
			return err
		}
		output = AuthOutput{User: publicUser(user), Token: token}
		return nil
	})
	return output, err
}
