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
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDaemonSetStageCheckRun(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	// Create a mock DaemonSet object with status
	ds := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"namespace": "kube-system",
				"name":      "istio-cni-node",
			},
			"status": map[string]interface{}{
				"desiredNumberScheduled": int64(3),
				"numberReady":            int64(3),
				"currentNumberScheduled": int64(3),
				"numberAvailable":        int64(3),
			},
		},
	}

	api := provider.NewKubernetesApiMock().
		WithDynamicObjects(ds).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
			},
		})
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "istio-cni-node",
		Namespace: "kube-system",
	}, *f)

	err := c.Run(context.Background(), cfg)
	assert.NoError(t, err, "DaemonSet check should pass when all pods are ready")
}

func TestDaemonSetStageCheckRunNotReady(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	// Create a mock DaemonSet with some pods not ready
	ds := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"namespace": "kube-system",
				"name":      "istio-cni-node",
			},
			"status": map[string]interface{}{
				"desiredNumberScheduled": int64(3),
				"numberReady":            int64(1), // Only 1 of 3 ready
				"currentNumberScheduled": int64(3),
				"numberAvailable":        int64(1),
			},
		},
	}

	api := provider.NewKubernetesApiMock().
		WithDynamicObjects(ds).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
			},
		})
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "istio-cni-node",
		Namespace: "kube-system",
	}, *f)

	err := c.Run(context.Background(), cfg)
	assert.Error(t, err, "DaemonSet check should fail when not all pods are ready")
	assert.Contains(t, err.Error(), "not fully ready")
}

func TestDaemonSetStageCheckId(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "istio-cni-node",
		Namespace: "kube-system",
	}, *f)

	id := c.Id()
	assert.Contains(t, id, "istio-cni-node")
	assert.Contains(t, id, "kube-system")
}

func TestDaemonSetStageCheckType(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "test",
		Namespace: "test",
	}, *f)

	assert.Equal(t, "daemonset", c.Type())
}

func TestDaemonSetStageCheckRetryOptsDefault(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "test",
		Namespace: "test",
		// No retry config - should use defaults
	}, *f)

	opts := c.RetryOpts()
	assert.Equal(t, 30, opts.Limit, "Default retry limit should be 30")
	assert.Equal(t, 10, opts.WaitSeconds, "Default wait seconds should be 10")
}

func TestDaemonSetStageCheckRetryOptsCustom(t *testing.T) {
	f := provider.NewProviderFactory(schema.QuartzConfig{}, schema.QuartzSecrets{})

	c := NewDaemonSetStageCheck(schema.StageChecksDaemonSetConfig{
		Name:      "test",
		Namespace: "test",
		Retry: schema.StageChecksRetryConfig{
			Limit:       60,
			WaitSeconds: 5,
		},
	}, *f)

	opts := c.RetryOpts()
	assert.Equal(t, 60, opts.Limit, "Custom retry limit should be 60")
	assert.Equal(t, 5, opts.WaitSeconds, "Custom wait seconds should be 5")
}
