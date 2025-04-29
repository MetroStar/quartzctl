package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf/v2"
)

func TestConfigResultConfigString(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")
	sut := Settings{rawConfig: k}

	actual := sut.ConfigString("test.property.one")
	if actual != "value" {
		t.Errorf("incorrect response to raw config lookup, expected %s, found %s", "value", actual)
	}
}

func TestConfigResultSecretString(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")
	sut := Settings{rawSecrets: k}

	actual := sut.SecretString("test.property.one")
	if actual != "value" {
		t.Errorf("incorrect response to raw secret lookup, expected %s, found %s", "value", actual)
	}
}

func TestConfigResultWriteJsonConfig(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")
	sut := Settings{rawConfig: k}

	path := filepath.Join(t.TempDir(), "test.json")
	err := sut.WriteJsonConfig(path, "", false)
	if err != nil {
		t.Errorf("error writing config to %s, %v", path, err)
		return
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("error reading config from %s, %v", path, err)
		return
	}

	expected := `{"test":{"property":{"one":"value"}}}`

	if expected != strings.TrimSpace(string(actual)) {
		t.Errorf("incorrect json response, expected %s, found %s", expected, actual)
	}
}

func TestConfigResultWriteYamlConfig(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")
	sut := Settings{rawConfig: k}

	path := filepath.Join(t.TempDir(), "test.yaml")
	err := sut.WriteYamlConfig(path)
	if err != nil {
		t.Errorf("error writing config to %s, %v", path, err)
		return
	}

	actual, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("error reading config from %s, %v", path, err)
		return
	}

	expected := strings.TrimSpace(`
test:
    property:
        one: value
`)

	if expected != strings.TrimSpace(string(actual)) {
		t.Errorf("incorrect yaml response, expected %s, found %s", expected, actual)
	}
}

func TestConfigResultMarshalJsonRoot(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")

	actual, err := marshalJsonRoot(k, "", false)
	if err != nil {
		t.Errorf("error marshalling to json %v", err)
		return
	}

	expected := []byte(`{"test":{"property":{"one":"value"}}}`)

	if string(expected) != string(actual) {
		t.Errorf("incorrect json response, expected %s, found %s", expected, actual)
	}
}

func TestConfigResultMarshalJsonRootCustom(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")

	actual, err := marshalJsonRoot(k, "newroot", false)
	if err != nil {
		t.Errorf("error marshalling to json %v", err)
		return
	}

	expected := []byte(`{"newroot":{"test":{"property":{"one":"value"}}}}`)

	if string(expected) != string(actual) {
		t.Errorf("incorrect json response, expected %s, found %s", expected, actual)
	}
}

func TestConfigResultMarshalJson(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")

	actual, err := marshalJson(k, false)
	if err != nil {
		t.Errorf("error marshalling to json %v", err)
		return
	}

	expected := []byte(`{"test":{"property":{"one":"value"}}}`)

	if string(expected) != string(actual) {
		t.Errorf("incorrect json response, expected %s, found %s", expected, actual)
	}
}

func TestConfigResultMarshalJsonIndented(t *testing.T) {
	k := koanf.New(".")
	k.Set("test.property.one", "value")

	actual, err := marshalJson(k, true)
	if err != nil {
		t.Errorf("error marshalling to json %v", err)
		return
	}

	expected := []byte(`{
  "test": {
    "property": {
      "one": "value"
    }
  }
}`)

	if string(expected) != string(actual) {
		t.Errorf("incorrect json response, expected %s, found %s", expected, actual)
	}
}
