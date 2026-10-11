package usecase

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cordialcrm/backend/internal/entity"
	"golang.org/x/crypto/bcrypt"
)

type apiKeyTestRepository struct {
	items []entity.APIKey
	user  entity.User
}

type sessionRoleTestRepository struct {
	Repository
	user entity.User
}

func (repository *sessionRoleTestRepository) FindSessionUser(_ context.Context, _ string, _ time.Time) (entity.User, error) {
	return repository.user, nil
}

func (r *apiKeyTestRepository) CreateAPIKey(_ context.Context, item entity.APIKey) error {
	r.items = append(r.items, item)
	return nil
}
func (r *apiKeyTestRepository) ListAPIKeys(_ context.Context, userID string) ([]entity.APIKey, error) {
	items := make([]entity.APIKey, 0, len(r.items))
	for _, item := range r.items {
		if item.UserID == userID {
			items = append(items, item)
		}
	}
	return items, nil
}
func (r *apiKeyTestRepository) FindAPIKeyByPrefix(_ context.Context, prefix string) (entity.APIKey, error) {
	for _, item := range r.items {
		if item.Prefix == prefix {
			return item, nil
		}
	}
	return entity.APIKey{}, errors.New("not found")
}
func (r *apiKeyTestRepository) UpdateAPIKey(_ context.Context, item entity.APIKey) error {
	for index := range r.items {
		if r.items[index].ID == item.ID {
			r.items[index] = item
			return nil
		}
	}
	return errors.New("not found")
}
func (r *apiKeyTestRepository) DeleteAPIKey(_ context.Context, id, userID string) error {
	for index, item := range r.items {
		if item.ID == id && item.UserID == userID {
			r.items = append(r.items[:index], r.items[index+1:]...)
			return nil
		}
	}
	return errors.New("not found")
}
func (r *apiKeyTestRepository) FindUserByID(_ context.Context, id string) (entity.User, error) {
	if r.user.ID == id {
		return r.user, nil
	}
	return entity.User{}, errors.New("not found")
}

type apiKeyTestHasher struct{}

func (apiKeyTestHasher) Hash(value string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(value), bcrypt.MinCost)
	return string(bytes), err
}
func (apiKeyTestHasher) Compare(hash, value string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(value))
}

type apiKeyTestTokens struct{ count int }

func (t *apiKeyTestTokens) New() (string, error) {
	t.count++
	return fmt.Sprintf("token-%040d", t.count), nil
}

func TestAPIKeyAuthenticateRespectsActiveState(t *testing.T) {
	repository := &apiKeyTestRepository{user: entity.User{ID: "user-1", Username: "seller", Name: "销售", Role: entity.RoleSales, Active: true}}
	tokens := &apiKeyTestTokens{}
	service := NewAPIKey(repository, apiKeyTestHasher{}, tokens)
	actor := UserOutput{ID: "user-1", Role: entity.RoleSales}
	created, err := service.Create(context.Background(), CreateAPIKeyInput{Actor: actor, Name: "agent"})
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if created.Secret == "" || created.Prefix == "" {
		t.Fatal("expected secret and prefix")
	}
	authenticated, err := service.Authenticate(context.Background(), AuthenticateAPIKeyInput{Secret: created.Secret})
	if err != nil || authenticated.ID != actor.ID {
		t.Fatalf("authenticate key: %#v, %v", authenticated, err)
	}
	if _, err = service.Update(context.Background(), UpdateAPIKeyInput{Actor: actor, ID: created.ID, Name: "agent", Active: false}); err != nil {
		t.Fatalf("disable key: %v", err)
	}
	if _, err = service.Authenticate(context.Background(), AuthenticateAPIKeyInput{Secret: created.Secret}); err != ErrUnauthorized {
		t.Fatalf("expected disabled key to be unauthorized, got %v", err)
	}
}

func TestCookieAndAPIKeyUseCurrentRolePermissions(t *testing.T) {
	user := entity.User{ID: "seller", Role: entity.RoleSales, OrganizationID: "org-a", RoleID: "role-a", PermissionIDs: []string{"PERSONAL_API_KEY:ADD", "customers.read"}, Active: true}
	sessions := &sessionRoleTestRepository{user: user}
	cookieAuth := NewAuth(sessions, nil, nil, time.Hour)
	keys := &apiKeyTestRepository{user: user}
	keyAuth := NewAPIKey(keys, apiKeyTestHasher{}, &apiKeyTestTokens{})
	created, err := keyAuth.Create(context.Background(), CreateAPIKeyInput{Actor: publicUser(user), Name: "agent"})
	if err != nil {
		t.Fatal(err)
	}
	for _, permissions := range [][]string{{"PERSONAL_API_KEY:ADD", "customers.read"}, {"PERSONAL_API_KEY:ADD", "customers.write"}} {
		sessions.user.PermissionIDs = permissions
		keys.user.PermissionIDs = permissions
		cookieActor, err := cookieAuth.CurrentUser(context.Background(), "session")
		if err != nil {
			t.Fatal(err)
		}
		keyActor, err := keyAuth.Authenticate(context.Background(), AuthenticateAPIKeyInput{Secret: created.Secret})
		if err != nil {
			t.Fatal(err)
		}
		if len(cookieActor.PermissionIDs) != len(permissions) || cookieActor.PermissionIDs[1] != permissions[1] || len(keyActor.PermissionIDs) != len(permissions) || keyActor.PermissionIDs[1] != permissions[1] {
			t.Fatalf("stale permissions: cookie=%v key=%v", cookieActor.PermissionIDs, keyActor.PermissionIDs)
		}
	}
}

func TestAPIKeyCreateLimitAndExpiryValidation(t *testing.T) {
	repository := &apiKeyTestRepository{user: entity.User{ID: "user-1", Active: true}}
	service := NewAPIKey(repository, apiKeyTestHasher{}, &apiKeyTestTokens{})
	actor := UserOutput{ID: "user-1"}
	for index := 0; index < maxAPIKeysPerUser; index++ {
		if _, err := service.Create(context.Background(), CreateAPIKeyInput{Actor: actor, Name: fmt.Sprintf("key-%d", index)}); err != nil {
			t.Fatalf("create key %d: %v", index, err)
		}
	}
	if _, err := service.Create(context.Background(), CreateAPIKeyInput{Actor: actor, Name: "too-many"}); err == nil {
		t.Fatal("expected key limit error")
	}
	past := time.Now().UTC().Add(-time.Minute)
	if _, err := NewAPIKey(&apiKeyTestRepository{}, apiKeyTestHasher{}, &apiKeyTestTokens{}).Create(context.Background(), CreateAPIKeyInput{Actor: actor, Name: "expired", ExpiresAt: &past}); err != ErrValidation {
		t.Fatalf("expected expiry validation error, got %v", err)
	}
}
