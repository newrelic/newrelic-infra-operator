// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	corev1 "k8s.io/api/core/v1"
)

// InfraAgentConfig holds the user's configuration for the sidecar to be injected.
type InfraAgentConfig struct {
	// Here we can map the whole user configuration from helm chart.
	ExtraEnvVars         map[string]string            `yaml:"extraEnvVars"`
	ResourceRequirements *corev1.ResourceRequirements `yaml:"resources"`
	Image                Image                        `yaml:"image"`
	PodSecurityContext   PodSecurityContext           `yaml:"podSecurityContext"`
}

// Image config.
type Image struct {
	Repository string            `yaml:"repository"`
	Tag        string            `yaml:"tag"`
	PullPolicy corev1.PullPolicy `yaml:"pullPolicy"`
}

// PodSecurityContext config.
type PodSecurityContext struct {
	RunAsUser  int64 `yaml:"runAsUser"`
	RunAsGroup int64 `yaml:"runAsGroup"`
}
