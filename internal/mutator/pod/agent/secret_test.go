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
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

//nolint:funlen,gocognit,cyclop
func Test_secrets(t *testing.T) {
	t.Parallel()

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	t.Run("created_no_namespace_specified", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Namespace: testNamespace,
			Name:      secretName(),
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found: %v", err)
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		reqDifferentNamespace := webhook.RequestOptions{
			Namespace: "not-default",
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, reqDifferentNamespace)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Namespace: "not-default",
			Name:      secretName(),
		}

		secret := &corev1.Secret{}
		err = c.Get(testutil.ContextWithDeadline(t), key, secret)
		if err != nil {
			t.Fatalf("secret not found: %v", err)
		}

		if !bytes.Equal(secret.Data["license"], []byte("license")) {
			t.Fatalf("payloads are different: %s!=%s", secret.Data["license"], []byte("license"))
		}
	})

	t.Run("untouched", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix), getSecret()).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Namespace: testNamespace,
			Name:      secretName(),
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix), s).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Namespace: testNamespace,
			Name:      secretName(),
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
}
