// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package operator exports top-level operator logic for users like CLI package to consume.
package operator

import (
	"context"
	"fmt"
	"os"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
)

const (
	// PodMutateEndpoint is a URI where admission webhook responds for Pod mutation requests.
	PodMutateEndpoint = "/mutate-v1-pod"

	// DefaultHealthProbeBindAddress is a default bind address for health probes.
	DefaultHealthProbeBindAddress = ":9440"

	defaultReleaseName = "newrelic-infra-operators"
)

// Options holds the configuration for an operator.
type Options struct {
	CertDir                string
	HealthProbeBindAddress string
	Port                   int
	RestConfig             *rest.Config
	Logger                 *logrus.Logger
}

// Run starts operator main loop. At the moment it only runs TLS webhook server and healthcheck web server.
func Run(ctx context.Context, options Options) error {
	if options.RestConfig == nil {
		// Required for in-cluster client configuration.
		restConfig, err := config.GetConfig()
		if err != nil {
			return fmt.Errorf("getting client configuration: %w", err)
		}

		options.RestConfig = restConfig
	}

	mgr, err := manager.New(options.RestConfig, options.withDefaults().toManagerOptions())
	if err != nil {
		return fmt.Errorf("initializing manager: %w", err)
	}

	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("adding readiness check: %w", err)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("adding health check: %w", err)
	}

	client, err := client.New(mgr.GetConfig(), client.Options{Scheme: mgr.GetScheme(), Mapper: mgr.GetRESTMapper()})
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	agentAconfig, err := readConfigStub()
	if err != nil {
		return fmt.Errorf("partsing agentConfig: %w", err)
	}

	agentInjector, err := agent.New(&agent.Config{
		Logger:      options.Logger,
		Client:      client,
		AgentConfig: agentAconfig,
	})
	if err != nil {
		return fmt.Errorf("creating injector: %w", err)
	}

	admission := &webhook.Admission{
		Handler: &podMutatorHandler{
			mutators: []podMutator{
				agentInjector,
			},
		},
	}

	mgr.GetWebhookServer().Register(PodMutateEndpoint, admission)

	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("running manager: %w", err)
	}

	return nil
}

func (o *Options) toManagerOptions() manager.Options {
	return manager.Options{
		CertDir:                o.CertDir,
		HealthProbeBindAddress: o.HealthProbeBindAddress,
		Port:                   o.Port,
	}
}

func (o *Options) withDefaults() *Options {
	if o == nil {
		o = &Options{}
	}

	if o.HealthProbeBindAddress == "" {
		o.HealthProbeBindAddress = DefaultHealthProbeBindAddress
	}

	return o
}

func readConfigStub() (*agent.InfraAgentConfig, error) {
	// TODO This should provide as well default values when we will be reading such data
	releaseName := os.Getenv("RELEASE_NAME")
	if releaseName == "" {
		releaseName = defaultReleaseName
	}

	memoryLimit, err := resource.ParseQuantity("100M")
	if err != nil {
		return nil, fmt.Errorf("parsing memoryLimit: %w", err)
	}

	memoryRequest, err := resource.ParseQuantity("100M")
	if err != nil {
		return nil, fmt.Errorf("parsing memoryRequest: %w", err)
	}

	CPULimit, err := resource.ParseQuantity("100m")
	if err != nil {
		return nil, fmt.Errorf("parsing CPULimit: %w", err)
	}

	CPURequest, err := resource.ParseQuantity("100m")
	if err != nil {
		return nil, fmt.Errorf("parsing CPURequest: %w", err)
	}

	return &agent.InfraAgentConfig{
		ExtraEnvVars: map[string]string{
			"NRIA_VERBOSE": "1",
		},
		ReleaseName: releaseName,
		LicenseKey:  os.Getenv("NRIA_LICENSE_KEY"),
		ResourceRequirements: &v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    CPULimit,
				v1.ResourceMemory: memoryLimit,
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    CPURequest,
				v1.ResourceMemory: memoryRequest,
			},
		},
		Image: agent.Image{
			Repository: "newrelic/infrastructure-k8s",
			Tag:        "2.4.0-unprivileged",
		},
	}, nil
}
