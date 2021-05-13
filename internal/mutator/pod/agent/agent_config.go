// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// InfraAgentConfig holds the user's configuration for the sidecar to be injected.
type InfraAgentConfig struct {
	// Here we can map the whole user configuration from helm chart.
	ExtraEnvVars           map[string]string  `yaml:"extraEnvVars"`
	ResourcesWithSelectors []Resource         `yaml:"resourcesWithSelectors"`
	Image                  Image              `yaml:"image"`
	PodSecurityContext     PodSecurityContext `yaml:"podSecurityContext"`
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

// Resource config.
type Resource struct {
	ResourceRequirements corev1.ResourceRequirements
	LabelSelector        metav1.LabelSelector

	selector labels.Selector
}
