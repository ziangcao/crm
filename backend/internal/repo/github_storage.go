package repo

import (
	"bytes"
	"container/list"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const githubImageCacheLimit = 32 << 20
const githubImageCacheTTL = 30 * time.Minute

type githubImageCacheEntry struct {
	id      string
	content []byte
	expires time.Time
}

// GitHubImageStorage keeps product images in a private repository. The CRM API
// continues to authorize every image read before calling this storage.
type GitHubImageStorage struct {
	owner, repository, branch, token string
	client                           *http.Client
	cacheMu                          sync.Mutex
	cache                            map[string]*list.Element
	cacheOrder                       *list.List
	cacheBytes                       int
}

func NewGitHubImageStorage(owner, repository, branch, token string) *GitHubImageStorage {
	return &GitHubImageStorage{owner: owner, repository: repository, branch: branch, token: token, client: &http.Client{Timeout: 25 * time.Second}, cache: make(map[string]*list.Element), cacheOrder: list.New()}
}

func (s *GitHubImageStorage) cachedImage(id string) ([]byte, bool) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	element, ok := s.cache[id]
	if !ok {
		return nil, false
	}
	entry := element.Value.(*githubImageCacheEntry)
	if time.Now().After(entry.expires) {
		s.removeCachedImage(element)
		return nil, false
	}
	s.cacheOrder.MoveToFront(element)
	return bytes.Clone(entry.content), true
}

func (s *GitHubImageStorage) cacheImage(id string, content []byte) {
	if len(content) > githubImageCacheLimit {
		return
	}
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if element, ok := s.cache[id]; ok {
		s.removeCachedImage(element)
	}
	for s.cacheBytes+len(content) > githubImageCacheLimit {
		s.removeCachedImage(s.cacheOrder.Back())
	}
	entry := &githubImageCacheEntry{id: id, content: bytes.Clone(content), expires: time.Now().Add(githubImageCacheTTL)}
	s.cache[id] = s.cacheOrder.PushFront(entry)
	s.cacheBytes += len(content)
}

func (s *GitHubImageStorage) removeCachedImage(element *list.Element) {
	entry := element.Value.(*githubImageCacheEntry)
	delete(s.cache, entry.id)
	s.cacheBytes -= len(entry.content)
	s.cacheOrder.Remove(element)
}

func (s *GitHubImageStorage) evictImage(id string) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()
	if element, ok := s.cache[id]; ok {
		s.removeCachedImage(element)
	}
}

func (s *GitHubImageStorage) endpoint(id string) string {
	return "https://api.github.com/repos/" + url.PathEscape(s.owner) + "/" + url.PathEscape(s.repository) + "/contents/products/" + url.PathEscape(id)
}

func (s *GitHubImageStorage) request(ctx context.Context, method, id, accept string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.endpoint(id), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if method == http.MethodGet {
		query := req.URL.Query()
		query.Set("ref", s.branch)
		req.URL.RawQuery = query.Encode()
	}
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return s.client.Do(req)
}

func (s *GitHubImageStorage) Save(ctx context.Context, id string, content []byte) error {
	payload, err := json.Marshal(map[string]string{
		"message": "Add CRM product image " + id,
		"content": base64.StdEncoding.EncodeToString(content),
		"branch":  s.branch,
	})
	if err != nil {
		return err
	}
	response, err := s.request(ctx, http.MethodPut, id, "application/vnd.github+json", payload)
	if err != nil {
		return fmt.Errorf("GitHub 图片上传失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		return fmt.Errorf("GitHub 图片上传失败（HTTP %d）", response.StatusCode)
	}
	s.cacheImage(id, content)
	return nil
}

func (s *GitHubImageStorage) Read(ctx context.Context, id string) ([]byte, error) {
	if content, ok := s.cachedImage(id); ok {
		return content, nil
	}
	response, err := s.request(ctx, http.MethodGet, id, "application/vnd.github.raw+json", nil)
	if err != nil {
		return nil, fmt.Errorf("GitHub 图片读取失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub 图片读取失败（HTTP %d）", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 20<<20+1))
	if err != nil {
		return nil, err
	}
	if len(content) > 20<<20 {
		return nil, fmt.Errorf("GitHub 图片超过大小限制")
	}
	s.cacheImage(id, content)
	return content, nil
}

func (s *GitHubImageStorage) Delete(ctx context.Context, id string) error {
	response, err := s.request(ctx, http.MethodGet, id, "application/vnd.github+json", nil)
	if err != nil {
		return fmt.Errorf("GitHub 图片删除前查询失败: %w", err)
	}
	if response.StatusCode == http.StatusNotFound {
		response.Body.Close()
		s.evictImage(id)
		return nil
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return fmt.Errorf("GitHub 图片删除前查询失败（HTTP %d）", response.StatusCode)
	}
	var file struct {
		SHA string `json:"sha"`
	}
	err = json.NewDecoder(response.Body).Decode(&file)
	response.Body.Close()
	if err != nil || strings.TrimSpace(file.SHA) == "" {
		return fmt.Errorf("GitHub 图片版本信息无效")
	}
	payload, err := json.Marshal(map[string]string{"message": "Delete CRM product image " + id, "sha": file.SHA, "branch": s.branch})
	if err != nil {
		return err
	}
	response, err = s.request(ctx, http.MethodDelete, id, "application/vnd.github+json", payload)
	if err != nil {
		return fmt.Errorf("GitHub 图片删除失败: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub 图片删除失败（HTTP %d）", response.StatusCode)
	}
	s.evictImage(id)
	return nil
}
