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

// KubernetesConfig represents the configuration for Kubernetes.
type KubernetesConfig struct {
	Version        string `koanf:"version"`
	KubeconfigPath string `koanf:"kubeconfig_path"`
}

// Kubeconfig represents the structure of a Kubernetes kubeconfig file.
type Kubeconfig struct {
	ApiVersion     string              `koanf:"apiVersion"`
	Kind           string              `koanf:"kind"`
	CurrentContext string              `koanf:"current-context"`
	Preferences    interface{}         `koanf:"preferences"`
	Clusters       []KubeconfigCluster `koanf:"clusters"`
	Contexts       []KubeconfigContext `koanf:"contexts"`
	Users          []KubeconfigUser    `koanf:"users"`
}

// KubeconfigCluster represents a cluster entry in a kubeconfig file.
type KubeconfigCluster struct {
	Name    string                `koanf:"name"`
	Cluster KubeconfigClusterInfo `koanf:"cluster"`
}

type KubeconfigClusterInfo struct {
	Server                   string `koanf:"server"`
	CertificateAuthorityData string `koanf:"certificate-authority-data"`
}

// KubeconfigContext represents a context entry in a kubeconfig file.
type KubeconfigContext struct {
	Name    string                `koanf:"name"`
	Context KubeconfigContextInfo `koanf:"context"`
}

type KubeconfigContextInfo struct {
	Cluster string `koanf:"cluster"`
	User    string `koanf:"user"`
}

// KubeconfigUser represents a user entry in a kubeconfig file.
type KubeconfigUser struct {
	Name string             `koanf:"name"`
	User KubeconfigUserInfo `koanf:"user"`
}

type KubeconfigUserInfo struct {
	Token *string             `koanf:"token"`
	Exec  *KubeconfigUserExec `koanf:"exec"`
}

type KubeconfigUserExec struct {
	ApiVersion string   `koanf:"apiVersion"`
	Command    string   `koanf:"command"`
	Args       []string `koanf:"args"`
}
