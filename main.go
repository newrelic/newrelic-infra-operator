// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
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

	options := stubOptions()

	options.Logger = logger

	if err := operator.Run(signals.SetupSignalHandler(), *options); err != nil {
		logger.Printf("Running operator failed: %v", err)

		os.Exit(1)
	}
}

//nolint:gomnd
func stubOptions() *operator.Options {
	infraAgentConfig := agent.InfraAgentConfig{
		ExtraEnvVars: map[string]string{
			"NRIA_VERBOSE": "1",
		},
		ResourceRequirements: &corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(200, resource.Milli),
				corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewScaledQuantity(100, resource.Milli),
				corev1.ResourceMemory: *resource.NewScaledQuantity(50, resource.Mega),
			},
		},
	}

	return &operator.Options{
		InfraAgentInjection: agent.InjectorConfig{
			AgentConfig: &infraAgentConfig,
			License:     os.Getenv("NRIA_LICENSE_KEY"),
			ClusterName: os.Getenv("CLUSTER_NAME"),
			CustomAttributes: []agent.CustomAttribute{
				{
					Name:         "computeType",
					DefaultValue: "serverless",
				},
				{
					Name: "fargateProfile",
					// To allow inclusion on non-fargate clusters for testing.
					DefaultValue: "unknown",
					FromLabel:    "eks.amazonaws.com/fargate-profile",
				},
			},
		},
	}
}
