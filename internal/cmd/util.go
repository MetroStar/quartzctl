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

package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/stages"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

var (
	// checkOpts defines options for health checks, including callbacks for start, completion, and retries.
	checkOpts = &stages.CheckOpts{
		OnStart:    onCheckStart,
		OnComplete: onCheckComplete,
		OnRetry:    onCheckRetry,
	}
)

func NewRootLoginCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "login",
			Usage: "Generate a kubeconfig for the current cluster",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output path", Value: "./out/kubeconfig"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				path := ccmd.String("out")
				return ClusterLogin(ctx, path, p)
			},
		},
	}
}

func NewRootInfoCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "info",
			Usage: "Output configuration info for the current cluster",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return ClusterInfo(ctx, p)
			},
		},
	}
}

func NewRootCheckCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "check",
			Usage: "Check environment and configuration for required values",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				Check(ctx, p)
				return nil
			},
		},
	}
}

func NewRootRenderCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "render",
			Usage: "Write fully rendered yaml config",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output path", Value: "./out/quartz.generated.yaml"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				path := ccmd.String("out")
				return Render(ctx, path, p)
			},
		},
	}
}

func NewRootRefreshSecretsCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:    "refresh-secrets",
			Aliases: []string{"rs"},
			Usage:   "Trigger all external secrets to be refreshed immediately",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return RefreshSecrets(ctx, p)
			},
		},
	}
}

func NewRootExportCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "export",
			Usage: "Export configured Kubernetes resources to yaml",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return Export(ctx, p)
			},
		},
	}
}

func NewRootRestartCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "restart",
			Usage: "Restart target resource(s)",
			Flags: []cli.Flag{
				&cli.StringSliceFlag{Name: "kind", Aliases: []string{"k"}, Usage: "Resource kind", Required: false},
				&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Namespace", Required: false},
				&cli.StringFlag{Name: "name", Usage: "Name", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				kinds := ccmd.StringSlice("kind")
				ns := ccmd.String("namespace")
				name := ccmd.String("name")

				if len(kinds) == 0 {
					kinds = []string{"deployment", "daemonset", "statefulset"}
				}

				for _, k := range kinds {
					err := Restart(ctx, k, ns, name, p)
					if err != nil {
						return err
					}
				}

				return nil
			},
		},
	}
}

// Version displays the version of the Quartz installer along with the build date.
//
// Parameters:
//   - version: The version of the Quartz installer.
//   - buildDate: The build date of the Quartz installer.
func Version(version string, buildDate string) {
	log.Debug("Entering", "command", "version")
	defer log.Debug("Completed", "command", "version")

	var format = "2006-01-02 15:04 MST"

	d := buildDate
	if d == "" {
		d = time.Now().UTC().Format(format)
	} else {
		d, _, _ = strings.Cut(d, ".") // in case a float was passed in
		c, err := strconv.ParseInt(d, 10, 64)
		if err != nil {
			panic(err)
		}
		d = time.Unix(c, 0).Format(format)
	}
	util.Msgf("Quartz %s\nBuild Date: %s\n", version, d)
}

// Render writes the full configuration to the specified file path.
//
// Parameters:
//   - ctx: The context for the operation.
//   - path: The file path where the configuration will be written.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the rendering fails, otherwise nil.
func Render(ctx context.Context, path string, p *CommandParams) error {
	log.Debug("Entering", "command", "render")
	defer log.Debug("Completed", "command", "render")

	f, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	util.Msgf("Writing generated Quartz YAML to %s", f)
	return p.Settings().WriteYamlConfig(f)
}

// ClusterInfo retrieves and displays information about the Quartz cluster.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if retrieving cluster information fails, otherwise nil.
func ClusterInfo(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "clusterInfo")
	defer log.Debug("Completed", "command", "clusterInfo")

	if !p.Settings().Config.Internal.Installer.Summary.Enabled {
		log.Warn("Summary disabled in config")
		return nil
	}

	util.Hdr("Cluster summary")
	cp, _ := p.Provider().Cloud(ctx)
	err := cp.PrintClusterInfo(ctx)
	if err != nil {
		return err
	}

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}
	k8s.PrintClusterInfo(ctx)

	util.Msgf("export KUBECONFIG=%s", p.Settings().Config.KubeconfigPath())
	util.Msg("CI/CD builds may take up to 15 minutes to complete following initial setup, progress may be tracked at the Jenkins and ArgoCD URL's above")

	return err
}

// ClusterLogin generates a kubeconfig file for the Quartz environment.
//
// Parameters:
//   - ctx: The context for the operation.
//   - path: The file path where the kubeconfig will be written.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if generating the kubeconfig fails, otherwise nil.
func ClusterLogin(ctx context.Context, path string, p *CommandParams) error {
	log.Debug("Entering", "command", "clusterLogin")
	defer log.Debug("Completed", "command", "clusterLogin")

	util.Hdr("Generate kubeconfig")

	k8sClient, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	if path == "" {
		path = p.Settings().Config.KubeconfigPath()
	}

	err = k8sClient.WriteKubeconfigFile(path)
	if err != nil {
		util.Msgf("Failed to write kubeconfig %v", err)
		return nil // suppressing error here, if the cluster is unavailable it's not always a blocker downstream
	}

	util.Msgf("Kubeconfig written to %s", path)
	return nil
}

