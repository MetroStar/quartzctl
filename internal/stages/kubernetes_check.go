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

// TODO: port quartz health check(s) to a job or service that deploys with the chart,
//   the cli can just check that process instead of needing to enumerate all the dependencies
//   explicitly

import (
	"context"
	"errors"
	"fmt"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
)

// KubernetesStageCheck represents a Kubernetes-based stage check.
type KubernetesStageCheck struct {
	src             schema.StageChecksKubernetesConfig // The configuration for the Kubernetes stage check.
	providerFactory provider.ProviderFactory           // The provider factory for accessing Kubernetes resources.
}

// NewKubernetesStageCheck creates a new KubernetesStageCheck instance with the specified configuration and provider factory.
func NewKubernetesStageCheck(src schema.StageChecksKubernetesConfig, providerFactory provider.ProviderFactory) KubernetesStageCheck {
	return KubernetesStageCheck{
		src:             src,
		providerFactory: providerFactory,
	}
}

// Run executes the Kubernetes stage check.
// It performs operations such as restarting resources, waiting for conditions, and validating resource states.
func (c KubernetesStageCheck) Run(ctx context.Context, _ schema.QuartzConfig) error {
	kube, err := c.providerFactory.Kubernetes(ctx)
	if err != nil {
		return err
	}

	kind, err := kube.LookupKind(ctx, c.src.Kind)
	if err != nil {
		return err
	}

	if c.src.Restart {
		err = kube.Restart(ctx, kind, c.src.Namespace, c.src.Name)
		if err != nil {
			return err
		}
	}

	if c.src.Wait == nil || !*(c.src.Wait) {
		return nil
	}

	if c.src.Name == "" || c.src.Namespace == "" {
		return errors.New("name and namespace required for check")
	}

	return kube.WaitConditionState(ctx, kind, c.src.Namespace, c.src.Name, c.src.State, c.src.Timeout)
}

// Id returns the unique identifier of the Kubernetes stage check.
// The identifier includes the kind, name, and namespace of the resource being checked.
func (c KubernetesStageCheck) Id() string {
	return fmt.Sprintf("%s/%s (%s)", c.src.Kind, c.src.Name, c.src.Namespace)
}

// Type returns the type of the stage check, which is "kubernetes".
func (c KubernetesStageCheck) Type() string {
	return "kubernetes"
}

// RetryOpts returns the retry configuration for the Kubernetes stage check.
// By default, it allows only one attempt with no wait time between retries.
func (c KubernetesStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	return schema.StageChecksRetryConfig{
		Limit:       1,
		WaitSeconds: -1,
	}
}
