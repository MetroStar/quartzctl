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

func TestStagesCheckAppendChecksDaemonSet(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	cfg := schema.StageChecksConfig{
		DaemonSet: []schema.StageChecksDaemonSetConfig{
			{Name: "istio-cni-node", Namespace: "kube-system"},
			{Name: "calico-node", Namespace: "kube-system"},
		},
	}

	result := appendChecks(nil, cfg, *f)

	if len(result) != 2 {
		t.Errorf("expected 2 checks, got %d", len(result))
		return
	}

	// Verify they're DaemonSet checks
	for i, check := range result {
		if check.Type() != "daemonset" {
			t.Errorf("check %d: expected type 'daemonset', got '%s'", i, check.Type())
		}
	}
}

func TestStagesCheckAppendChecksMultipleTypes(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer svr.Close()

	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	cfg := schema.StageChecksConfig{
		Http: []schema.StageChecksHttpConfig{
			{Url: svr.URL, StatusCodes: []int{200}},
		},
		DaemonSet: []schema.StageChecksDaemonSetConfig{
			{Name: "istio-cni-node", Namespace: "kube-system"},
		},
	}

	result := appendChecks(nil, cfg, *f)

	if len(result) != 2 {
		t.Errorf("expected 2 checks (http + daemonset), got %d", len(result))
		return
	}

	typeCount := map[string]int{}
	for _, check := range result {
		typeCount[check.Type()]++
	}

	if typeCount["http"] != 1 {
		t.Errorf("expected 1 http check, got %d", typeCount["http"])
	}
	if typeCount["daemonset"] != 1 {
		t.Errorf("expected 1 daemonset check, got %d", typeCount["daemonset"])
	}
}

func TestStagesPreEventChecksWithOrder(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	stageConfig := schema.StageConfig{
		Checks: map[string]schema.StageChecksConfig{
			"first": {
				Order:  1,
				Before: []string{"install"},
				DaemonSet: []schema.StageChecksDaemonSetConfig{
					{Name: "ds1", Namespace: "ns1"},
				},
			},
			"second": {
				Order:  2,
				Before: []string{"install"},
				DaemonSet: []schema.StageChecksDaemonSetConfig{
					{Name: "ds2", Namespace: "ns2"},
				},
			},
		},
	}

	result := preEventChecks(stageConfig, "install", *f)

	if len(result) != 2 {
		t.Errorf("expected 2 check groups, got %d", len(result))
		return
	}

	// First group should have ds1, second group should have ds2
	if len(result[0]) != 1 {
		t.Errorf("expected 1 check in first group, got %d", len(result[0]))
	}
	if len(result[1]) != 1 {
		t.Errorf("expected 1 check in second group, got %d", len(result[1]))
	}
}

func TestStagesPostEventChecksWithDaemonSet(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	stageConfig := schema.StageConfig{
		Checks: map[string]schema.StageChecksConfig{
			"cni-ready": {
				After: []string{"helm-install"},
				DaemonSet: []schema.StageChecksDaemonSetConfig{
					{Name: "istio-cni-node", Namespace: "kube-system"},
				},
			},
		},
	}

	result := postEventChecks(stageConfig, "helm-install", *f)

	if len(result) != 1 {
		t.Errorf("expected 1 check group, got %d", len(result))
		return
	}

	if len(result[0]) != 1 {
		t.Errorf("expected 1 check in group, got %d", len(result[0]))
	}

	if result[0][0].Type() != "daemonset" {
		t.Errorf("expected daemonset check, got '%s'", result[0][0].Type())
	}
}
