// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package operator exports top-level operator logic for users like CLI package to consume.
package operator

import (
	"context"
	"fmt"

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
)

// Options holds the configuration for an operator.
type Options struct {
	CertDir                string
	HealthProbeBindAddress string
	MetricsBindAddress     string
	Port                   int
	RestConfig             *rest.Config
	Logger                 *logrus.Logger
	IgnoreMutationErrors   bool

	InfraAgentInjection agent.InjectorConfig
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

	clientOptions := client.Options{
		Scheme: mgr.GetScheme(),
		Mapper: mgr.GetRESTMapper(),
	}

	noCacheClient, err := client.New(mgr.GetConfig(), clientOptions)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	agentInjector, err := options.InfraAgentInjection.New(mgr.GetClient(), noCacheClient)
	if err != nil {
		return fmt.Errorf("creating injector: %w", err)
	}

	admission := &webhook.Admission{
		Handler: &podMutatorHandler{
			ignoreMutationErrors: options.IgnoreMutationErrors,
			logger:               options.Logger,
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
		MetricsBindAddress:     o.MetricsBindAddress,
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
