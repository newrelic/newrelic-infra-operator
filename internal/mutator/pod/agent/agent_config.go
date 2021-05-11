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
	ExtraEnvVars           map[string]string  `json:"extraEnvVars"`
	ResourcesWithSelectors []Resource         `json:"resourcesWithSelectors"`
	Image                  Image              `json:"image"`
	PodSecurityContext     PodSecurityContext `json:"podSecurityContext"`
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

// Resource config.
type Resource struct {
	ResourceRequirements corev1.ResourceRequirements `json:"resourceRequirements"`
	LabelSelector        metav1.LabelSelector        `json:"labelSelector"`

	selector labels.Selector `json:"-"`
}
