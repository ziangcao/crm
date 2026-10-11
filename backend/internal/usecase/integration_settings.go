package usecase

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"cordialcrm/backend/internal/entity"
	"cordialcrm/backend/internal/integration"
	"cordialcrm/backend/internal/wecom"
)

var IntegrationKeys = []string{"feishu", "dingtalk", "sso", "tax", "bank", "tender"}

type IntegrationSettingRepository interface {
	ListIntegrationSettings(context.Context) ([]entity.IntegrationSetting, error)
	FindIntegrationSetting(context.Context, string) (entity.IntegrationSetting, error)
	SaveIntegrationSetting(context.Context, entity.IntegrationSetting) error
	DeleteIntegrationSetting(context.Context, string) error
}

type IntegrationSettingUseCase struct {
	repository   IntegrationSettingRepository
	registry     *integration.Registry
	weCom        *wecom.Client
	weComDefault WeComConfig
	aead         cipher.AEAD
}

type WeComConfig struct {
	CorpID      string `json:"corp_id"`
	AgentID     string `json:"agent_id"`
	Secret      string `json:"secret,omitempty"`
	RedirectURL string `json:"redirect_url"`
}

type WeComStatus struct {
	Configured  bool   `json:"configured"`
	OAuth       bool   `json:"oauth"`
	Managed     bool   `json:"managed"`
	CorpID      string `json:"corp_id"`
	AgentID     string `json:"agent_id"`
	RedirectURL string `json:"redirect_url"`
	HasSecret   bool   `json:"has_secret"`
}

func NewIntegrationSetting(repository IntegrationSettingRepository, registry *integration.Registry, weCom *wecom.Client, weComDefault WeComConfig, encryptionKey []byte) (*IntegrationSettingUseCase, error) {
	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &IntegrationSettingUseCase{repository: repository, registry: registry, weCom: weCom, weComDefault: weComDefault, aead: aead}, nil
}

func (u *IntegrationSettingUseCase) Load(ctx context.Context) error {
	items, err := u.repository.ListIntegrationSettings(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		token, err := u.decrypt(item.EncryptedToken)
		if err != nil {
			return err
		}
		if item.Key == "wecom" {
			var config WeComConfig
			if err := json.Unmarshal([]byte(token), &config); err != nil {
				return err
			}
			config.RedirectURL = item.Endpoint
			u.applyWeCom(config)
			continue
		}
		if err := u.registry.Set(integration.Connector{Key: item.Key, Endpoint: item.Endpoint, Token: token}); err != nil {
			return err
		}
	}
	return nil
}

func (u *IntegrationSettingUseCase) WeComStatus(ctx context.Context) (WeComStatus, error) {
	config := u.weComDefault
	managed := false
	item, err := u.repository.FindIntegrationSetting(ctx, "wecom")
	if err == nil {
		plain, decryptErr := u.decrypt(item.EncryptedToken)
		if decryptErr != nil {
			return WeComStatus{}, decryptErr
		}
		if unmarshalErr := json.Unmarshal([]byte(plain), &config); unmarshalErr != nil {
			return WeComStatus{}, unmarshalErr
		}
		config.RedirectURL = item.Endpoint
		managed = true
	}
	configured := config.CorpID != "" && config.AgentID != "" && config.Secret != ""
	return WeComStatus{Configured: configured, OAuth: configured && config.RedirectURL != "", Managed: managed, CorpID: config.CorpID, AgentID: config.AgentID, RedirectURL: config.RedirectURL, HasSecret: config.Secret != ""}, nil
}

func (u *IntegrationSettingUseCase) SaveWeCom(ctx context.Context, actor UserOutput, config WeComConfig) error {
	existing, findErr := u.repository.FindIntegrationSetting(ctx, "wecom")
	action := "UPDATE"
	if findErr != nil {
		action = "ADD"
	}
	if err := requirePermission(actor, "SYSTEM_SETTING:"+action); err != nil {
		return err
	}
	config.CorpID = strings.TrimSpace(config.CorpID)
	config.AgentID = strings.TrimSpace(config.AgentID)
	config.Secret = strings.TrimSpace(config.Secret)
	config.RedirectURL = strings.TrimSpace(config.RedirectURL)
	if config.Secret == "" && action == "UPDATE" {
		plain, err := u.decrypt(existing.EncryptedToken)
		if err != nil {
			return err
		}
		var current WeComConfig
		if err := json.Unmarshal([]byte(plain), &current); err != nil {
			return err
		}
		config.Secret = current.Secret
	}
	if config.CorpID == "" || config.AgentID == "" || config.Secret == "" {
		return ErrValidation
	}
	agentID, err := strconv.Atoi(config.AgentID)
	if err != nil || agentID < 1 {
		return ErrValidation
	}
	if config.RedirectURL != "" {
		parsed, err := url.ParseRequestURI(config.RedirectURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return ErrValidation
		}
	}
	secretPayload, err := json.Marshal(WeComConfig{CorpID: config.CorpID, AgentID: config.AgentID, Secret: config.Secret})
	if err != nil {
		return err
	}
	encrypted, err := u.encrypt(string(secretPayload))
	if err != nil {
		return err
	}
	if err := u.repository.SaveIntegrationSetting(ctx, entity.IntegrationSetting{Key: "wecom", Endpoint: config.RedirectURL, EncryptedToken: encrypted, UpdatedAt: time.Now().UTC()}); err != nil {
		return err
	}
	u.applyWeCom(config)
	return nil
}

