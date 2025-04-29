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
