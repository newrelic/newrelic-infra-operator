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
	defaultHealthProbeBindAddress = ":9440"
)

// Options represents configurable options when running operator.
type Options struct {
	CertDir                string
	HealthProbeBindAddress string
	RestConfig             *rest.Config
}

// Run starts operator main loop. At the moment it only runs TLS webhook server and healthcheck web server.
func Run(ctx context.Context, options Options) error {
	if options.RestConfig == nil {
		// Has no Kubernetes credentials available.
		config, err := config.GetConfig()
		if err != nil {
			return fmt.Errorf("getting client configuration: %w", err)
		}

		options.RestConfig = config
	}

	// Has bad configuration.
	mgr, err := manager.New(options.RestConfig, options.withDefaults().toManagerOptions())
	if err != nil {
		return fmt.Errorf("initializing manager: %w", err)
	}

	// Serves /readyz request
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		return fmt.Errorf("adding readiness check: %w", err)
	}

	// Serves /healthz request.
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

	// Responds to requests at /mutate-v1-pod.
	mgr.GetWebhookServer().Register("/mutate-v1-pod", admission)

	// Stops when context is cancelled.
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("running manager: %w", err)
	}

	return nil
}

func (o *Options) toManagerOptions() manager.Options {
	return manager.Options{
		CertDir:                o.CertDir,
		HealthProbeBindAddress: o.HealthProbeBindAddress,
	}
}

func (o *Options) withDefaults() *Options {
	if o == nil {
		o = &Options{}
	}

	if o.HealthProbeBindAddress == "" {
		o.HealthProbeBindAddress = defaultHealthProbeBindAddress
	}

	return o
}
