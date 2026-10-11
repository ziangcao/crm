package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Address, Driver, DSN, RedisURL, RedisPrefix, Frontend, UploadDir       string
	GitHubImageOwner, GitHubImageRepo, GitHubImageBranch, GitHubImageToken string
	WeComCorpID, WeComAgentID, WeComSecret, WeComOAuthRedirect             string
	CookieSecure                                                           bool
	AllowedOrigins                                                         []string
	SessionTTL                                                             time.Duration
	CustomerRecycleDays                                                    int
	IntegrationMock                                                        bool
	IntegrationEndpoints, IntegrationTokens                                map[string]string
	ConfigEncryptionKey                                                    string
}

func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return Config{}, fmt.Errorf("无法读取 .env")
	}
	port, err := strconv.Atoi(env("CRM_PORT", "8000"))
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("CRM_PORT 无效")
	}
	recycleDays, err := strconv.Atoi(env("CRM_CUSTOMER_RECYCLE_DAYS", "0"))
	if err != nil || recycleDays < 0 || recycleDays > 3650 {
		return Config{}, fmt.Errorf("CRM_CUSTOMER_RECYCLE_DAYS 无效")
	}
	c := Config{Address: net.JoinHostPort(env("CRM_HOST", "127.0.0.1"), strconv.Itoa(port)), Driver: env("CRM_DB_DRIVER", "postgres"), DSN: os.Getenv("CRM_DB_DSN"), RedisURL: env("CRM_REDIS_URL", "redis://127.0.0.1:6379/0"), RedisPrefix: env("CRM_REDIS_PREFIX", "cordial:"), Frontend: env("CRM_FRONTEND", "frontend/dist"), UploadDir: env("CRM_UPLOAD_DIR", "backend/data/uploads"), WeComCorpID: os.Getenv("CRM_WECOM_CORP_ID"), WeComAgentID: os.Getenv("CRM_WECOM_AGENT_ID"), WeComSecret: os.Getenv("CRM_WECOM_SECRET"), WeComOAuthRedirect: os.Getenv("CRM_WECOM_OAUTH_REDIRECT_URL"), CookieSecure: os.Getenv("CRM_COOKIE_SECURE") == "1", SessionTTL: 12 * time.Hour, CustomerRecycleDays: recycleDays, ConfigEncryptionKey: os.Getenv("CRM_CONFIG_ENCRYPTION_KEY")}
	c.GitHubImageOwner = os.Getenv("CRM_GITHUB_IMAGE_OWNER")
	c.GitHubImageRepo = os.Getenv("CRM_GITHUB_IMAGE_REPO")
	c.GitHubImageBranch = env("CRM_GITHUB_IMAGE_BRANCH", "main")
	c.GitHubImageToken = os.Getenv("CRM_GITHUB_IMAGE_TOKEN")
	if c.GitHubImageOwner != "" || c.GitHubImageRepo != "" || c.GitHubImageToken != "" {
		if c.GitHubImageOwner == "" || c.GitHubImageRepo == "" || c.GitHubImageToken == "" {
			return c, fmt.Errorf("GitHub 图片仓库配置需要同时提供 owner、repo 和 token")
		}
	}
	c.IntegrationMock = os.Getenv("CRM_INTEGRATION_MOCK") == "1"
	c.IntegrationEndpoints = map[string]string{"feishu": os.Getenv("CRM_FEISHU_WEBHOOK_URL"), "dingtalk": os.Getenv("CRM_DINGTALK_WEBHOOK_URL"), "sso": os.Getenv("CRM_SSO_DISCOVERY_URL"), "tax": os.Getenv("CRM_TAX_ADAPTER_URL"), "bank": os.Getenv("CRM_BANK_ADAPTER_URL"), "tender": os.Getenv("CRM_TENDER_ADAPTER_URL")}
	c.IntegrationTokens = map[string]string{"feishu": os.Getenv("CRM_FEISHU_TOKEN"), "dingtalk": os.Getenv("CRM_DINGTALK_TOKEN"), "sso": os.Getenv("CRM_SSO_CLIENT_SECRET"), "tax": os.Getenv("CRM_TAX_ADAPTER_TOKEN"), "bank": os.Getenv("CRM_BANK_ADAPTER_TOKEN"), "tender": os.Getenv("CRM_TENDER_ADAPTER_TOKEN")}
	if c.Driver != "postgres" && c.Driver != "mysql" {
		return c, fmt.Errorf("CRM_DB_DRIVER 仅支持 postgres 或 mysql")
	}
	if c.DSN == "" {
		return c, fmt.Errorf("请配置 CRM_DB_DSN（参见 .env.example）")
	}
	weComConfigured := c.WeComCorpID != "" || c.WeComAgentID != "" || c.WeComSecret != ""
	if weComConfigured && (c.WeComCorpID == "" || c.WeComAgentID == "" || c.WeComSecret == "") {
		return c, fmt.Errorf("企业微信配置必须同时提供 CRM_WECOM_CORP_ID、CRM_WECOM_AGENT_ID 和 CRM_WECOM_SECRET")
	}
	if weComConfigured {
		agentID, err := strconv.Atoi(c.WeComAgentID)
		if err != nil || agentID < 1 {
			return c, fmt.Errorf("CRM_WECOM_AGENT_ID 无效")
		}
	}
	for _, origin := range strings.Split(os.Getenv("CRM_ALLOWED_ORIGINS"), ",") {
		if origin = strings.TrimSpace(origin); origin != "" {
			c.AllowedOrigins = append(c.AllowedOrigins, origin)
		}
	}
	return c, nil
}
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
