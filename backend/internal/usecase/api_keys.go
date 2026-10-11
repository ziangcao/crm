package usecase

import (
	"context"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
)

const maxAPIKeysPerUser = 5

type APIKeyUseCase struct {
	repository APIKeyRepository
	passwords  PasswordHasher
	tokens     TokenGenerator
}

type APIKeyOutput struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Prefix     string     `json:"prefix"`
	Active     bool       `json:"active"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type ListAPIKeysInput struct{ Actor UserOutput }
type ListAPIKeysOutput struct {
	Items []APIKeyOutput `json:"items"`
}
type CreateAPIKeyInput struct {
	Actor     UserOutput
	Name      string
	ExpiresAt *time.Time
}
type CreateAPIKeyOutput struct {
	APIKeyOutput
	Secret string `json:"secret"`
}
type UpdateAPIKeyInput struct {
	Actor     UserOutput
	ID        string
	Name      string
	Active    bool
	ExpiresAt *time.Time
}
type DeleteAPIKeyInput struct {
	Actor UserOutput
	ID    string
}
type AuthenticateAPIKeyInput struct{ Secret string }

func NewAPIKey(repository APIKeyRepository, passwords PasswordHasher, tokens TokenGenerator) *APIKeyUseCase {
	return &APIKeyUseCase{repository: repository, passwords: passwords, tokens: tokens}
}

func (u *APIKeyUseCase) List(ctx context.Context, input ListAPIKeysInput) (ListAPIKeysOutput, error) {
	if err := requirePermission(input.Actor, "PERSONAL_API_KEY:READ"); err != nil {
		return ListAPIKeysOutput{}, err
	}
	items, err := u.repository.ListAPIKeys(ctx, input.Actor.ID)
	if err != nil {
		return ListAPIKeysOutput{}, err
	}
	output := make([]APIKeyOutput, len(items))
	for index, item := range items {
		output[index] = apiKeyOutput(item)
	}
	return ListAPIKeysOutput{Items: output}, nil
}

func (u *APIKeyUseCase) Create(ctx context.Context, input CreateAPIKeyInput) (CreateAPIKeyOutput, error) {
	if err := requirePermission(input.Actor, "PERSONAL_API_KEY:ADD"); err != nil {
		return CreateAPIKeyOutput{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 100 || (input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC())) {
		return CreateAPIKeyOutput{}, ErrValidation
	}
	items, err := u.repository.ListAPIKeys(ctx, input.Actor.ID)
	if err != nil {
		return CreateAPIKeyOutput{}, err
	}
	if len(items) >= maxAPIKeysPerUser {
		return CreateAPIKeyOutput{}, entity.NewBusinessError("conflict", "每个用户最多创建 5 个 API Key")
	}
	secret, err := u.tokens.New()
	if err != nil {
		return CreateAPIKeyOutput{}, err
	}
	secret = "ccrm_" + secret
	hash, err := u.passwords.Hash(secret)
	if err != nil {
		return CreateAPIKeyOutput{}, err
	}
	now := time.Now().UTC()
	item := entity.APIKey{ID: entity.NewID(), UserID: input.Actor.ID, Name: name, Prefix: secret[:17], SecretHash: hash, Active: true, CreatedAt: now, ExpiresAt: input.ExpiresAt}
	if err := u.repository.CreateAPIKey(ctx, item); err != nil {
		return CreateAPIKeyOutput{}, err
	}
	return CreateAPIKeyOutput{APIKeyOutput: apiKeyOutput(item), Secret: secret}, nil
}

func (u *APIKeyUseCase) Update(ctx context.Context, input UpdateAPIKeyInput) (APIKeyOutput, error) {
	if err := requirePermission(input.Actor, "PERSONAL_API_KEY:UPDATE"); err != nil {
		return APIKeyOutput{}, err
	}
	if input.Actor.ID == "" || input.ID == "" {
		return APIKeyOutput{}, ErrValidation
	}
	items, err := u.repository.ListAPIKeys(ctx, input.Actor.ID)
	if err != nil {
		return APIKeyOutput{}, err
	}
	for _, item := range items {
		if item.ID != input.ID {
			continue
		}
		name := strings.TrimSpace(input.Name)
		if name == "" || len([]rune(name)) > 100 || (input.ExpiresAt != nil && !input.ExpiresAt.After(time.Now().UTC())) {
			return APIKeyOutput{}, ErrValidation
		}
		item.Name, item.Active, item.ExpiresAt = name, input.Active, input.ExpiresAt
		if err := u.repository.UpdateAPIKey(ctx, item); err != nil {
			return APIKeyOutput{}, err
		}
		return apiKeyOutput(item), nil
	}
	return APIKeyOutput{}, ErrNotFound
}

func (u *APIKeyUseCase) Delete(ctx context.Context, input DeleteAPIKeyInput) error {
	if err := requirePermission(input.Actor, "PERSONAL_API_KEY:DELETE"); err != nil {
		return err
	}
	if input.Actor.ID == "" || input.ID == "" {
		return ErrValidation
	}
	items, err := u.repository.ListAPIKeys(ctx, input.Actor.ID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ID == input.ID {
			return u.repository.DeleteAPIKey(ctx, input.ID, input.Actor.ID)
		}
	}
	return ErrNotFound
}

func (u *APIKeyUseCase) Authenticate(ctx context.Context, input AuthenticateAPIKeyInput) (UserOutput, error) {
	secret := strings.TrimSpace(input.Secret)
	if !strings.HasPrefix(secret, "ccrm_") || len(secret) < 18 {
		return UserOutput{}, ErrUnauthorized
	}
	item, err := u.repository.FindAPIKeyByPrefix(ctx, secret[:17])
	if err != nil || !item.Active || (item.ExpiresAt != nil && !item.ExpiresAt.After(time.Now().UTC())) || u.passwords.Compare(item.SecretHash, secret) != nil {
		return UserOutput{}, ErrUnauthorized
	}
	user, err := u.repository.FindUserByID(ctx, item.UserID)
	if err != nil || !user.Active {
		return UserOutput{}, ErrUnauthorized
	}
	now := time.Now().UTC()
	item.LastUsedAt = &now
	_ = u.repository.UpdateAPIKey(ctx, item)
	return publicUser(user), nil
}

func apiKeyOutput(item entity.APIKey) APIKeyOutput {
	return APIKeyOutput{ID: item.ID, Name: item.Name, Prefix: item.Prefix, Active: item.Active, CreatedAt: item.CreatedAt, ExpiresAt: item.ExpiresAt, LastUsedAt: item.LastUsedAt}
}
