// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"testing"

	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

//nolint:funlen,gocognit,cyclop
func Test_CRB(t *testing.T) {
	t.Parallel()

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	t.Run("updated_with_service_account_default", func(t *testing.T) {
		t.Parallel()

		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
		}

		crb := &v1.ClusterRoleBinding{}
		err = c.Get(testutil.ContextWithDeadline(t), key, crb)
		if err != nil {
			t.Fatalf("crb not found: %v", err)
		}

		if crb.Subjects[0].Name != "default" || crb.Subjects[0].Namespace != testNamespace {
			t.Fatalf("crb not including expecting sa: %v, %v", crb.Subjects[0].Name, crb.Subjects[0].Namespace)
		}
	})

	t.Run("does_not_add_same_subject_twice", func(t *testing.T) {
		t.Parallel()

		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
		}

		crb := &v1.ClusterRoleBinding{}
		err = c.Get(testutil.ContextWithDeadline(t), key, crb)
		if err != nil {
			t.Fatalf("crb not found: %v", err)
		}

		if len(crb.Subjects) != 1 {
			t.Fatalf("subjects unexpected: %d, %v", len(crb.Subjects), crb.Subjects)
		}
	})

	t.Run("adds_multiple_subjects", func(t *testing.T) {
		t.Parallel()

		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		req2 := webhook.RequestOptions{
			Namespace: "not=default",
		}
		err = i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req2)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("should not fail: %v", err)
		}

		key := client.ObjectKey{
			Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
		}

		crb := &v1.ClusterRoleBinding{}
		err = c.Get(testutil.ContextWithDeadline(t), key, crb)
		if err != nil {
			t.Fatalf("crb not found: %v", err)
		}

		if len(crb.Subjects) != 2 {
			t.Fatalf("subjects unexpected: %d, %v", len(crb.Subjects), crb.Subjects)
		}
	})
}
