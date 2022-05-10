// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/newrelic/newrelic-infra-operator/internal/cli"
	"github.com/newrelic/newrelic-infra-operator/internal/operator"
)

func main() {
	logger := logrus.New()
	logger.Printf("Starting NewRelic infra operator")

	options, err := cli.Options(cli.DefaultConfigFilePath)
	if err != nil {
		logger.Printf("Generation operator configuration: %v", err)

		os.Exit(1)
	}

	options.Logger = logger

	if err := operator.Run(signals.SetupSignalHandler(), *options); err != nil {
		logger.Printf("Running operator failed: %v", err)

		os.Exit(1)
	}
}
