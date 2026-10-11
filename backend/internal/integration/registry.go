package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Connector struct {
	Key, Name, Endpoint, Token string
	Mock                       bool
}

type Status struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
	Mock       bool   `json:"mock"`
	Endpoint   string `json:"endpoint"`
	HasToken   bool   `json:"has_token"`
	Managed    bool   `json:"managed"`
}

type Registry struct {
	connectors map[string]Connector
	defaults   map[string]Connector
	managed    map[string]bool
	httpClient *http.Client
	mutex      sync.RWMutex
}

func New(connectors ...Connector) *Registry {
	items := make(map[string]Connector, len(connectors))
	for _, connector := range connectors {
		items[connector.Key] = connector
	}
	defaults := make(map[string]Connector, len(items))
	for key, connector := range items {
		defaults[key] = connector
	}
	return &Registry{connectors: items, defaults: defaults, managed: map[string]bool{}, httpClient: &http.Client{Timeout: 8 * time.Second}}
}

func (r *Registry) Statuses() []Status {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	keys := []string{"feishu", "dingtalk", "sso", "tax", "bank", "tender"}
	items := make([]Status, 0, len(keys))
	for _, key := range keys {
		connector := r.connectors[key]
		items = append(items, Status{Key: key, Name: connector.Name, Configured: connector.Mock || strings.TrimSpace(connector.Endpoint) != "", Mock: connector.Mock, Endpoint: connector.Endpoint, HasToken: connector.Token != "", Managed: r.managed[key]})
	}
	return items
}

func (r *Registry) Test(ctx context.Context, key string) error {
	r.mutex.RLock()
	connector, found := r.connectors[key]
	r.mutex.RUnlock()
	if !found || (!connector.Mock && strings.TrimSpace(connector.Endpoint) == "") {
		return fmt.Errorf("集成未配置")
	}
	if connector.Mock {
		return nil
	}
	body, _ := json.Marshal(map[string]any{"event": "crm.connection.test", "timestamp": time.Now().UTC().Format(time.RFC3339)})
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, connector.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	if connector.Token != "" {
		request.Header.Set("Authorization", "Bearer "+connector.Token)
	}
	response, err := r.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("集成端点返回 HTTP %d", response.StatusCode)
	}
	return nil
}

func (r *Registry) Set(connector Connector) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	current, found := r.connectors[connector.Key]
	if !found {
		return fmt.Errorf("不支持的集成类型")
	}
	connector.Name = current.Name
	r.connectors[connector.Key] = connector
	r.managed[connector.Key] = true
	return nil
}

func (r *Registry) Reset(key string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	connector, found := r.defaults[key]
	if !found {
		return fmt.Errorf("不支持的集成类型")
	}
	r.connectors[key] = connector
	delete(r.managed, key)
	return nil
}
