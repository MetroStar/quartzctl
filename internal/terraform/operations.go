package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/tidwall/gjson"
)

// Version retrieves the version of the Terraform CLI.
// It runs the `terraform version` command and returns the version string.
func (c *TerraformClient) Version(ctx context.Context) (string, error) {
	log.Debug("terraform version")

	tf, err := c.newTfOpts(&TfOpts{})
	if err != nil {
		return "", err
	}

	tfVersion, _, err := tf.Version(ctx, true)
	if err != nil {
		return "", err
	}

	return tfVersion.String(), nil
}

// Init initializes the Terraform working directory for the specified stage.
// It runs `terraform init -upgrade -reconfigure` with the provided backend configuration options.
func (c *TerraformClient) Init(ctx context.Context, stage schema.StageConfig, opts TerraformInitOpts) error {
	log.Debug("terraform init", "stage", stage)

	var args []tfexec.InitOption
	args = append(args, tfexec.Upgrade(true))
	args = append(args, tfexec.Reconfigure(true))
	for _, bc := range opts.BackendConfig {
		args = append(args, tfexec.BackendConfig(bc))
	}

	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	c.setStageEnv(tf, stage)
	return tf.Init(ctx, args...)
}

// Validate validates the Terraform configuration for the specified stage.
// It runs `terraform validate` and returns the validation output.
func (c *TerraformClient) Validate(ctx context.Context, stage schema.StageConfig) (*tfjson.ValidateOutput, error) {
	log.Debug("terraform validate", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return nil, err
	}
	return tf.Validate(ctx)
}

