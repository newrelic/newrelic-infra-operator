// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"log"
	"os"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/newrelic/newrelic-infra-operator/internal/operator"
)

func main() {
	log.Printf("Test Reload")

	if err := operator.Run(signals.SetupSignalHandler(), operator.Options{Logger: logrus.New()}); err != nil {
		log.Printf("Running operator failed: %v", err)

		os.Exit(1)
	}
}
