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

package schema

// StageConfig represents the configuration for a single stage in the Quartz pipeline.
// It includes details such as dependencies, variables, and checks.
type StageConfig struct {
	Id           string                       `koanf:"id"`
	Description  string                       `koanf:"description"`
	Path         string                       `koanf:"path"`
	Type         string                       `koanf:"type"`         // terraform, other
	Dependencies []string                     `koanf:"dependencies"` // slice of stages that have to run before
	Disabled     bool                         `koanf:"disabled"`
	Manual       bool                         `koanf:"manual"`
	Order        int                          `koanf:"order"`
	Providers    StageProvidersConfig         `koanf:"providers"`
	OverrideVars bool                         `koanf:"override_vars"`
	Vars         map[string]StageVarsConfig   `koanf:"vars"`
	Checks       map[string]StageChecksConfig `koanf:"checks"`
	Destroy      StageDestroyConfig           `koanf:"destroy"`
	Debug        StageDebugConfig             `koanf:"debug"`
}

// StageChecksConfig represents the configuration for checks associated with a stage.
type StageChecksConfig struct {
	Before     []string                       `koanf:"before"`
	After      []string                       `koanf:"after"`
	Http       []StageChecksHttpConfig        `koanf:"http"`
	Kubernetes []StageChecksKubernetesConfig  `koanf:"kubernetes"`
	DaemonSet  []StageChecksDaemonSetConfig   `koanf:"daemonset"`
	State      []StageChecksStateConfig       `koanf:"state"`
	Order      int                            `koanf:"order"`
}

// StageProvidersConfig represents the configuration for providers used in a stage.
type StageProvidersConfig struct {
	Kubernetes bool `koanf:"kubernetes"`
}

// StageVarsStageConfig represents the configuration for a variable stage.
type StageVarsStageConfig struct {
	Name   string `koanf:"name"`
	Output string `koanf:"output"`
}

// StageVarsConfig represents the configuration for input variables for a stage.
type StageVarsConfig struct {
	Value  string               `koanf:"value"`
	Stage  StageVarsStageConfig `koanf:"stage"`
	Env    string               `koanf:"env"`
	Config string               `koanf:"config"`
	Secret string               `koanf:"secret"`
}

// StageChecksHttpConfig represents the configuration for HTTP-based checks in a stage.
type StageChecksHttpConfig struct {
	Url         string                       `koanf:"url"`
	Path        string                       `koanf:"path"`
	App         string                       `koanf:"app"`
	StatusCodes []int                        `koanf:"status_codes"`
	Content     StageChecksHttpContentConfig `koanf:"content"`
	Verify      bool                         `koanf:"verify"`
	Retry       StageChecksRetryConfig       `koanf:"retry"`
}

// StageChecksHttpContentConfig represents the configuration for HTTP content checks.
type StageChecksHttpContentConfig struct {
	Json  StageChecksHttpJsonContentConfig `koanf:"json"`
	Value string                           `koanf:"value"`
}

// StageChecksHttpJsonContentConfig represents the configuration for JSON content checks.
type StageChecksHttpJsonContentConfig struct {
	// TODO: add jsonpath support
	Key string `koanf:"key"`
}

// StageChecksKubernetesConfig represents the configuration for Kubernetes-based checks in a stage.
type StageChecksKubernetesConfig struct {
	Name      string `koanf:"name"`
	Namespace string `koanf:"namespace"`
	Kind      string `koanf:"kind"`
	State     string `koanf:"state"`
	Timeout   int    `koanf:"timeout"`
	Restart   bool   `koanf:"restart"`
	Wait      *bool  `koanf:"wait"`
}

// StageChecksStateConfig represents the configuration for state-based checks in a stage.
type StageChecksStateConfig struct {
	Key   string                 `koanf:"key"`
	Value string                 `koanf:"value"`
	Retry StageChecksRetryConfig `koanf:"retry"`
}

// StageChecksDaemonSetConfig represents the configuration for DaemonSet readiness checks.
// This is used to verify that critical system DaemonSets (like istio-cni) are fully
// deployed on all nodes before proceeding with workload deployment.
type StageChecksDaemonSetConfig struct {
	Name      string                 `koanf:"name"`
	Namespace string                 `koanf:"namespace"`
	Retry     StageChecksRetryConfig `koanf:"retry"`
}

// StageChecksRetryConfig represents the retry configuration for stage checks.
type StageChecksRetryConfig struct {
	Limit       int `koanf:"limit"`
	WaitSeconds int `koanf:"wait_seconds"`
}

// StageDestroyConfig represents the configuration for destroying resources in a stage.
type StageDestroyConfig struct {
	Skip    bool     `koanf:"skip"`
	Include []string `koanf:"include"`
	Exclude []string `koanf:"exclude"`
}

// StageDebugConfig represents the debug configuration for a stage.
type StageDebugConfig struct {
	Break bool `koanf:"break"`
}
