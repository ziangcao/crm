package integration

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

func TestRegistryMockAndHTTPConnectors(t *testing.T) {
	registry := New(Connector{Key: "feishu", Name: "飞书", Mock: true}, Connector{Key: "tax", Name: "税务开票", Endpoint: "https://adapter.invalid/test", Token: "secret"})
	registry.httpClient.Transport = roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("Authorization") != "Bearer secret" || request.Method != http.MethodPost {
			t.Errorf("unexpected request: %#v", request)
		}
		return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(""))}, nil
	})
	if err := registry.Test(context.Background(), "feishu"); err != nil {
		t.Fatalf("mock connector: %v", err)
	}
	if err := registry.Test(context.Background(), "tax"); err != nil {
		t.Fatalf("HTTP connector: %v", err)
	}
	if err := registry.Test(context.Background(), "bank"); err == nil {
		t.Fatal("expected unconfigured connector rejection")
	}
	statuses := registry.Statuses()
	if len(statuses) != 6 || !statuses[0].Configured || !statuses[0].Mock {
		t.Fatalf("unexpected statuses: %#v", statuses)
	}
}