// Check verifies the dependencies required for Quartz installation.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
func Check(ctx context.Context, p *CommandParams) {
	log.Debug("Entering", "command", "check")
	defer log.Debug("Completed", "command", "check")

	util.Hdr("Check")

	opts := provider.NewProviderCheckOpts(ctx, *p.Provider())
	provider.Check(ctx, &opts)
}

// RefreshSecrets triggers an immediate refresh of all external secrets.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if refreshing secrets fails, otherwise nil.
func RefreshSecrets(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "refreshSecrets")
	defer log.Debug("Completed", "command", "refreshSecrets")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	_, err = k8s.RefreshExternalSecrets(ctx)
	if err != nil {
		util.Errorf("Failed to refresh secrets, %v", err)
	}

	return nil
}

// Cleanup removes temporary files created by the installer.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if cleanup fails, otherwise nil.
func Cleanup(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "cleanup")
	defer log.Debug("Completed", "internal", "cleanup")

	err := os.RemoveAll(p.Settings().Config.Tmp)
	if err != nil {
		log.Warn("Error during cleanup", "err", err)
	}
	return nil
}

// Banner displays the Quartz banner.
func Banner() {
	log.Debug("Entering", "internal", "banner")
	defer log.Debug("Completed", "internal", "banner")

	util.PrintBanner()
}

// Confirm prompts the user for confirmation before proceeding with an operation.
//
// Parameters:
//   - ctx: The context for the operation.
//   - msg: The confirmation message to display.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the user does not confirm, otherwise nil.
func Confirm(ctx context.Context, msg string, p *CommandParams) error {
	log.Debug("Entering", "internal", "confirm")
	defer log.Debug("Completed", "internal", "confirm")

	cp, _ := p.Provider().Cloud(ctx)
	cp.PrintConfig()

	util.Msgf("Domain: %s\n", p.Settings().Config.Dns.Domain)

	if r := util.PromptYesNo(msg); !r {
		return fmt.Errorf("aborting")
	}

	return nil
}

// Export saves the configured Kubernetes resources to YAML files.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if exporting resources fails, otherwise nil.
func Export(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "export")
	defer log.Debug("Completed", "command", "export")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	res, err := k8s.Export(ctx, p.Settings().Config.Export)
	if err != nil {
		return err
	}

	out := p.Settings().Config.Export.Path
	for k, v := range res {
		err := util.WriteBytesToFile(v, path.Join(out, p.Settings().Config.Dns.Domain, k))
		if err != nil {
			return err
		}
	}

	return nil
}

// PrepareAccount prepares the cloud account for Quartz operations.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if preparing the account fails, otherwise nil.
func PrepareAccount(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "prepareAccount")
	defer log.Debug("Completed", "internal", "prepareAccount")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.PrepareAccount(ctx)
}

// Restart restarts a Kubernetes resource in the specified namespace.
//
// Parameters:
//   - ctx: The context for the operation.
//   - res: The resource type to restart (e.g., deployment, daemonset).
//   - ns: The namespace of the resource.
//   - name: The name of the resource.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if restarting the resource fails, otherwise nil.
func Restart(ctx context.Context, res string, ns string, name string, p *CommandParams) error {
	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	kind, err := k8s.LookupKind(ctx, res)
	if err != nil {
		return err
	}

	return k8s.Restart(ctx, kind, ns, name)
}

// onCheckStart logs the start of a health check for a stage.
//
// Parameters:
//   - cr: The result of the health check.
func onCheckStart(cr stages.CheckResult) {
	util.Msgf("Starting %s check for stage %s - %s", cr.Type, cr.Stage, cr.Id)
}

// onCheckComplete logs the completion of a health check for a stage.
//
// Parameters:
//   - cr: The result of the health check.
func onCheckComplete(cr stages.CheckResult) {
	log.Debug("Health check complete", "result", cr)
	if cr.Error != nil {
		util.Errorf("Error running %s check for stage %s - %s [%v]", cr.Type, cr.Stage, cr.Id, cr.Error)
	} else {
		util.Msgf("Completed %s check for stage %s - %s", cr.Type, cr.Stage, cr.Id)
	}
}

// onCheckRetry logs a retry attempt for a health check.
//
// Parameters:
//   - cr: The result of the health check.
//   - i: The retry attempt number.
func onCheckRetry(cr stages.CheckResult, i int) {
	util.Printf("Retrying %s check for stage %s - %s (%d)", cr.Type, cr.Stage, cr.Id, i)
}
