package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const apiBaseURL = "https://qyapi.weixin.qq.com/cgi-bin"

type Client struct {
	corpID, agentID, secret, redirectURL string
	apiBaseURL                           string
	httpClient                           *http.Client
	mu                                   sync.Mutex
	token                                string
	tokenExpiresAt                       time.Time
}

func New(corpID, agentID, secret string) *Client {
	return &Client{corpID: corpID, agentID: agentID, secret: secret, apiBaseURL: apiBaseURL, httpClient: &http.Client{Timeout: 5 * time.Second}}
}

func (c *Client) Configure(corpID, agentID, secret, redirectURL string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.corpID = strings.TrimSpace(corpID)
	c.agentID = strings.TrimSpace(agentID)
	c.secret = strings.TrimSpace(secret)
	c.redirectURL = strings.TrimSpace(redirectURL)
	c.token = ""
	c.tokenExpiresAt = time.Time{}
}

func (c *Client) Configured() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.corpID != "" && c.agentID != "" && c.secret != ""
}

func (c *Client) OAuthRedirectURL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.redirectURL
}

func (c *Client) Test(ctx context.Context) error {
	_, err := c.accessToken(ctx)
	return err
}

type DirectoryUser struct {
	UserID string `json:"userid"`
	Name   string `json:"name"`
}

func (c *Client) AuthorizationURL(redirectURL, state string) string {
	c.mu.Lock()
	corpID, agentID := c.corpID, c.agentID
	c.mu.Unlock()
	query := url.Values{"appid": {corpID}, "redirect_uri": {redirectURL}, "response_type": {"code"}, "scope": {"snsapi_base"}, "state": {state}, "agentid": {agentID}}
	return "https://open.weixin.qq.com/connect/oauth2/authorize?" + query.Encode() + "#wechat_redirect"
}

func (c *Client) UserIDByCode(ctx context.Context, code string) (string, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return "", err
	}
	var result struct {
		ErrorCode int    `json:"errcode"`
		ErrorText string `json:"errmsg"`
		UserID    string `json:"UserId"`
	}
	if err := c.get(ctx, "/user/getuserinfo?access_token="+url.QueryEscape(token)+"&code="+url.QueryEscape(code), &result); err != nil {
		return "", err
	}
	if result.ErrorCode != 0 || result.UserID == "" {
		return "", fmt.Errorf("企业微信 OAuth 失败: %s", result.ErrorText)
	}
	return result.UserID, nil
}

func (c *Client) ListUsers(ctx context.Context) ([]DirectoryUser, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	var result struct {
		ErrorCode int             `json:"errcode"`
		ErrorText string          `json:"errmsg"`
		Users     []DirectoryUser `json:"userlist"`
	}
	if err := c.get(ctx, "/user/simplelist?access_token="+url.QueryEscape(token)+"&department_id=1&fetch_child=1", &result); err != nil {
		return nil, err
	}
	if result.ErrorCode != 0 {
		return nil, fmt.Errorf("企业微信通讯录同步失败: %s", result.ErrorText)
	}
	return result.Users, nil
}

func (c *Client) get(ctx context.Context, path string, result any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+path, nil)
	if err != nil {
		return err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("企业微信接口返回 HTTP %d", response.StatusCode)
	}
	return json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(result)
}

func (c *Client) Send(ctx context.Context, recipient, subject, content string) error {
	if strings.TrimSpace(recipient) == "" {
		return nil
	}
	if !c.Configured() {
		return nil
	}
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	payload := struct {
		ToUser  string `json:"touser"`
		MsgType string `json:"msgtype"`
		AgentID int    `json:"agentid"`
		Text    struct {
			Content string `json:"content"`
		} `json:"text"`
	}{ToUser: recipient, MsgType: "text"}
	c.mu.Lock()
	agentID := c.agentID
	c.mu.Unlock()
	payload.AgentID, _ = strconv.Atoi(agentID)
	payload.Text.Content = truncate(subject+"\n"+content, 2048)
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBaseURL+"/message/send?access_token="+url.QueryEscape(token), bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return responseError(response)
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.corpID == "" || c.agentID == "" || c.secret == "" {
		return "", fmt.Errorf("企业微信未配置")
	}
	if c.token != "" && time.Now().Before(c.tokenExpiresAt) {
		return c.token, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBaseURL+"/gettoken?corpid="+url.QueryEscape(c.corpID)+"&corpsecret="+url.QueryEscape(c.secret), nil)
	if err != nil {
		return "", err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var result struct {
		ErrorCode int    `json:"errcode"`
		ErrorText string `json:"errmsg"`
		Token     string `json:"access_token"`
		ExpiresIn int    `json:"expires_in"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return "", err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.ErrorCode != 0 || result.Token == "" {
		return "", fmt.Errorf("企业微信获取 access_token 失败: %s", result.ErrorText)
	}
	c.token = result.Token
	c.tokenExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return c.token, nil
}

func responseError(response *http.Response) error {
	var result struct {
		ErrorCode int    `json:"errcode"`
		ErrorText string `json:"errmsg"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 || result.ErrorCode != 0 {
		return fmt.Errorf("企业微信发送消息失败: %s", result.ErrorText)
	}
	return nil
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
