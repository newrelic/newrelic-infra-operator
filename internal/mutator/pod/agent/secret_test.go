// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"bytes"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

//nolint:funlen,gocognit,cyclop
func Test_secrets(t *testing.T) {
	t.Parallel()

	req := webhook.RequestOptions{
		Namespace: "default",
	}

	t.Run("created_no_namespace_dpecified", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()
		config.Client = c

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail : %v", err)
		}

		key := client.ObjectKey{
			Namespace: "default",
			Name:      "newrelic-secret",
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found : %v", err)
		}

		if !bytes.Equal(secret.Data["license"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["license"], []byte("license"))
		}
		if secret.ObjectMeta.Labels[agent.OperatorCreatedLabel] != "true" {
			t.Fatalf("label not injected %v", secret.ObjectMeta.Labels[agent.OperatorCreatedLabel])
		}
	})

	t.Run("created_in_non_default_namespace", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()
		config.Client = c

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		reqDifferentNamespace := webhook.RequestOptions{
			Namespace: "not-default",
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, reqDifferentNamespace)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail : %v", err)
		}

		key := client.ObjectKey{
			Namespace: "not-default",
			Name:      "newrelic-secret",
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found : %v", err)
		}

		if !bytes.Equal(secret.Data["license"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["license"], []byte("license"))
		}
	})

	t.Run("untouched", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(), getSecret()).Build()
		config := getConfig()
		config.Client = c

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail : %v", err)
		}

		key := client.ObjectKey{
			Namespace: "default",
			Name:      "newrelic-secret",
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found: %v", err)
		}

		if !bytes.Equal(secret.Data["license"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["license"], []byte("license"))
		}
		if _, ok := secret.ObjectMeta.Labels[agent.OperatorCreatedLabel]; ok {
			t.Fatalf("label is not expected: %v", secret.ObjectMeta.Labels[agent.OperatorCreatedLabel])
		}
	})

	t.Run("updated_data", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		s := getSecret()
		s.Data["license"] = []byte("old_data")
		c := fake.NewClientBuilder().WithObjects(getCRB(), s).Build()
		config := getConfig()
		config.Client = c

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail : %v", err)
		}

		key := client.ObjectKey{
			Namespace: "default",
			Name:      "newrelic-secret",
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found: %v", err)
		}

		if !bytes.Equal(secret.Data["license"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["license"], []byte("license"))
		}
		if _, ok := secret.ObjectMeta.Labels[agent.OperatorCreatedLabel]; ok {
			t.Fatalf("label is not expected: %v", secret.ObjectMeta.Labels[agent.OperatorCreatedLabel])
		}
	})

	t.Run("updated_license_key", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(), getSecret()).Build()
		config := getConfig()
		config.Client = c
		config.LicenseSecretKey = "different-license-key"

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail : %v", err)
		}

		key := client.ObjectKey{
			Namespace: "default",
			Name:      "newrelic-secret",
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found: %v", err)
		}

		if !bytes.Equal(secret.Data["different-license-key"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["different-license-key"], []byte("license"))
		}
		if _, ok := secret.ObjectMeta.Labels[agent.OperatorCreatedLabel]; ok {
			t.Fatalf("label is not expected: %v", secret.ObjectMeta.Labels[agent.OperatorCreatedLabel])
		}
	})
}
