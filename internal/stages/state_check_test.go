package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"

	corev1 "k8s.io/api/core/v1"
)

func TestStagesStateCheckRunHappy(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	cm := corev1.ConfigMap{}
	cm.Name = cfg.State.ConfigMapName
	cm.Namespace = cfg.State.ConfigMapNamespace
	cm.Data = map[string]string{
		"key1": "match",
	}

	api := provider.NewKubernetesApiMock().WithClientObjects(&cm)
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewStateStageCheck(schema.StageChecksStateConfig{
		Key:   "key1",
		Value: "match",
	}, *f)

	id := c.Id()
	tp := c.Type()
	opts := c.RetryOpts()
	if id == "" ||
		tp == "" ||
		opts.Limit < 0 {
		t.Errorf("unexpected properties of state check, %v, %v, %v", id, tp, opts)
	}

	err := c.Run(context.Background(), cfg)
	if err != nil {
		t.Errorf("unexpected error in state check, %v", err)
		return
	}
}

func TestStagesStateCheckRunDisabled(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.StateConfig{
			Enabled: false,
		},
	}

	c := NewStateStageCheck(schema.StageChecksStateConfig{
		Key:   "key1",
		Value: "match",
	}, provider.ProviderFactory{})

	err := c.Run(context.Background(), cfg)
	if err != nil {
		t.Errorf("unexpected error in state check (disabled), %v", err)
		return
	}
}

func TestStagesStateCheckRunMissingKey(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	cm := corev1.ConfigMap{}
	cm.Name = cfg.State.ConfigMapName
	cm.Namespace = cfg.State.ConfigMapNamespace
	cm.Data = map[string]string{
		"key1": "match",
	}

	api := provider.NewKubernetesApiMock().WithClientObjects(&cm)
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewStateStageCheck(schema.StageChecksStateConfig{
		Key:   "key2",
		Value: "",
	}, *f)

	err := c.Run(context.Background(), cfg)
	if err == nil {
		t.Error("expected error in state check not found (missing key)")
		return
	}

	if !strings.Contains(err.Error(), "key not found") {
		t.Errorf("invalid error message, expected %s, found %v", "key not found", err)
	}
}

func TestStagesStateCheckRunMismatchedValue(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	cm := corev1.ConfigMap{}
	cm.Name = cfg.State.ConfigMapName
	cm.Namespace = cfg.State.ConfigMapNamespace
	cm.Data = map[string]string{
		"key1": "match",
	}

	api := provider.NewKubernetesApiMock().WithClientObjects(&cm)
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewStateStageCheck(schema.StageChecksStateConfig{
		Key:   "key1",
		Value: "mismatched",
	}, *f)

	err := c.Run(context.Background(), cfg)
	if err == nil {
		t.Error("expected error in state check not found (mismatched)")
		return
	}

	if !strings.Contains(err.Error(), "value failed to match") {
		t.Errorf("invalid error message, expected %s, found %v", "value failed to match", err)
	}
}

func TestStagesStateCheckRunConfigmapNotFound(t *testing.T) {
	cfg := schema.QuartzConfig{
		State: schema.NewStateConfig(),
	}

	cm := corev1.ConfigMap{}
	cm.Name = cfg.State.ConfigMapName + "foobar"
	cm.Namespace = cfg.State.ConfigMapNamespace
	cm.Data = map[string]string{
		"key1": "match",
	}

	api := provider.NewKubernetesApiMock().WithClientObjects(&cm)
	kubeconfig := provider.KubeconfigInfo{}
	k8s, _ := provider.NewKubernetesClient(api, kubeconfig, cfg)
	f := provider.NewProviderFactory(cfg, schema.QuartzSecrets{}, provider.WithKubernetesProvider(k8s))

	c := NewStateStageCheck(schema.StageChecksStateConfig{
		Key:   "key1",
		Value: "match",
	}, *f)

	err := c.Run(context.Background(), cfg)
	if err == nil {
		t.Error("expected error in state check not found (configmap not found)")
		return
	}

	if !strings.Contains(err.Error(), "configmaps \"quartz-install-state\" not found") {
		t.Errorf("invalid error message, expected %s, found %v", "configmaps \"quartz-install-state\" not found", err)
	}
}
