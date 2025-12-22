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
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/util"
)

// StageCheck defines the interface for stage checks.
type StageCheck interface {
	// Run executes the stage check and returns an error if the check fails.
	Run(ctx context.Context, cfg schema.QuartzConfig) error
	// Id returns the unique identifier of the stage check.
	Id() string
	// Type returns the type of the stage check.
	Type() string
	// RetryOpts returns the retry configuration for the stage check.
	RetryOpts() schema.StageChecksRetryConfig
}

// CheckResult represents the result of a stage check.
type CheckResult struct {
	Id    string // The unique identifier of the check.
	Type  string // The type of the check.
	Stage string // The stage associated with the check.
	Event string // The event associated with the check.
	Error error  // Any error encountered during the check.
}

// CheckOpts contains options for running stage checks.
type CheckOpts struct {
	OnStart    func(cr CheckResult)        // Callback invoked when a check starts.
	OnComplete func(cr CheckResult)        // Callback invoked when a check completes.
	OnRetry    func(cr CheckResult, n int) // Callback invoked when a check is retried.
}

// StageCheckRetryOpts defines retry options for stage checks.
type StageCheckRetryOpts struct {
	Max            int // The maximum number of retries.
	BackoffSeconds int // The backoff time in seconds between retries.
}

// RunPreChecks executes pre-event checks for the specified stage and event.
// Returns the results of the checks and any errors encountered.
func RunPreChecks(ctx context.Context, cfg schema.QuartzConfig, providerFactory provider.ProviderFactory, stage string, event string, opts *CheckOpts) ([]CheckResult, error) {
	var rs []CheckResult

	stg := cfg.Stages[stage]
	cs := preEventChecks(stg, event, providerFactory)
	for _, c := range cs {
		r, err := RunChecks(ctx, cfg, stage, event, c, opts)
		rs = append(rs, r...)
		if err != nil {
			return rs, err
		}
	}

	return rs, nil
}

// RunPostChecks executes post-event checks for the specified stage and event.
// Returns the results of the checks and any errors encountered.
func RunPostChecks(ctx context.Context, cfg schema.QuartzConfig, providerFactory provider.ProviderFactory, stage string, event string, opts *CheckOpts) ([]CheckResult, error) {
	var rs []CheckResult

	stg := cfg.Stages[stage]
	cs := postEventChecks(stg, event, providerFactory)
	for _, c := range cs {
		r, err := RunChecks(ctx, cfg, stage, event, c, opts)
		rs = append(rs, r...)
		if err != nil {
			return rs, err
		}
	}

	return rs, nil
}

// RunChecks executes the specified stage checks and handles retries based on the retry configuration.
// Returns the results of the checks and any errors encountered.
func RunChecks(ctx context.Context, cfg schema.QuartzConfig, stage string, event string, checks []StageCheck, opts *CheckOpts) ([]CheckResult, error) {
	var wg sync.WaitGroup
	wg.Add(len(checks))

	ch := make(chan CheckResult, len(checks))
	for _, sc := range checks {
		go func(sc StageCheck) {
			defer wg.Done()
			cr := CheckResult{
				Id:    sc.Id(),
				Type:  sc.Type(),
				Stage: stage,
				Event: event,
			}

			if opts.OnStart != nil {
				opts.OnStart(cr)
			}

			i := 1
			ro := sc.RetryOpts()
			for i <= ro.Limit {
				cr.Error = sc.Run(ctx, cfg)
				if cr.Error == nil || i == ro.Limit {
					break
				}

				if opts.OnRetry != nil {
					opts.OnRetry(cr, i)
				}

				if ro.WaitSeconds > 0 {
					// Use exponential backoff: start with configured wait, cap at 60s
					// Formula: min(baseWait * 2^(attempt-1), maxWait)
					baseWait := ro.WaitSeconds
					if baseWait < 10 {
						baseWait = 10 // Minimum 10 seconds
					}
					maxWait := 60 // Maximum 60 seconds cap

					waitTime := baseWait
					// Apply exponential growth for retries 2+, capped at maxWait
					if i > 1 {
						// Calculate exponential delay, but cap growth after a few retries
						factor := 1 << min(i-1, 3) // 2^(i-1), capped at 2^3 = 8x
						waitTime = min(baseWait*factor, maxWait)
					}

					time.Sleep(time.Duration(waitTime) * time.Second)
				}

				i = i + 1
			}

			if opts.OnComplete != nil {
				opts.OnComplete(cr)
			}

			ch <- cr
		}(sc)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var res []CheckResult
	var errs []error
	for r := range ch {
		res = append(res, r)
		if r.Error != nil {
			errs = append(errs, r.Error)
		}
	}

	if len(errs) > 0 {
		return res, errors.Join(errs...)
	}

	return res, nil
}

// preEventChecks retrieves the pre-event checks for the specified stage and event.
// Returns the checks grouped by their order.
func preEventChecks(sc schema.StageConfig, event string, providerFactory provider.ProviderFactory) [][]StageCheck {
	tmp := make(map[int][]StageCheck)

	for _, v := range sc.Checks {
		if len(v.Before) <= 0 {
			continue
		}

		var c []StageCheck

		for _, e := range v.Before {
			// event not matched for this group, skip to the next
			if !strings.EqualFold(e, event) {
				continue
			}

			c = appendChecks(c, v, providerFactory)
		}

		if tmp[v.Order] == nil {
			tmp[v.Order] = c
			continue
		}

		tmp[v.Order] = append(tmp[v.Order], c...)
	}

	return util.MapIntKeysToSortedSlice(tmp)
}

// postEventChecks retrieves the post-event checks for the specified stage and event.
// Returns the checks grouped by their order.
func postEventChecks(sc schema.StageConfig, event string, providerFactory provider.ProviderFactory) [][]StageCheck {
	tmp := make(map[int][]StageCheck)

	for _, v := range sc.Checks {
		if len(v.After) <= 0 {
			continue
		}

		var c []StageCheck

		for _, e := range v.After {
			// event not matched for this group, skip to the next
			if !strings.EqualFold(e, event) {
				continue
			}

			c = appendChecks(c, v, providerFactory)
		}

		if tmp[v.Order] == nil {
			tmp[v.Order] = c
			continue
		}

		tmp[v.Order] = append(tmp[v.Order], c...)
	}

	return util.MapIntKeysToSortedSlice(tmp)
}

// appendChecks appends the specified stage checks to the result slice.
// Handles HTTP, Kubernetes, and state checks.
func appendChecks(r []StageCheck, s schema.StageChecksConfig, providerFactory provider.ProviderFactory) []StageCheck {
	for _, hc := range s.Http {
		ihc := hc
		r = append(r, HttpStageCheck(ihc))
	}

	for _, kc := range s.Kubernetes {
		ikc := kc
		r = append(r, NewKubernetesStageCheck(ikc, providerFactory))
	}

	for _, sc := range s.State {
		isc := sc
		r = append(r, NewStateStageCheck(isc, providerFactory))
	}

	return r
}
