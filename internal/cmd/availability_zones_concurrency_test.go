package cmd

import (
	"context"
	"os"
	"testing"
)

func TestPinAvailabilityZonesRejectsConcurrentConfigChange(t *testing.T) {
	p := zoneParams(t, "")
	original, _ := os.ReadFile(p.configFile)
	changed := append(append([]byte(nil), original...), []byte("concurrent: preserve\n")...)
	err := pinAvailabilityZones(context.Background(), p, func(context.Context, string) ([]string, error) {
		if err := os.WriteFile(p.configFile, changed, 0600); err != nil {
			t.Fatal(err)
		}
		return []string{"us-east-1a", "us-east-1b"}, nil
	})
	after, _ := os.ReadFile(p.configFile)
	if err == nil || string(after) != string(changed) || len(p.Settings().Config.Aws.AvailabilityZoneNames) != 0 {
		t.Fatal("concurrent configuration must not be overwritten or exported as a saved snapshot")
	}
	if _, err := os.Stat(p.configFile + ".zone-identities"); !os.IsNotExist(err) {
		t.Fatal("owned failed-write staging file was not removed")
	}
}

func TestPinAvailabilityZonesPreservesOtherInstallerStaging(t *testing.T) {
	p := zoneParams(t, "")
	staging := p.configFile + ".zone-identities"
	if err := os.WriteFile(staging, []byte("other installer"), 0600); err != nil {
		t.Fatal(err)
	}
	err := pinAvailabilityZones(context.Background(), p, func(context.Context, string) ([]string, error) {
		return []string{"us-east-1a", "us-east-1b"}, nil
	})
	content, _ := os.ReadFile(staging)
	if err == nil || string(content) != "other installer" {
		t.Fatal("installer must not overwrite or remove another installer's staging file")
	}
}
