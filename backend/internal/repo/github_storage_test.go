package repo

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type githubTransport func(*http.Request) (*http.Response, error)

func (transport githubTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

func TestGitHubImageStorageSaveReadDelete(t *testing.T) {
	storage := NewGitHubImageStorage("ziangcao", "crm-product-images", "main", "test-token")
	var calls []string
	storage.client.Transport = githubTransport(func(request *http.Request) (*http.Response, error) {
		calls = append(calls, request.Method+" "+request.URL.Path)
		if request.URL.Path != "/repos/ziangcao/crm-product-images/contents/products/gh_123.png" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Method == http.MethodGet && request.URL.Query().Get("ref") != "main" {
			t.Fatal("image lookup must use configured branch")
		}
		if request.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatal("missing private repository authorization")
		}
		response := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)}
		switch len(calls) {
		case 1:
			var body struct{ Content, Branch string }
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Content != base64.StdEncoding.EncodeToString([]byte("picture")) || body.Branch != "main" {
				t.Fatal("incorrect upload payload")
			}
			response.StatusCode = http.StatusCreated
		case 2:
			response.Body = io.NopCloser(strings.NewReader(`{"sha":"blob-sha"}`))
		case 3:
			var body struct{ SHA, Branch string }
			if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.SHA != "blob-sha" || body.Branch != "main" {
				t.Fatal("incorrect delete payload")
			}
		case 4:
			if request.Header.Get("Accept") != "application/vnd.github.raw+json" {
				t.Fatal("image read must request raw bytes")
			}
			response.Body = io.NopCloser(strings.NewReader("picture"))
		default:
			t.Fatal("unexpected GitHub request")
		}
		if response.Body == nil {
			response.Body = io.NopCloser(strings.NewReader("{}"))
		}
		return response, nil
	})
	ctx := context.Background()
	if err := storage.Save(ctx, "gh_123.png", []byte("picture")); err != nil {
		t.Fatal(err)
	}
	content, err := storage.Read(ctx, "gh_123.png")
	if err != nil || string(content) != "picture" {
		t.Fatalf("read: %q, %v", content, err)
	}
	if err := storage.Delete(ctx, "gh_123.png"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		content, err = storage.Read(ctx, "gh_123.png")
		if err != nil || string(content) != "picture" {
			t.Fatalf("read after delete: %q, %v", content, err)
		}
	}
	if len(calls) != 4 || !strings.HasPrefix(calls[0], "PUT ") || !strings.HasPrefix(calls[1], "GET ") || !strings.HasPrefix(calls[2], "DELETE ") || !strings.HasPrefix(calls[3], "GET ") {
		t.Fatalf("unexpected request sequence: %v", calls)
	}
}

func TestGitHubImageStorageCacheLimit(t *testing.T) {
	storage := NewGitHubImageStorage("owner", "repo", "main", "token")
	storage.cacheImage("first", make([]byte, 20<<20))
	storage.cacheImage("second", make([]byte, 20<<20))
	if _, ok := storage.cachedImage("first"); ok {
		t.Fatal("oldest image should be evicted")
	}
	if _, ok := storage.cachedImage("second"); !ok || storage.cacheBytes > githubImageCacheLimit {
		t.Fatal("cache should retain the newest image within its limit")
	}
}
