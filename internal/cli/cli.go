// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package cli provides CLI implementation for operator.
package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"sigs.k8s.io/yaml"

	"github.com/newrelic/newrelic-infra-operator/internal/operator"
)

const (
	// DefaultConfigFilePath is a path from where operator binary reads the configuration.
	DefaultConfigFilePath = "/etc/newrelic/newrelic-infra-operator/operator.yaml"

	// EnvLicenseKey is an environment variable, from which license key will be read.
	EnvLicenseKey = "NRIA_LICENSE_KEY"

	// EnvClusterName is an environment variable from which cluster name will be read if not set in configuration file.
	EnvClusterName = "CLUSTER_NAME"
)

// Options tries to read configuration from a given path and later fills missing configuration using
// environment variables.
//
// If configuration file is not found, only environment variables will be read.
func Options(path string) (*operator.Options, error) {
	options := &operator.Options{}

	optionsBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("reading configuration file %q: %w", path, err)
	}

	if err := yaml.UnmarshalStrict(optionsBytes, options); err != nil {
		return nil, fmt.Errorf("parsing configuration file content: %w", err)
	}

	options.InfraAgentInjection.License = os.Getenv(EnvLicenseKey)

	if options.InfraAgentInjection.ClusterName == "" {
		options.InfraAgentInjection.ClusterName = os.Getenv(EnvClusterName)
	}

	return options, nil
}