// Format formats the Terraform configuration files in the specified stage directory.
// It runs `terraform fmt -recursive`.
func (c *TerraformClient) Format(ctx context.Context, stage schema.StageConfig) error {
	log.Debug("terraform fmt", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	return tf.FormatWrite(ctx, tfexec.Recursive(true))
}

// Plan creates an execution plan for the specified stage.
// It runs `terraform plan` with the configured input variables and returns whether changes are required.
func (c *TerraformClient) Plan(ctx context.Context, stage schema.StageConfig) (bool, error) {
	log.Debug("terraform plan", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return false, err
	}

	var vars []tfexec.PlanOption
	for _, v := range c.stageVars(ctx, stage) {
		vars = append(vars, v)
	}
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	c.setStageEnv(tf, stage)
	return tf.Plan(ctx, vars...)
}

// Apply applies the Terraform configuration for the specified stage.
// It runs `terraform apply` with the configured input variables.
func (c *TerraformClient) Apply(ctx context.Context, stage schema.StageConfig) error {
	if stage.Debug.Break {
		util.Msgf("Break point at stage %s", stage.Id)
		return fmt.Errorf("break")
	}

	log.Debug("terraform apply", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	var vars []tfexec.ApplyOption
	for _, v := range c.stageVars(ctx, stage) {
		vars = append(vars, v)
	}
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	c.setStageEnv(tf, stage)

	return tf.Apply(ctx, vars...)
}

// Destroy destroys the Terraform-managed infrastructure for the specified stage.
// It runs `terraform destroy` with the configured input variables and targets.
func (c *TerraformClient) Destroy(ctx context.Context, stage schema.StageConfig) error {
	if stage.Debug.Break {
		util.Msgf("Break point at stage %s", stage.Id)
		return fmt.Errorf("break")
	}

	if stage.Destroy.Skip {
		util.Msgf("Destruction skipped for stage %s", stage.Id)
		return nil
	}

	log.Debug("terraform destroy", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	var vars []tfexec.DestroyOption
	vars = append(vars, tfexec.Refresh(false))
	for _, v := range c.stageVars(ctx, stage) {
		vars = append(vars, v)
	}
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}

	targets, found, err := targetsToDestroy(ctx, tf, stage)
	if err != nil {
		return err
	}

	if !found {
		log.Info("No matching state entries found, bypassing destroy", "stage", stage.Id)
		return nil
	}

	for _, t := range targets {
		log.Warn("Explicitly destroying", "resource", t, "stage", stage.Id)
		vars = append(vars, tfexec.Target(t))
	}

	c.setStageEnv(tf, stage)
	return tf.Destroy(ctx, vars...)
}

// Refresh updates the Terraform state for the specified stage.
// It runs `terraform refresh` with the configured input variables.
func (c *TerraformClient) Refresh(ctx context.Context, stage schema.StageConfig) error {
	log.Debug("terraform refresh", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	var vars []tfexec.RefreshCmdOption
	for _, v := range c.stageVars(ctx, stage) {
		vars = append(vars, v)
	}
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	c.setStageEnv(tf, stage)
	return tf.Refresh(ctx, vars...)
}

// Output retrieves the Terraform output for the specified stage directory.
// It returns a map of output variable names to their values in JSON format.
func (c *TerraformClient) Output(ctx context.Context, stage schema.StageConfig) (map[string][]byte, error) {
	log.Debug("terraform output", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return nil, err
	}

	output, err := tf.Output(ctx)
	if err != nil {
		return nil, err
	}

	res := make(map[string][]byte)

	for k, v := range output {
		vp, _ := json.MarshalIndent(v.Value, "", " ")
		res[k] = vp
	}

	return res, nil
}

// setStageEnv sets the environment variables for the Terraform process based on the stage configuration.
func (c *TerraformClient) setStageEnv(tf *tfexec.Terraform, stage schema.StageConfig) {
	env := util.OsEnvMap()

	if stage.Providers.Kubernetes {
		env = util.MergeMaps(env, map[string]string{
			"KUBE_CONFIG_PATH": c.cfg.Config.KubeconfigPath(),
		})
	}

	if err := tf.SetEnv(env); err != nil {
		log.Warn("Failed to set terraform environment", "stage", stage, "err", err)
	}
}

// stageVars generates the input variables for the specified stage based on its configuration.
// It supports literal values, environment variables, configuration values, secrets, and outputs from other stages.
func (c *TerraformClient) stageVars(ctx context.Context, stage schema.StageConfig) []*tfexec.VarOption {
	var vars []*tfexec.VarOption

	outputs := make(map[string]map[string][]byte)

	log.Debug("Adding stage vars", "stage", stage)

	for k, v := range stage.Vars {
		if v.Value != "" {
			log.Debug("Terraform literal input var", "val", v.Value)
			vars = append(vars, tfexec.Var(fmt.Sprintf("%s=%s", k, v.Value)))
		} else if v.Env != "" {
			val, found := os.LookupEnv(v.Env)
			if !found {
				log.Info("Stage env input not found", "stage", stage, "env", v.Env)
			}

			log.Debug("Terraform env input var", "key", v.Env, "val", val)
			vars = append(vars, tfexec.Var(fmt.Sprintf("%s=%s", k, val)))
		} else if v.Config != "" {
			val := c.cfg.ConfigString(v.Config)
			if val == "" {
				log.Info("Stage config input not found", "stage", stage, "config", v.Config)
				continue
			}

			log.Debug("Terraform config var", "key", v.Config, "val", val)
			vars = append(vars, tfexec.Var(fmt.Sprintf("%s=%s", k, val)))
		} else if v.Secret != "" {
			val := c.cfg.SecretString(v.Secret)
			if val == "" {
				log.Info("Stage secret input not found", "stage", stage, "secret", v.Secret)
				continue
			}

			log.Debug("Terraform secret input var", "key", v.Secret, "val", val)
			vars = append(vars, tfexec.Var(fmt.Sprintf("%s=%s", k, val)))
		} else if v.Stage.Name != "" {
			_, ok := outputs[v.Stage.Name]
			if !ok {
				s := c.cfg.Config.Stages[v.Stage.Name]
				o, err := c.Output(ctx, s)
				if err != nil {
					log.Warn("Failed to get output", "stage", s.Id, "err", err)
					continue
				}

				outputs[v.Stage.Name] = o
			}

			val, err := parseStageOutputValue(outputs[v.Stage.Name], v.Stage.Output)
			if err != nil {
				log.Warn("Failed to parse stage output", "stage", v.Stage.Name, "output", v.Stage.Output)
				continue
			}

			log.Debug("Terraform stage input var", "stage", v.Stage.Name, "output", v.Stage.Output, "key", k, "val", val)
			vars = append(vars, tfexec.Var(fmt.Sprintf("%s=%s", k, val)))
		}
	}

	return vars
}

// parseStageOutputValue parses a specific output value from the Terraform state of another stage.
// It supports nested keys using dot notation.
func parseStageOutputValue(o map[string][]byte, key string) (string, error) {
	before, after, found := strings.Cut(key, ".")

	v, f := o[before]
	if !f {
		return "", fmt.Errorf("key not found")
	}

	val := string(v)

	// if multiple components to the key, try to extract sub key(s)
	if found {
		r := gjson.Get(val, after)
		val = r.String()
	}

	return strings.Trim(val, "\""), nil
}

// targetsToDestroy determines the specific resources to destroy for the specified stage.
// It applies inclusion and exclusion filters based on the stage configuration.
func targetsToDestroy(ctx context.Context, tf *tfexec.Terraform, stage schema.StageConfig) ([]string, bool, error) {
	hasIncludes := len(stage.Destroy.Include) > 0
	hasExcludes := len(stage.Destroy.Exclude) > 0

	if !hasIncludes && !hasExcludes {
		// nothing specified, default to a normal terraform destroy
		return nil, true, nil
	}

	state, err := tf.Show(ctx)
	if err != nil {
		return nil, false, err
	}

	if state == nil ||
		state.Values == nil ||
		state.Values.RootModule == nil ||
		(len(state.Values.RootModule.Resources) == 0 && len(state.Values.RootModule.ChildModules) == 0) {
		log.Debug("Empty state response from module", "stage", stage.Id, "state", state)
		return nil, false, nil
	}

	var targets []string
	checkResource := func(res *tfjson.StateResource) {
		log.Debug("Checking terraform state resource for explicit inclusion/exclusion in destroy operation", "address", res.Address)
		compFunc := func(e string) bool {
			return util.EqualsOrRegexMatchString(e, res.Address, true)
		}

		if hasExcludes && slices.ContainsFunc(stage.Destroy.Exclude, compFunc) {
			// exclude has priority, if it's in this list, skip it
			log.Debug("Matched entry in exclusion list, removing resource from destroy set", "address", res.Address)
			return
		}

		if hasIncludes && !slices.ContainsFunc(stage.Destroy.Include, compFunc) {
			// include has anything at all and doesn't contain this resource, skip it
			log.Debug("Did not match anything in inclusion list, removing resource from destroy set", "address", res.Address)
			return
		}

		// if we got this far, we want it included in the destroy
		log.Debug("Adding resource to destroy set", "address", res.Address)
		targets = append(targets, res.Address)
	}

	var checkModule func(*tfjson.StateModule)
	checkModule = func(mod *tfjson.StateModule) {
		for _, res := range mod.Resources {
			checkResource(res)
		}

		for _, mod := range mod.ChildModules {
			log.Debug("Recursively checking child module", "address", mod.Address)
			checkModule(mod)
		}
	}

	log.Debug("Checking root module", "address", state.Values.RootModule.Address)
	checkModule(state.Values.RootModule)

	// filters were applied but the result set was empty, skip the destroy
	if len(targets) == 0 {
		return nil, false, nil
	}

	targets = util.DistinctSlice(targets)

	return targets, true, nil
}
