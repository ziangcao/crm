package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/integration"
	"cordialcrm/backend/internal/wecom"
)

type integrationSettingMemory struct {
	items map[string]entity.IntegrationSetting
}

func (memory *integrationSettingMemory) ListIntegrationSettings(context.Context) ([]entity.IntegrationSetting, error) {
	items := make([]entity.IntegrationSetting, 0, len(memory.items))
	for _, item := range memory.items {
		items = append(items, item)
	}
	return items, nil
}

func (memory *integrationSettingMemory) FindIntegrationSetting(_ context.Context, key string) (entity.IntegrationSetting, error) {
	item, ok := memory.items[key]
	if !ok {
		return entity.IntegrationSetting{}, errors.New("not found")
	}
	return item, nil
}

func (memory *integrationSettingMemory) SaveIntegrationSetting(_ context.Context, item entity.IntegrationSetting) error {
	memory.items[item.Key] = item
	return nil
}

func (memory *integrationSettingMemory) DeleteIntegrationSetting(_ context.Context, key string) error {
	delete(memory.items, key)
	return nil
}

func TestWeComManagedConfigurationLifecycle(t *testing.T) {
	repository := &integrationSettingMemory{items: map[string]entity.IntegrationSetting{}}
	client := wecom.New("", "", "")
	defaults := WeComConfig{CorpID: "default-corp", AgentID: "100001", Secret: "default-secret", RedirectURL: "https://default.example.com/api/auth/wecom/callback"}
	service, err := NewIntegrationSetting(repository, integration.New(), client, defaults, []byte("12345678901234567890123456789012"))
	if err != nil {
		t.Fatal(err)
	}
	actor := UserOutput{Role: entity.RoleAdmin}
	managed := WeComConfig{CorpID: "managed-corp", AgentID: "100002", Secret: "managed-secret", RedirectURL: "https://crm.example.com/api/auth/wecom/callback"}
	if err := service.SaveWeCom(context.Background(), actor, managed); err != nil {
		t.Fatal(err)
	}
	stored := repository.items["wecom"]
	if strings.Contains(stored.EncryptedToken, managed.Secret) {
		t.Fatal("enterprise WeChat secret was stored in plain text")
	}
	status, err := service.WeComStatus(context.Background())
	if err != nil || !status.Configured || !status.Managed || !status.OAuth || status.CorpID != managed.CorpID || !status.HasSecret {
		t.Fatalf("unexpected managed status: %#v, %v", status, err)
	}
	if !client.Configured() || client.OAuthRedirectURL() != managed.RedirectURL {
		t.Fatal("managed configuration was not applied to the runtime client")
	}
	managed.CorpID = "updated-corp"
	managed.Secret = ""
	if err := service.SaveWeCom(context.Background(), actor, managed); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteWeCom(context.Background(), actor); err != nil {
		t.Fatal(err)
	}
	status, err = service.WeComStatus(context.Background())
	if err != nil || status.Managed || status.CorpID != defaults.CorpID || client.OAuthRedirectURL() != defaults.RedirectURL {
		t.Fatalf("environment defaults were not restored: %#v, %v", status, err)
	}
}
