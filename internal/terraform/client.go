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

package terraform

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"

	"github.com/hashicorp/go-version"
	hcInstall "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/fs"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/releases"
	"github.com/hashicorp/hc-install/src"
	"github.com/hashicorp/terraform-exec/tfexec"
)

var (
	instance *TerraformClient
	tfOnce   sync.Once
)

// TerraformClient is a wrapper for the Terraform CLI, providing functionality for managing Terraform operations.
type TerraformClient struct {
	installer *hcInstall.Installer

	version  string
	execPath string
	cfg      config.Settings

	clientCache map[string]*tfexec.Terraform
}

// TfOpts represents options for configuring a Terraform instance.
type TfOpts struct {
	dir    string    // The directory where Terraform will operate.
	stdout io.Writer // The writer for standard output.
	stderr io.Writer // The writer for standard error.
}

// TerraformInitOpts represents options for initializing Terraform with backend configuration.
type TerraformInitOpts struct {
	BackendConfig []string // The backend configuration options.
}

// TfExecTerraformLogger defines the interface for configuring Terraform logging.
type TfExecTerraformLogger interface {
	SetLogPath(string) error // Sets the log file path.
	SetLog(string) error     // Sets the log level.
}

// Instance returns a singleton instance of the TerraformClient.
// It initializes the client if it has not been created already.
func Instance(ctx context.Context, cfg config.Settings) *TerraformClient {
	var t TerraformClient
	var err error
	tfOnce.Do(func() {
		t, err = NewTerraformClient(ctx, cfg)
		instance = &t
	})

	if err != nil {
		panic(err)
	}

	return instance
}

// ResetInstance resets the singleton instance of the TerraformClient.
// This is useful for testing to ensure a fresh client is created.
func ResetInstance() {
	tfOnce = sync.Once{}
}

// NewTerraformClient creates a new TerraformClient instance and ensures the Terraform CLI is available.
func NewTerraformClient(ctx context.Context, cfg config.Settings) (TerraformClient, error) {
	execPath, installer, err := install(ctx, cfg.Config.Terraform.Version, cfg.Config.Tmp)
	if err != nil {
		return TerraformClient{}, err
	}

	return TerraformClient{
		version:     cfg.Config.Terraform.Version,
		cfg:         cfg,
		installer:   installer,
		execPath:    execPath,
		clientCache: make(map[string]*tfexec.Terraform),
	}, nil
}

// Cleanup removes the downloaded Terraform CLI from the file system.
func (c *TerraformClient) Cleanup(ctx context.Context) error {
	if c.installer == nil {
		log.Debug("No installer instance configured, skipping...")
		return nil
	}

	log.Debug("Removing terraform installer")
	return c.installer.Remove(ctx)
}

// getTf retrieves a cached Terraform instance for the specified directory.
// If no instance exists, it creates a new one.
func (c *TerraformClient) getTf(dir string) (*tfexec.Terraform, error) {
	if i, found := c.clientCache[dir]; found {
		return i, nil
	}

	i, err := c.newTf(dir)
	if err != nil {
		return nil, err
	}

	c.clientCache[dir] = i
	return i, nil
}

// newTf creates a new Terraform instance for the specified directory with default options.
func (c *TerraformClient) newTf(dir string) (*tfexec.Terraform, error) {
	return c.newTfOpts(&TfOpts{dir: dir, stdout: os.Stdout, stderr: os.Stderr})
}

// newTfOpts creates a new Terraform instance with the specified options.
func (c *TerraformClient) newTfOpts(opts *TfOpts) (*tfexec.Terraform, error) {
	dir := opts.dir
	if len(dir) == 0 {
		log.Debug("TfOpts.dir not provided, defaulting to current directory")
		dir = "."
	}

	tf, err := tfexec.NewTerraform(dir, c.execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create terraform client %w", err)
	}

	if opts.stdout != nil {
		tf.SetStdout(opts.stdout)
	}

	if opts.stderr != nil {
		tf.SetStderr(opts.stderr)
	}

	initLog(tf, c.cfg.Config)

	return tf, nil
}

// install downloads and installs the specified version of the Terraform CLI.
// It returns the executable path, installer instance, and any error encountered.
func install(ctx context.Context, v string, dir string) (string, *hcInstall.Installer, error) {
	installer := hcInstall.NewInstaller()

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		log.Debug("Not found, creating", "dir", dir)
		os.Mkdir(dir, 0750) //nolint:errcheck
	}

	execPath, err := installer.Ensure(ctx, []src.Source{
		&fs.ExactVersion{
			Product:    product.Terraform,
			Version:    version.Must(version.NewVersion(v)),
			ExtraPaths: []string{dir},
		},
		&releases.ExactVersion{
			Product:    product.Terraform,
			Version:    version.Must(version.NewVersion(v)),
			InstallDir: dir,
		},
	})

	if err != nil {
		return "", installer, err
	}

	log.Debug("Terraform installed", "path", execPath)

	return execPath, installer, err
}

// initLog configures logging for the Terraform instance based on the Quartz configuration.
func initLog(tf TfExecTerraformLogger, cfg schema.QuartzConfig) {
	if !cfg.Log.Terraform.Enabled ||
		cfg.Log.Terraform.Path == "" {
		log.Debug("Terraform logging disabled")
		return
	}

	level := cfg.Log.Terraform.Level
	log.Debug("Attempting to configure Terraform log", "rawpath", cfg.Log.Terraform.Path, "level", level)

	path, _ := filepath.Abs(cfg.Log.Terraform.Path)
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0740) //nolint:errcheck

	now := time.Now()

	path = strings.ReplaceAll(path, "$name", cfg.Name)
	path = strings.ReplaceAll(path, "$date", now.Format("2006-01-02"))

	log.Info("Configuring Terraform log", "path", path, "level", level)
	if err := tf.SetLogPath(path); err != nil {
		log.Debug("Failed to set terraform log path", "err", err)
		return
	}

	if err := tf.SetLog(strings.ToUpper(level)); err != nil {
		log.Debug("Failed to set terraform log level", "err", err)
		return
	}
}
