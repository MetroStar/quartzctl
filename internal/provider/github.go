// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/google/go-github/v63/github"
	"k8s.io/apimachinery/pkg/util/errors"
)

// GithubTokenSource represents a source for GitHub access tokens.
type GithubTokenSource struct {
	AccessToken string // The GitHub access token.
}

// GithubClient represents a client for interacting with the GitHub API.
type GithubClient struct {
	providerName string                   // The name of the provider.
	cfg          schema.QuartzConfig      // The Quartz configuration.
	creds        schema.GithubCredentials // The GitHub credentials.
	httpClient   util.HttpClientFactory   // The HTTP client factory for making requests.
}

// GithubCheckAccessResult represents the result of a GitHub repository access check.
type GithubCheckAccessResult struct {
	Organization string // The organization name.
	Repository   string // The repository name.
	Error        error  // Any error encountered during the access check.

	Name     string // The full name of the repository.
	Pull     bool   // Indicates if the user has pull access.
	Push     bool   // Indicates if the user has push access.
	Triage   bool   // Indicates if the user has triage access.
	Maintain bool   // Indicates if the user has maintain access.
	Admin    bool   // Indicates if the user has admin access.
	Packages bool   // Indicates if the user has access to packages.
}

// GithubProviderCheckResult represents the result of a GitHub provider check.
type GithubProviderCheckResult struct {
	Status  bool                      // Indicates if the check was successful.
	Results []GithubCheckAccessResult // The results of the access checks.
	Error   error                     // Any error encountered during the check.
}

// NewGithubClient creates a new GitHub client with the specified configuration and credentials.
// Returns an error if the credentials are missing.
func NewGithubClient(httpClient util.HttpClientFactory, providerName string, cfg schema.QuartzConfig, creds schema.GithubCredentials) (GithubClient, error) {
	if creds.Username == "" || creds.Token == "" {
		return GithubClient{}, fmt.Errorf("github credentials not found")
	}

	if providerName == "" {
		providerName = "Github"
	}

	return GithubClient{
		providerName: providerName,
		cfg:          cfg,
		creds:        creds,
		httpClient:   httpClient,
	}, nil
}

// ProviderName returns the name of the GitHub provider.
func (c GithubClient) ProviderName() string {
	return c.providerName
}

// CheckAccess performs an access check for the GitHub provider.
// It returns a GithubProviderCheckResult containing the results of the check.
func (c GithubClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	r, err := c.CheckGithubRepoAccess(ctx)

	res := GithubProviderCheckResult{
		Status:  err == nil,
		Results: r,
		Error:   err,
	}

	return res
}

// CheckGithubRepoAccess checks access to the configured GitHub repositories.
// It returns a list of GithubCheckAccessResult and an error if any issues are encountered.
func (c GithubClient) CheckGithubRepoAccess(ctx context.Context) ([]GithubCheckAccessResult, error) {
	http := c.httpClient.NewClient()
	client := github.NewClient(http).WithAuthToken(c.creds.Token)

	resch := make(chan GithubCheckAccessResult)
	wg := sync.WaitGroup{}

	repositories := c.Repositories()
	for _, r := range repositories {
		wg.Add(1)
		go func(ri schema.RepositoryConfig) {
			log.Debug("Github access check start", "org", ri.Organization, "repo", ri.Name)
			defer wg.Done()

			repo, resp, err := client.Repositories.Get(ctx, ri.Organization, ri.Name)

			res := GithubCheckAccessResult{
				Organization: ri.Organization,
				Repository:   ri.Name,
				Error:        err,
			}

			if err == nil {
				log.Debug("Github access check result",
					"name", *repo.FullName,
					"permissions", repo.Permissions,
					"scopes", resp.Header.Get("X-Oauth-Scopes"),
				)
				res.Name = *repo.FullName
				res.Pull = repo.Permissions["pull"]
				res.Push = repo.Permissions["push"]
				res.Maintain = repo.Permissions["maintain"]
				res.Triage = repo.Permissions["triage"]
				res.Admin = repo.Permissions["admin"]
				res.Packages = slices.ContainsFunc(resp.Header.Values("X-Oauth-Scopes"), func(s string) bool {
					return strings.Contains(s, "read:packages") || strings.Contains(s, "write:packages")
				})
			} else {
				log.Info("Github access check error", "name", ri.Name, "err", err)
			}

			resch <- res
		}(r)
	}

	go func() {
		wg.Wait()
		close(resch)
	}()

	var res []GithubCheckAccessResult
	var errs []error
	for r := range resch {
		res = append(res, r)
		if r.Error != nil {
			errs = append(errs, r.Error)
		}
	}

	if len(errs) > 0 {
		return res, errors.NewAggregate(errs)
	}

	return res, nil
}

// ToTable converts the GithubProviderCheckResult into table headers and rows for display.
func (r GithubProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	headers := []string{"Repository", "Pull", "Push", "Triage", "Maintain", "Admin", "Packages"}
	var rows []ProviderCheckResultRow

	for _, r := range r.Results {
		if r.Error != nil {
			rows = append(rows, ProviderCheckResultRow{
				Status: false,
				Error:  r.Error,
				Data:   []string{fmt.Sprintf("%s/%s", r.Organization, r.Repository)},
			})
			continue
		}

		var err error
		if !r.Pull {
			err = fmt.Errorf("insufficient permissions")
		}

		rows = append(rows, ProviderCheckResultRow{
			Status: err == nil,
			Error:  err,
			Data: []string{
				r.Name,
				strconv.FormatBool(r.Pull),
				strconv.FormatBool(r.Pull),
				strconv.FormatBool(r.Triage),
				strconv.FormatBool(r.Maintain),
				strconv.FormatBool(r.Admin),
				strconv.FormatBool(r.Packages),
			},
		})
	}

	return headers, rows
}

// Repositories retrieves the list of repositories configured in the Quartz configuration.
func (c GithubClient) Repositories() []schema.RepositoryConfig {
	repositories := []schema.RepositoryConfig{
		c.cfg.Gitops.Core,
		c.cfg.Gitops.Apps,
	}

	for _, app := range c.cfg.Applications {
		a := app
		r := a.RepositoryConfig()
		if r.RepoUrl != "" {
			repositories = append(repositories, r)
		}
	}

	return repositories
}
