package stages

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
)

// StateStageCheck represents a state-based stage check.
type StateStageCheck struct {
	src             schema.StageChecksStateConfig // The configuration for the state stage check.
	providerFactory provider.ProviderFactory      // The provider factory for accessing Kubernetes resources.
}

// NewStateStageCheck creates a new StateStageCheck instance with the specified configuration and provider factory.
func NewStateStageCheck(src schema.StageChecksStateConfig, providerFactory provider.ProviderFactory) StateStageCheck {
	return StateStageCheck{
		src:             src,
		providerFactory: providerFactory,
	}
}

// Run executes the state stage check by validating the state stored in a Kubernetes ConfigMap.
// It ensures the specified key exists and matches the expected value.
func (c StateStageCheck) Run(ctx context.Context, cfg schema.QuartzConfig) error {
	if !cfg.State.Enabled {
		log.Error("Quartz platform state tracking disabled, skipping stage check", "stage", c.Id())
		return nil
	}

	kube, err := c.providerFactory.Kubernetes(ctx)
	if err != nil {
		return err
	}

	log.Debug("Retrieving state configmap", "name", cfg.State.ConfigMapName, "namespace", cfg.State.ConfigMapNamespace)
	data, err := kube.GetConfigMapValue(ctx, cfg.State.ConfigMapNamespace, cfg.State.ConfigMapName)
	if err != nil {
		return err
	}

	log.Debug("Configmap found", "data", data)
	val, ok := data[c.src.Key]
	if !ok {
		log.Debug("Requested configmap key not found", "key", c.src.Key)
		return fmt.Errorf("key not found, %s", c.src.Key)
	}

	if !strings.EqualFold(val, c.src.Value) {
		log.Debug("Requested configmap key found but incorrect value", "key", c.src.Key, "expected", c.src.Value, "actual", val)
		return fmt.Errorf("value failed to match, expected %s, found %s", c.src.Value, val)
	}

	log.Debug("State check succeeded", "id", c.Id(), "key", c.src.Key, "value", c.src.Value)
	return nil
}

// Id returns the unique identifier of the state stage check.
// The identifier includes the key and expected value being checked.
func (c StateStageCheck) Id() string {
	return fmt.Sprintf("%s - %s", c.src.Key, c.src.Value)
}

// Type returns the type of the stage check, which is "state".
func (c StateStageCheck) Type() string {
	return "state"
}

// RetryOpts returns the retry configuration for the state stage check.
// It includes the retry limit and wait time between retries.
func (c StateStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	r := c.src.Retry.Limit
	if r <= 0 {
		r = math.MaxInt // go forever if not specified
	}

	w := c.src.Retry.WaitSeconds
	if w <= 0 {
		w = 5 // 5 second default
	}

	return schema.StageChecksRetryConfig{
		Limit:       r,
		WaitSeconds: w,
	}
}
