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

package tofu

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"

	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/opentofu/tofudl"
)

var (
	instance *TofuClient
	tfOnce   sync.Once
)

// TofuClient is a wrapper for the OpenTofu CLI, providing functionality for managing OpenTofu operations.
type TofuClient struct {
	version  string
	execPath string
	cfg      config.Settings

	clientCache map[string]*tfexec.Terraform
}

// TfOpts represents options for configuring an OpenTofu instance.
type TfOpts struct {
	dir    string    // The directory where OpenTofu will operate.
	stdout io.Writer // The writer for standard output.
	stderr io.Writer // The writer for standard error.
}

// TofuInitOpts represents options for initializing OpenTofu with backend configuration.
type TofuInitOpts struct {
	BackendConfig []string // The backend configuration options.
}

// TfExecLogger defines the interface for configuring OpenTofu logging.
type TfExecLogger interface {
	SetLogPath(string) error // Sets the log file path.
	SetLog(string) error     // Sets the log level.
}

// Instance returns a singleton instance of TofuClient.
// It initializes the client if it has not been created already.
func Instance(ctx context.Context, cfg config.Settings) *TofuClient {
	var t TofuClient
	var err error
	tfOnce.Do(func() {
		t, err = NewTofuClient(ctx, cfg)
		instance = &t
	})

	if err != nil {
		panic(err)
	}

	return instance
}

// ResetInstance resets the singleton instance of TofuClient.
// This is useful for testing to ensure a fresh client is created.
func ResetInstance() {
	tfOnce = sync.Once{}
}

// NewTofuClient creates a new TofuClient and ensures the OpenTofu CLI binary is available.
func NewTofuClient(ctx context.Context, cfg config.Settings) (TofuClient, error) {
	execPath, err := install(ctx, cfg.Config.Tofu.Version, cfg.Config.Tmp)
	if err != nil {
		return TofuClient{}, err
	}

	return TofuClient{
		version:     cfg.Config.Tofu.Version,
		cfg:         cfg,
		execPath:    execPath,
		clientCache: make(map[string]*tfexec.Terraform),
	}, nil
}

// Cleanup removes the downloaded OpenTofu CLI from the file system.
func (c *TofuClient) Cleanup(ctx context.Context) error {
	if c.execPath == "" {
		log.Debug("No exec path configured, skipping...")
		return nil
	}

	log.Debug("Removing OpenTofu binary", "path", c.execPath)
	return os.Remove(c.execPath)
}

// getTf retrieves a cached OpenTofu instance for the specified directory.
// If no instance exists, it creates a new one.
func (c *TofuClient) getTf(dir string) (*tfexec.Terraform, error) {
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

// newTf creates a new OpenTofu instance for the specified directory with default options.
func (c *TofuClient) newTf(dir string) (*tfexec.Terraform, error) {
	return c.newTfOpts(&TfOpts{dir: dir, stdout: os.Stdout, stderr: os.Stderr})
}

// newTfOpts creates a new OpenTofu instance with the specified options.
func (c *TofuClient) newTfOpts(opts *TfOpts) (*tfexec.Terraform, error) {
	dir := opts.dir
	if len(dir) == 0 {
		log.Debug("TfOpts.dir not provided, defaulting to current directory")
		dir = "."
	}

	tf, err := tfexec.NewTerraform(dir, c.execPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tofu client %w", err)
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

// install downloads and installs the specified version of the OpenTofu CLI.
// It returns the executable path and any error encountered.
func install(ctx context.Context, v string, dir string) (string, error) {
	binaryName := "tofu"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	if err := os.MkdirAll(dir, 0750); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	execPath := filepath.Join(dir, binaryName)

	// If the binary already exists, reuse it
	if _, err := os.Stat(execPath); err == nil {
		log.Debug("OpenTofu binary found", "path", execPath)
		return execPath, nil
	}

	dl, err := tofudl.New()
	if err != nil {
		return "", fmt.Errorf("failed to initialize OpenTofu downloader: %w", err)
	}

	binary, err := dl.Download(ctx, tofudl.DownloadOptVersion(tofudl.Version(v)))
	if err != nil {
		return "", fmt.Errorf("failed to download OpenTofu %s: %w", v, err)
	}

	if err := os.WriteFile(execPath, binary, 0750); err != nil {
		return "", fmt.Errorf("failed to write OpenTofu binary: %w", err)
	}

	log.Debug("OpenTofu installed", "path", execPath)

	return execPath, nil
}

// initLog configures logging for the OpenTofu instance based on the Quartz configuration.
func initLog(tf TfExecLogger, cfg schema.QuartzConfig) {
	if !cfg.Log.Tofu.Enabled ||
		cfg.Log.Tofu.Path == "" {
		log.Debug("OpenTofu logging disabled")
		return
	}

	level := cfg.Log.Tofu.Level
	log.Debug("Attempting to configure OpenTofu log", "rawpath", cfg.Log.Tofu.Path, "level", level)

	path, _ := filepath.Abs(cfg.Log.Tofu.Path)
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0740) //nolint:errcheck

	now := time.Now()

	path = strings.ReplaceAll(path, "$name", cfg.Name)
	path = strings.ReplaceAll(path, "$date", now.Format("2006-01-02"))

	// Compress and archive any pre-existing log at this path so a re-run on the
	// same day starts fresh instead of appending without bound. Keeps each run's
	// OpenTofu log isolated and the on-disk footprint small.
	rotateExistingTofuLog(path)

	log.Info("Configuring OpenTofu log", "path", path, "level", level)
	if err := tf.SetLogPath(path); err != nil {
		log.Debug("Failed to set tofu log path", "err", err)
		return
	}

	if err := tf.SetLog(strings.ToUpper(level)); err != nil {
		log.Debug("Failed to set tofu log level", "err", err)
		return
	}
}

// rotateExistingTofuLog compresses a pre-existing OpenTofu log file (gzip) to
// `<path>.<unixts>.gz` and removes the original, so the next run starts with a
// fresh, empty log. No-op when the file is absent or empty. All failures are
// non-fatal — logging must never block an install/destroy.
func rotateExistingTofuLog(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() == 0 {
		return
	}

	src, err := os.Open(path) // #nosec G304 -- path derived from trusted config
	if err != nil {
		log.Debug("Failed to open existing tofu log for rotation", "err", err)
		return
	}
	defer src.Close() //nolint:errcheck

	archivePath := fmt.Sprintf("%s.%d.gz", path, time.Now().Unix())
	dst, err := os.OpenFile(archivePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640) // #nosec G304
	if err != nil {
		log.Debug("Failed to create rotated tofu log archive", "err", err)
		return
	}
	defer dst.Close() //nolint:errcheck

	gz := gzip.NewWriter(dst)
	if _, err := io.Copy(gz, src); err != nil { // #nosec G110 -- local trusted log file
		log.Debug("Failed to compress rotated tofu log", "err", err)
		_ = gz.Close()
		return
	}
	if err := gz.Close(); err != nil {
		log.Debug("Failed to finalize rotated tofu log archive", "err", err)
		return
	}

	if err := os.Remove(path); err != nil {
		log.Debug("Failed to remove rotated tofu log original", "err", err)
		return
	}

	log.Debug("Rotated existing OpenTofu log", "archive", archivePath, "bytes", info.Size())
}
