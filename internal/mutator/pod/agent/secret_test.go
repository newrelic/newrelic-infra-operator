// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

const fakeLicense = "fake-license"

//nolint:funlen,cyclop
func Test_Secret(t *testing.T) {
	t.Parallel()

	t.Run("creates_secret", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().Build()

		i, err := agent.New(&agent.Config{
			Logger:             logrus.New(),
			Client:             c,
			AgentConfig:        nil,
			ResourcePrefix:     "fake-release",
			LicenseSecretName:  agent.GetLicenseSecretName("fake-release"),
			LicenseSecretKey:   "license",
			LicenseSecretValue: []byte(fakeLicense),
		})
		if err != nil {
			t.Fatalf("No error expected creating injector : %v", err)
		}

		err = i.EnsureLicenseSecretExistence(testutil.ContextWithDeadline(t), "namespace")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %v", err.Error())
		}

		secret := &v1.Secret{}
		key := client.ObjectKey{
			Name:      agent.GetLicenseSecretName("fake-release"),
			Namespace: "namespace",
		}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}

		if !bytes.Equal(secret.Data["license"], []byte(fakeLicense)) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}
	})
	t.Run("updates_secret_data", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().Build()

		i, err := agent.New(&agent.Config{
			Logger:             logrus.New(),
			Client:             c,
			AgentConfig:        nil,
			ResourcePrefix:     "fake-release",
			LicenseSecretName:  agent.GetLicenseSecretName("fake-release"),
			LicenseSecretKey:   "license",
			LicenseSecretValue: []byte(fakeLicense),
		})
		if err != nil {
			t.Fatalf("No error expected creating injector : %v", err)
		}

		// Assuring existence of a secret causing its creation
		err = i.EnsureLicenseSecretExistence(testutil.ContextWithDeadline(t), "namespace")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %v", err.Error())
		}

		secret := &v1.Secret{}
		key := client.ObjectKey{
			Name:      agent.GetLicenseSecretName("fake-release"),
			Namespace: "namespace",
		}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}

		if !bytes.Equal(secret.Data["license"], []byte(fakeLicense)) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}

		// Assuring existence of the secret with different data causing its update
		i.LicenseSecretValue = []byte("new-value")
		err = i.EnsureLicenseSecretExistence(testutil.ContextWithDeadline(t), "namespace")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %v", err.Error())
		}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}

		if !bytes.Equal(secret.Data["license"], []byte("new-value")) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}

		// Assuring existence of the secret with different data causing its update
		i.LicenseSecretKey = "different-key"
		err = i.EnsureLicenseSecretExistence(testutil.ContextWithDeadline(t), "namespace")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %v", err.Error())
		}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}

		if !bytes.Equal(secret.Data["different-key"], []byte("new-value")) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}
	})
}
