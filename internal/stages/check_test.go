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

package stages

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
)

type TestStageCheck struct {
	err         error
	t           *testing.T
	limit       int
	waitseconds int
}

func (c TestStageCheck) Run(ctx context.Context, cfg schema.QuartzConfig) error {
	return c.err
}

func (c TestStageCheck) Id() string {
	return "test-check"
}

func (c TestStageCheck) Type() string {
	return "test"
}

func (c TestStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	return schema.StageChecksRetryConfig{Limit: c.limit, WaitSeconds: c.waitseconds}
}

func (c TestStageCheck) OnStart(cr CheckResult) {
	c.t.Logf("start %v", cr)
}

func (c TestStageCheck) OnComplete(cr CheckResult) {
	c.t.Logf("complete %v", cr)
}

func (c TestStageCheck) OnRetry(cr CheckResult, n int) {
	c.t.Logf("retry %d %v", n, cr)
}

func TestStagesCheckRunPreChecks(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	stage := "test-stage"
	event := "test-event"
	cfg := schema.QuartzConfig{
		Stages: map[string]schema.StageConfig{
			stage: {
				Checks: map[string]schema.StageChecksConfig{
					"1": {
						Http: []schema.StageChecksHttpConfig{
							{Url: svr.URL, StatusCodes: []int{200}},
						},
					},
				},
			},
		},
	}
	opts := &CheckOpts{}
	res, _ := RunPreChecks(context.Background(), cfg, provider.ProviderFactory{}, stage, event, opts)
	if len(res) > 0 {
		t.Errorf("unexpected results found, %v", res)
	}

	cfg = schema.QuartzConfig{
		Stages: map[string]schema.StageConfig{
			stage: {
				Checks: map[string]schema.StageChecksConfig{
					"1": {
						Before: []string{event, "other-event"},
						Http: []schema.StageChecksHttpConfig{
							{Url: svr.URL, StatusCodes: []int{200}},
						},
					},
				},
			},
		},
	}
	res, err := RunPreChecks(context.Background(), cfg, provider.ProviderFactory{}, stage, event, opts)

	if err != nil {
		t.Errorf("unexpected error in stages runprechecks, %v", err)
		return
	}

	if len(res) == 0 {
		t.Error("no results found")
		return
	}

	if len(res) != 1 {
		t.Errorf("unexpected result from stages runprechecks, %v", res)
	}
}

func TestStagesCheckRunPostChecks(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	stage := "test-stage"
	event := "test-event"
	cfg := schema.QuartzConfig{
		Stages: map[string]schema.StageConfig{
			stage: {
				Checks: map[string]schema.StageChecksConfig{
					"1": {
						Http: []schema.StageChecksHttpConfig{
							{Url: svr.URL, StatusCodes: []int{200}},
						},
					},
				},
			},
		},
	}
	opts := &CheckOpts{}
	res, _ := RunPostChecks(context.Background(), cfg, provider.ProviderFactory{}, stage, event, opts)
	if len(res) > 0 {
		t.Errorf("unexpected results found, %v", res)
	}

	cfg = schema.QuartzConfig{
		Stages: map[string]schema.StageConfig{
			stage: {
				Checks: map[string]schema.StageChecksConfig{
					"1": {
						After: []string{event, "other-event"},
						Http: []schema.StageChecksHttpConfig{
							{Url: svr.URL, StatusCodes: []int{200}},
						},
					},
				},
			},
		},
	}
	res, err := RunPostChecks(context.Background(), cfg, provider.ProviderFactory{}, stage, event, opts)

	if err != nil {
		t.Errorf("unexpected error in stages runpostchecks, %v", err)
		return
	}

	if len(res) == 0 {
		t.Error("no results found")
		return
	}

	if len(res) != 1 {
		t.Errorf("unexpected result from stages runpostchecks, %v", res)
	}
}

func TestStagesCheckRunChecks(t *testing.T) {
	cfg := schema.QuartzConfig{}
	stage := "test-stage"
	event := "test-event"

	c := TestStageCheck{t: t, limit: 2}
	checks := []StageCheck{c}
	opts := &CheckOpts{
		OnStart:    c.OnStart,
		OnComplete: c.OnComplete,
		OnRetry:    c.OnRetry,
	}
	res, err := RunChecks(context.Background(), cfg, stage, event, checks, opts)

	if err != nil {
		t.Errorf("unexpected error in stages runchecks, %v", err)
		return
	}

	if len(res) == 0 {
		t.Error("no results found")
		return
	}

	if len(res) != 1 {
		t.Errorf("unexpected result from stages runchecks, %v", res)
	}
}

func TestStagesCheckRunChecksError(t *testing.T) {
	cfg := schema.QuartzConfig{}
	stage := "test-stage"
	event := "test-event"
	c := TestStageCheck{t: t, err: fmt.Errorf("check failed"), limit: 2, waitseconds: 1}
	checks := []StageCheck{c}
	opts := &CheckOpts{
		OnStart:    c.OnStart,
		OnComplete: c.OnComplete,
		OnRetry:    c.OnRetry,
	}
	res, err := RunChecks(context.Background(), cfg, stage, event, checks, opts)

	if err == nil {
		t.Errorf("expected error in stages runchecks, %v", err)
		return
	}

	if len(res) == 0 {
		t.Error("no results found")
		return
	}

	if len(res) != 1 {
		t.Errorf("unexpected result from stages runchecks, %v", res)
	}
}
