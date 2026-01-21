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

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
)

// DaemonSetStageCheck represents a DaemonSet readiness check.
// This check ensures that a DaemonSet has all desired pods running and ready
// on all applicable nodes, which is critical for CNI plugins like istio-cni.
type DaemonSetStageCheck struct {
	src             schema.StageChecksDaemonSetConfig // The configuration for the DaemonSet check.
	providerFactory provider.ProviderFactory          // The provider factory for accessing Kubernetes resources.
}

// NewDaemonSetStageCheck creates a new DaemonSetStageCheck instance with the specified configuration.
func NewDaemonSetStageCheck(src schema.StageChecksDaemonSetConfig, providerFactory provider.ProviderFactory) DaemonSetStageCheck {
	return DaemonSetStageCheck{
		src:             src,
		providerFactory: providerFactory,
	}
}

// Run executes the DaemonSet readiness check.
// It verifies that all desired replicas are ready and available.
func (c DaemonSetStageCheck) Run(ctx context.Context, _ schema.QuartzConfig) error {
	kube, err := c.providerFactory.Kubernetes(ctx)
	if err != nil {
		return err
	}

	kind, err := kube.LookupKind(ctx, "DaemonSet")
	if err != nil {
		return err
	}

	ready, desired, err := kube.GetDaemonSetStatus(ctx, kind, c.src.Namespace, c.src.Name)
	if err != nil {
		return err
	}

	if ready < desired {
		return fmt.Errorf("daemonset %s/%s not fully ready: %d/%d pods ready", c.src.Namespace, c.src.Name, ready, desired)
	}

	return nil
}

// Id returns the unique identifier of the DaemonSet stage check.
func (c DaemonSetStageCheck) Id() string {
	return fmt.Sprintf("DaemonSet/%s (%s)", c.src.Name, c.src.Namespace)
}

// Type returns the type of the stage check, which is "daemonset".
func (c DaemonSetStageCheck) Type() string {
	return "daemonset"
}

// RetryOpts returns the retry configuration for the DaemonSet stage check.
func (c DaemonSetStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	limit := c.src.Retry.Limit
	if limit <= 0 {
		limit = 30 // Default 30 retries
	}
	waitSeconds := c.src.Retry.WaitSeconds
	if waitSeconds <= 0 {
		waitSeconds = 10 // Default 10 seconds between retries
	}

	return schema.StageChecksRetryConfig{
		Limit:       limit,
		WaitSeconds: waitSeconds,
	}
}
