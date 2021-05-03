package agent

import v1 "k8s.io/api/core/v1"

// InfraAgentConfig holds the user's configuration for the sidecar to be injected.
type InfraAgentConfig struct {
	// Here we can map the whole user configuration from helm chart
	ExtraEnvVars         map[string]string        `yaml:"extraEnvVars"`
	ResourceRequirements *v1.ResourceRequirements `yaml:"resources"`
	Image                Image                    `yaml:"image"`
	PodSecurityContext   PodSecurityContext       `yaml:"podSecurityContext"`
	LicenseKey           string
	ReleaseName          string
}

// Image config.
type Image struct {
	Repository string `yaml:"repository"`
	Tag        string `yaml:"tag"`
	PullPolicy string `yaml:"pullPolicy"`
}

// PullSecrets config.
type PullSecrets struct {
	Name string `yaml:"name"`
}

// PodSecurityContext config.
type PodSecurityContext struct {
	RunAsUser  int64 `yaml:"runAsUser"`
	RunAsGroup int64 `yaml:"runAsGroup"`
}
