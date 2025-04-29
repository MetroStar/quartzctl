package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/google/go-github/v63/github"
)

func TestProviderGithubClientProviderName(t *testing.T) {
	_, err := NewGithubClient(nil, "", schema.QuartzConfig{}, schema.GithubCredentials{})
	if err == nil {
		t.Error("expected error from github client for missing required arguments")
	}

	c, err := NewGithubClient(nil, "", schema.QuartzConfig{}, schema.GithubCredentials{
		Username: "testuser",
		Token:    "supersecrettoken",
	})
	if err != nil {
		t.Errorf("unexpected error from github client constructor, %v", err)
	}

	if c.ProviderName() != "Github" {
		t.Errorf("unexpected default provider name from github client, expected %v, found %v", "Github", c.ProviderName())
	}
}

// TestProviderGithubClientCheckAccess tests the GitHub client access check functionality.
// It validates that the client correctly handles access tokens and repository configurations.
func TestProviderGithubClientCheckAccess(t *testing.T) {
	httpClient := util.HttpClientFactoryMock{
		Callback: func(req *http.Request) *http.Response {
			if req.Method != "GET" {
				t.Errorf("unexpected http request method, expected %v, found %v", "GET", req.Method)
			}

			if req.URL.Host != "api.github.com" {
				t.Errorf("unexpected http request url, expected %v, found %v", "GET", req.URL.String())
			}

			if strings.HasSuffix(req.URL.String(), "error") {
				return &http.Response{
					StatusCode: 500,
					Body:       io.NopCloser(bytes.NewBufferString("")),
					Header:     http.Header{},
				}
			}

			repo, _ := json.Marshal(github.Repository{
				FullName: github.String("test"),
				Permissions: map[string]bool{
					"pull":     false,
					"push":     true,
					"maintain": true,
					"triage":   true,
					"admin":    true,
				},
			})
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewBuffer(repo)),
				Header: http.Header{
					"X-Oauth-Scopes": []string{"write:packages"},
				},
			}
		},
	}

	cfg := schema.QuartzConfig{
		Gitops: schema.GitopsConfig{
			Core: schema.RepositoryConfig{
				Name:         "testinfrarepo",
				Organization: "example",
				RepoUrl:      "https://github.com/example/testinfrarepo",
			},
			Apps: schema.RepositoryConfig{
				Name:         "testappsrepo",
				Organization: "example",
				RepoUrl:      "https://github.com/example/testappsrepo",
			},
		},
		Applications: map[string]schema.ApplicationRepositoryConfig{
			"testapp1": {
				Name:         "testapp1",
				Organization: "example",
				RepoUrl:      "https://github.com/example/testapp1",
			},
			"error": {
				Name:         "error",
				Organization: "example",
				RepoUrl:      "https://github.com/example/error",
			},
		},
	}

	c, err := NewGithubClient(httpClient, "", cfg, schema.GithubCredentials{
		Username: "testuser",
		Token:    "supersecrettoken",
	})
	if err != nil {
		t.Errorf("unexpected error from github client constructor, %v", err)
	}

	res := c.CheckAccess(context.Background())
	switch r := res.(type) {
	case GithubProviderCheckResult:
		if r.Status ||
			r.Error == nil {
			t.Errorf("expected error from github check access not found, %v", r)
		}

		if len(r.Results) == 0 {
			t.Errorf("unexpected result set value from github check access, %v", r)
		}
	default:
		t.Errorf("unexpected response type from github check access, %v", r)
	}

	headers, rows := res.ToTable()
	if len(rows) != 4 {
		t.Errorf("unexpected response from github check access table, %v, %v", headers, rows)
	}

	errorCount := 0
	for _, r := range rows {
		if !r.Status ||
			r.Error != nil {
			errorCount = errorCount + 1
		}
	}

	if errorCount != 4 {
		t.Errorf("expected 4 errors, found %v", errorCount)
	}
}
