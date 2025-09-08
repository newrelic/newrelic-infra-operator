// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"

	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/newrelic/newrelic-infra-operator/internal/cli"
	"github.com/newrelic/newrelic-infra-operator/internal/operator"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctrl.SetLogger(zap.New())

	entryLog := ctrl.Log.WithName("entrypoint")

	entryLog.Info("Starting NewRelic infra operator")

	options, err := cli.Options(cli.DefaultConfigFilePath)
	if err != nil {
		entryLog.Error(err, "Generation operator configuration")
		os.Exit(1)
	}

	options.Logger = entryLog.WithName("PodMutatorLogger")

	if err := operator.Run(signals.SetupSignalHandler(), *options); err != nil {
		entryLog.Error(err, "unable to run operator")
		os.Exit(1)
	}
}
