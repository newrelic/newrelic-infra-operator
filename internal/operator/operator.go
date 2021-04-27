// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package operator exports top-level operator logic for users like CLI package to consume.
package operator

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"
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

// Options represents configurable options when running operator.
type Options struct {
	CertDir                string
	HealthProbeBindAddress string
	Port                   int
	RestConfig             *rest.Config
}

// Run starts operator main loop. At the moment it only runs TLS webhook server and healthcheck web server.
func Run(ctx context.Context, options Options) error {
	if options.RestConfig == nil {
		// Required for in-cluster client configuration.
		config, err := config.GetConfig()
		if err != nil {
			return fmt.Errorf("getting client configuration: %w", err)
		}

		options.RestConfig = config
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

	admission := &webhook.Admission{
		Handler: &podMutatorHandler{
			Client: mgr.GetClient(),
			mutators: []podMutator{
				&agent.Injector{},
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
