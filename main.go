// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/operator"
)

func main() {
	logger := logrus.New()
	logger.Printf("Starting NewRelic infra operator")

	options, err := stubOptions()
	if err != nil {
		logger.Printf("Generating stub config: %v", err)

		os.Exit(1)
	}

	options.Logger = logger

	if err := operator.Run(signals.SetupSignalHandler(), *options); err != nil {
		logger.Printf("Running operator failed: %v", err)

		os.Exit(1)
	}
}

func stubOptions() (*operator.Options, error) {
	memoryLimit, err := resource.ParseQuantity("100M")
	if err != nil {
		return nil, fmt.Errorf("parsing memoryLimit: %w", err)
	}

	memoryRequest, err := resource.ParseQuantity("100M")
	if err != nil {
		return nil, fmt.Errorf("parsing memoryRequest: %w", err)
	}

	cpuLimit, err := resource.ParseQuantity("100m")
	if err != nil {
		return nil, fmt.Errorf("parsing CPULimit: %w", err)
	}

	cpuRequest, err := resource.ParseQuantity("100m")
	if err != nil {
		return nil, fmt.Errorf("parsing CPURequest: %w", err)
	}

	infraAgentConfig := agent.InfraAgentConfig{
		ExtraEnvVars: map[string]string{
			"NRIA_VERBOSE": "1",
		},
		ResourceRequirements: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    cpuLimit,
				corev1.ResourceMemory: memoryLimit,
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    cpuRequest,
				corev1.ResourceMemory: memoryRequest,
			},
		},
	}

	return &operator.Options{
		InfraAgentInjection: agent.InjectorConfig{
			AgentConfig: &infraAgentConfig,
			License:     os.Getenv("NRIA_LICENSE_KEY"),
		},
	}, nil
}
