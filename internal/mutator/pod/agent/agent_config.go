package agent

// InfraAgentConfig holds the user's configuration for the sidecar to be injected.
type InfraAgentConfig struct {
	// Here we can map the whole user configuration from helm chart
	ExtraEnvVars         map[string]string  `yaml:"extraEnvVars"`
	ResourceRequirements *Resources         `yaml:"resources"`
	Image                Image              `yaml:"image"`
	PodSecurityContext   PodSecurityContext `yaml:"podSecurityContext"`
	LicenseKey           string
	ReleaseName          string
}

// Quantities config used for both Limits and Requests.
type Quantities struct {
	CPU    string `yaml:"cpu"`
	Memory string `yaml:"memory"`
}

// Resources config.
type Resources struct {
	Requests Quantities `yaml:"requests"`
	Limits   Quantities `yaml:"limits"`
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