func (u *IntegrationSettingUseCase) DeleteWeCom(ctx context.Context, actor UserOutput) error {
	if err := requirePermission(actor, "SYSTEM_SETTING:DELETE"); err != nil {
		return err
	}
	if err := u.repository.DeleteIntegrationSetting(ctx, "wecom"); err != nil {
		return err
	}
	u.applyWeCom(u.weComDefault)
	return nil
}

func (u *IntegrationSettingUseCase) TestWeCom(ctx context.Context, actor UserOutput) error {
	if err := requirePermission(actor, "SYSTEM_SETTING:UPDATE"); err != nil {
		return err
	}
	if u.weCom == nil || !u.weCom.Configured() {
		return entity.NewBusinessError("validation", "企业微信未配置")
	}
	if err := u.weCom.Test(ctx); err != nil {
		return entity.NewBusinessError("validation", err.Error())
	}
	return nil
}

func (u *IntegrationSettingUseCase) applyWeCom(config WeComConfig) {
	if u.weCom != nil {
		u.weCom.Configure(config.CorpID, config.AgentID, config.Secret, config.RedirectURL)
	}
}

func (u *IntegrationSettingUseCase) Save(ctx context.Context, actor UserOutput, key, endpoint, token string) error {
	_, err := u.repository.FindIntegrationSetting(ctx, key)
	action := "UPDATE"
	if err != nil {
		action = "ADD"
	}
	if err := requirePermission(actor, "SYSTEM_SETTING:"+action); err != nil {
		return err
	}
	endpoint = strings.TrimSpace(endpoint)
	parsed, err := url.ParseRequestURI(endpoint)
	if !slices.Contains(IntegrationKeys, key) || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrValidation
	}
	encryptedToken := ""
	if token != "" {
		encryptedToken, err = u.encrypt(token)
		if err != nil {
			return err
		}
	} else if action == "UPDATE" {
		existing, findErr := u.repository.FindIntegrationSetting(ctx, key)
		if findErr != nil {
			return findErr
		}
		encryptedToken = existing.EncryptedToken
		token, err = u.decrypt(encryptedToken)
		if err != nil {
			return err
		}
	}
	item := entity.IntegrationSetting{Key: key, Endpoint: endpoint, EncryptedToken: encryptedToken, UpdatedAt: time.Now().UTC()}
	if err := u.repository.SaveIntegrationSetting(ctx, item); err != nil {
		return err
	}
	return u.registry.Set(integration.Connector{Key: key, Endpoint: endpoint, Token: token})
}

func (u *IntegrationSettingUseCase) Delete(ctx context.Context, actor UserOutput, key string) error {
	if err := requirePermission(actor, "SYSTEM_SETTING:DELETE"); err != nil {
		return err
	}
	if !slices.Contains(IntegrationKeys, key) {
		return ErrValidation
	}
	if err := u.repository.DeleteIntegrationSetting(ctx, key); err != nil {
		return err
	}
	return u.registry.Reset(key)
}

func (u *IntegrationSettingUseCase) encrypt(value string) (string, error) {
	nonce := make([]byte, u.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(append(nonce, u.aead.Seal(nil, nonce, []byte(value), nil)...)), nil
}

func (u *IntegrationSettingUseCase) decrypt(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	data, err := base64.RawStdEncoding.DecodeString(value)
	if err != nil || len(data) < u.aead.NonceSize() {
		return "", ErrValidation
	}
	plain, err := u.aead.Open(nil, data[:u.aead.NonceSize()], data[u.aead.NonceSize():], nil)
	return string(plain), err
}
