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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestStagesKubernetesCheckRun(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	o := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "group/version",
			"kind":       "TestThing",
			"metadata": map[string]interface{}{
				"namespace": "test-ns",
				"name":      "test-obj",
			},
		},
	}
	unstructured.SetNestedSlice(o.Object, []interface{}{
		map[string]interface{}{"status": "True", "type": "FOOBAR?"},
	}, "status", "conditions")

	api := provider.NewKubernetesApiMock().
		WithDynamicObjects(o).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "group/version",
			APIResources: []metav1.APIResource{
				{Name: "testthings", Namespaced: true, Kind: "TestThing"},
			},
		})
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewKubernetesStageCheck(schema.StageChecksKubernetesConfig{
		Name:      "test-obj",
		Namespace: "test-ns",
		Kind:      "TestThing",
		State:     "FOOBAR?",
		Timeout:   1,
	}, *f)

	id := c.Id()
	tp := c.Type()
	opts := c.RetryOpts()
	if id == "" ||
		tp == "" ||
		opts.Limit < 0 {
		t.Errorf("unexpected properties of kubernetes check, %v, %v, %v", id, tp, opts)
	}

	err := c.Run(context.Background(), cfg)
	if err != nil {
		t.Errorf("unexpected error in kubernetes check, %v", err)
		return
	}
}
