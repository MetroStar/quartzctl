package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

func zoneParams(t *testing.T, aws string) *CommandParams {
	t.Helper()
	dir := ".zone-fixture-" + t.Name()
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	document := []byte("providers:\n  cloud: aws\naws:\n  region: us-east-1\n" + aws + "unrelated: preserve\n")
	path := filepath.Join(dir, "quartz.yaml")
	if err := os.WriteFile(path, document, 0600); err != nil {
		t.Fatal(err)
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
		t.Fatal(err)
	}
	settings, err := config.NewSettings(k, koanf.New("."))
	if err != nil {
		t.Fatal(err)
	}
	return &CommandParams{configFile: path, settings: &settings}
}

func TestPinAvailabilityZonesPersistsOrderedFallback(t *testing.T) {
	p := zoneParams(t, "")
	names := []string{"us-east-1c", "us-east-1a", "us-east-1b"}
	calls := 0
	discover := func(context.Context, string) ([]string, error) {
		calls++
		return names, nil
	}
	if err := pinAvailabilityZones(context.Background(), p, discover); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Settings().Config.Aws.AvailabilityZoneNames, names) {
		t.Fatal("discovery order changed")
	}
	serialized := filepath.Join(filepath.Dir(p.configFile), "settings.json")
	if err := p.Settings().WriteJsonConfig(serialized, "settings", false); err != nil {
		t.Fatal(err)
	}
	exported, _ := os.ReadFile(serialized)
	var terraformSettings struct {
		Settings struct {
			Aws struct {
				Names []string `json:"availability_zone_names"`
			} `json:"aws"`
		} `json:"settings"`
	}
	if err := json.Unmarshal(exported, &terraformSettings); err != nil || !reflect.DeepEqual(terraformSettings.Settings.Aws.Names, names) {
		t.Fatal("Terraform JSON export lost the persisted identity snapshot", err)
	}
	k := koanf.New(".")
	if err := k.Load(file.Provider(p.configFile), yaml.Parser()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(k.Strings("aws.availability_zone_names"), names) || k.String("unrelated") != "preserve" {
		t.Fatal("persisted snapshot must preserve order and unrelated settings")
	}
	reloaded, err := config.NewSettings(k, koanf.New("."))
	if err != nil {
		t.Fatal(err)
	}
	p.settings = &reloaded
	if err := pinAvailabilityZones(context.Background(), p, discover); err != nil || calls != 1 {
		t.Fatal("reinstall must not rediscover or silently expand AWS zone identities", err)
	}
}

func TestPinAvailabilityZonesExplicitPrecedence(t *testing.T) {
	for _, setting := range []string{
		"  availability_zones: [us-east-1c, us-east-1a]\n",
		"  availability_zone_names: [us-east-1c, us-east-1a, us-east-1b]\n",
	} {
		p := zoneParams(t, setting)
		before, _ := os.ReadFile(p.configFile)
		err := pinAvailabilityZones(context.Background(), p, func(context.Context, string) ([]string, error) {
			t.Fatal("explicit identities must not be rediscovered")
			return nil, nil
		})
		after, _ := os.ReadFile(p.configFile)
		if err != nil || string(before) != string(after) {
			t.Fatal("explicit configuration changed", err)
		}
	}
}

func TestPinAvailabilityZonesFailsClosed(t *testing.T) {
	p := zoneParams(t, "")
	before, _ := os.ReadFile(p.configFile)
	for _, names := range [][]string{
		{"us-east-1a"}, {"us-east-1a", "us-east-1a"}, {"us-east-1a", "us-east-2a"},
		{"us-east-1a", "us-east-1-lax-1a"},
	} {
		if err := pinAvailabilityZones(context.Background(), p, func(context.Context, string) ([]string, error) { return names, nil }); err == nil {
			t.Fatal("invalid discovery must fail before persistence")
		}
	}
	if err := pinAvailabilityZones(context.Background(), p, func(context.Context, string) ([]string, error) {
		return nil, errors.New("discovery unavailable")
	}); err == nil {
		t.Fatal("missing AWS dependency or failed discovery must not invent zone identities")
	}
	after, _ := os.ReadFile(p.configFile)
	if string(before) != string(after) {
		t.Fatal("failed discovery mutated configuration")
	}
}
