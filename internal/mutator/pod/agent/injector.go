// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package agent implements injection of infrastructure-agent container into given Pod.
package agent

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	agentInjectedLabel      = "agent-injected"
	agentInjectedLabelValue = "true"
)

// Injector holds agent injection configuration.
type Injector struct{}

// Mutate mutates given Pod object by injecting infrastructure-agent container into it with all dependencies.
func (i *Injector) Mutate(_ context.Context, _ client.Client, pod *corev1.Pod, _ string) error {
	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}

	pod.Labels[agentInjectedLabel] = agentInjectedLabelValue

	return nil
}
