package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"sigs.k8s.io/yaml"
)

func pinInstallAvailabilityZones(ctx context.Context, p *CommandParams) error {
	return pinAvailabilityZones(ctx, p, func(ctx context.Context, region string) ([]string, error) {
		output, err := exec.CommandContext(ctx, "aws", "ec2", "describe-availability-zones",
			"--region", region, "--filters",
			"Name=zone-type,Values=availability-zone",
			"Name=opt-in-status,Values=opt-in-not-required",
			"Name=state,Values=available",
			"--query", "AvailabilityZones[].ZoneName", "--output", "json").Output()
		if err != nil {
			return nil, fmt.Errorf("discover ordered availability zones: %w", err)
		}
		var names []string
		if err := json.Unmarshal(output, &names); err != nil {
			return nil, fmt.Errorf("decode availability-zone discovery: %w", err)
		}
		return names, nil
	})
}

func pinAvailabilityZones(ctx context.Context, p *CommandParams, discover func(context.Context, string) ([]string, error)) error {
	settings := p.Settings()
	if settings.Config.Providers.Cloud != "aws" {
		return nil
	}
	awsConfig := settings.Config.Aws
	if len(awsConfig.AvailabilityZones) > 0 || len(awsConfig.AvailabilityZoneNames) > 0 {
		return nil
	}
	original, err := os.ReadFile(p.configFile)
	if err != nil {
		return fmt.Errorf("zone identity persistence requires a writable Quartz config: %w", err)
	}
	info, err := os.Lstat(p.configFile)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("zone identity persistence requires a regular Quartz config file")
	}
	names, err := discover(ctx, awsConfig.Region)
	if err != nil {
		return err
	}
	if err := validateZoneSnapshot(awsConfig, names); err != nil {
		return err
	}
	var document map[string]interface{}
	if err := yaml.Unmarshal(original, &document); err != nil {
		return err
	}
	awsDocument, ok := document["aws"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("zone identity persistence requires an aws config mapping")
	}
	for _, key := range []string{"availability_zone_names", "availability_zones"} {
		if existing, ok := awsDocument[key]; ok && existing != nil {
			// An explicitly empty list is the existing discovery-fallback contract.
			if values, ok := existing.([]interface{}); !ok || len(values) != 0 {
				return fmt.Errorf("config changed during discovery; reload before persisting zone identities")
			}
		}
	}
	awsDocument["availability_zone_names"] = names
	updated, err := yaml.Marshal(document)
	if err != nil {
		return err
	}
	// Exclusive staging avoids partial config writes or concurrent installer overwrite.
	staging := p.configFile + ".zone-identities"
	file, err := os.OpenFile(staging, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	persisted := false
	defer func() {
		if !persisted {
			os.Remove(staging)
		}
	}()
	_, writeErr := file.Write(updated)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	current, err := os.ReadFile(p.configFile)
	if err != nil {
		return err
	}
	if string(current) != string(original) {
		return fmt.Errorf("config changed during discovery; refusing zone identity overwrite")
	}
	if err := os.Rename(staging, p.configFile); err != nil {
		return err
	}
	persisted = true
	return settings.SetAvailabilityZoneNames(names)
}

func validateZoneSnapshot(config schema.AwsConfig, names []string) error {
	if len(names) < 2 {
		return fmt.Errorf("at least two available regional zone identities are required")
	}
	seen := make(map[string]bool)
	for _, name := range names {
		suffix := strings.TrimPrefix(name, config.Region)
		if config.Region == "" || len(suffix) != 1 || suffix[0] < 'a' || suffix[0] > 'z' || seen[name] {
			return fmt.Errorf("discovery returned invalid or duplicate regional zone identities")
		}
		seen[name] = true
	}
	return nil
}
