// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package webhook provides the configuration for the different webhooks
package webhook

// RequestOptions contains all the configs coming from the request needed by the mutator.
type RequestOptions struct {
	Namespace string
	DryRun    bool
}
