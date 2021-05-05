// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// agent_test collects tests for the agent package.
package agent_test

import (
	"testing"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

//nolint:funlen,gocognit,cyclop
func Test_injector(t *testing.T) {
	t.Parallel()

	req := webhook.RequestOptions{
		Namespace: "default",
	}

	t.Run("fail_missing_crb", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().Build()
		config := getConfig()
		config.Client = c

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err == nil || !apierrors.IsNotFound(err) {
			t.Fatalf("crb not created, should fail : %v", err)
		}
	})

	t.Run("succeed", func(t *testing.T) {
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
			t.Fatalf("crb created, should not fail : %v", err)
		}

		if len(p.Spec.Containers) != 1 {
			t.Fatalf("missing injected container : %v", err)
		}

		if _, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}
	})

	t.Run("succeed_with_extra_env", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()
		config.Client = c
		config.AgentConfig.ExtraEnvVars = map[string]string{"new-key": "new-val"}

		i, err := agent.New(config)
		if err != nil {
			t.Fatalf("creating injector : %v", err)
		}

		err = i.Mutate(testutil.ContextWithDeadline(t), p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail : %v", err)
		}

		if len(p.Spec.Containers) != 1 {
			t.Fatalf("missing injected container : %v", err)
		}

		found := false
		for _, env := range p.Spec.Containers[0].Env {
			if env.Name == "new-key" && env.Value == "new-val" {
				found = true
			}
		}
		if !found {
			t.Fatalf("extra var not injected : %v", p.Spec.Containers[0].Env)
		}
	})
}

//nolint:funlen,gocognit,cyclop
func Test_hash(t *testing.T) {
	t.Parallel()

	req := webhook.RequestOptions{
		Namespace: "default",
	}

	t.Run("injected_and_different", func(t *testing.T) {
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
			t.Fatalf("crb created, should not fail : %v", err)
		}

		if _, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		p2 := getEmptyPod()
		c2 := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config2 := getConfig()
		config2.Client = c2
		config2.AgentConfig.Image.Repository = "different-repo"

		i2, err := agent.New(config2)
		if err != nil {
			t.Fatalf("creating second injector : %v", err)
		}

		err = i2.Mutate(testutil.ContextWithDeadline(t), p2, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail : %v", err)
		}

		if _, ok := p2.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		if p.ObjectMeta.Labels[agent.AgentInjectedLabel] == p2.ObjectMeta.Labels[agent.AgentInjectedLabel] {
			t.Fatalf("labels should not match: %v==%v",
				p.ObjectMeta.Labels[agent.AgentInjectedLabel],
				p2.ObjectMeta.Labels[agent.AgentInjectedLabel])
		}
	})

	t.Run("do_not_depend_on_pod", func(t *testing.T) {
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
			t.Fatalf("crb created, should not fail : %v", err)
		}
		if _, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		p2 := getEmptyPod()
		p2.Labels = map[string]string{"newLabel": "newValue"}
		p2.Spec.Containers = append(p2.Spec.Containers, corev1.Container{Name: "test-container"})

		err = i.Mutate(testutil.ContextWithDeadline(t), p2, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail : %v", err)
		}

		if _, ok := p2.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		if p.ObjectMeta.Labels[agent.AgentInjectedLabel] != p2.ObjectMeta.Labels[agent.AgentInjectedLabel] {
			t.Fatalf("labels should not match: %v!=%v",
				p.ObjectMeta.Labels[agent.AgentInjectedLabel],
				p2.ObjectMeta.Labels[agent.AgentInjectedLabel])
		}
	})
}

func getCRB() *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "crb-name",
		},
		Subjects: []v1.Subject{},
		RoleRef:  v1.RoleRef{},
	}
}

func getSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "newrelic-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"license": []byte("license"),
		},
	}
}

func getConfig() *agent.Config {
	return &agent.Config{
		Logger:                 logrus.New(),
		AgentConfig:            &agent.InfraAgentConfig{},
		ResourcePrefix:         "newrelic",
		LicenseSecretName:      "newrelic-secret",
		LicenseSecretKey:       "license",
		LicenseSecretValue:     []byte("license"),
		ClusterRoleBindingName: "crb-name",
	}
}

func getEmptyPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
}
