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
	ExtraEnvVars       map[string]string  `json:"extraEnvVars"`
	ConfigSelectors    []ConfigSelector   `json:"configSelectors"`
	Image              Image              `json:"image"`
	PodSecurityContext PodSecurityContext `json:"podSecurityContext"`
}

// Image config.
type Image struct {
	Repository string            `json:"repository"`
	Tag        string            `json:"tag"`
	PullPolicy corev1.PullPolicy `json:"pullPolicy"`
}

// PodSecurityContext config.
type PodSecurityContext struct {
	RunAsUser  int64 `json:"runAsUser"`
	RunAsGroup int64 `json:"runAsGroup"`
}

// ConfigSelector allows you to set resourceRequirements and extraEnvVars based on labels.
type ConfigSelector struct {
	ResourceRequirements *corev1.ResourceRequirements `json:"resourceRequirements"`
	// ExtraEnvVars are additional ones and will be appended to InfraAgentConfig.ExtraEnvVars.
	ExtraEnvVars  map[string]string    `json:"extraEnvVars"`
	LabelSelector metav1.LabelSelector `json:"labelSelector"`

	selector labels.Selector `json:"-"`
}
