// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package main

import (
	"log"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/newrelic/nri-k8s-operator/internal/operator"
)

func main() {
	if err := operator.Run(signals.SetupSignalHandler(), operator.Options{}); err != nil {
		log.Printf("Running operator failed: %v", err)

		os.Exit(1)
	}
}
