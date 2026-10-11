package wecom

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestOAuthAndDirectoryProtocol(t *testing.T) {
	client := New("corp", "100001", "secret")
	client.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body := `{"errcode":0,"access_token":"token","expires_in":7200}`
		switch {
		case strings.HasSuffix(request.URL.Path, "/user/getuserinfo"):
			if request.URL.Query().Get("code") != "oauth-code" || request.URL.Query().Get("access_token") != "token" {
				body = `{"errcode":40029,"errmsg":"invalid code"}`
			} else {
				body = `{"errcode":0,"UserId":"zhangsan"}`
			}
		case strings.HasSuffix(request.URL.Path, "/user/simplelist"):
			body = `{"errcode":0,"userlist":[{"userid":"zhangsan","name":"张三"}]}`
		}
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	authorizationURL := client.AuthorizationURL("https://crm.example.com/api/auth/wecom/callback", "state-value")
	if !strings.Contains(authorizationURL, "appid=corp") || !strings.Contains(authorizationURL, "state=state-value") || !strings.Contains(authorizationURL, "snsapi_base") {
		t.Fatalf("unexpected authorization URL: %s", authorizationURL)
	}
	userID, err := client.UserIDByCode(context.Background(), "oauth-code")
	if err != nil || userID != "zhangsan" {
		t.Fatalf("unexpected OAuth user: %s, %v", userID, err)
	}
	users, err := client.ListUsers(context.Background())
	if err != nil || len(users) != 1 || users[0].UserID != "zhangsan" || users[0].Name != "张三" {
		t.Fatalf("unexpected directory users: %#v, %v", users, err)
	}
}
