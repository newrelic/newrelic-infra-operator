// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	testNamespace = "default"
)

//nolint:funlen,gocognit,cyclop
func Test_Mutate(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	t.Run("when_succeeds", func(t *testing.T) {
		t.Run("adds_infrastructure_agent_sidecar_container_to_given_pod", func(t *testing.T) { t.Parallel() })

		t.Run("adds_pods_ServiceAccount_to_infrastructure_agent_ClusterRoleBinding", func(t *testing.T) { t.Parallel() })

		t.Run("adds_sidecar_configuration_hash_as_pod_label", func(t *testing.T) { t.Parallel() })

		t.Run("creates_license_secret_for_pod_when_it_does_not_exist", func(t *testing.T) { t.Parallel() })

		t.Run("labels_license_secret_with_owner_label", func(t *testing.T) { t.Parallel() })
	})

	t.Run("is_idempotent", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		mutatedPod := p.DeepCopy()

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		if diff := cmp.Diff(mutatedPod, p); diff != "" {
			t.Fatalf("unexpected Pod diff\n: %v", diff)
		}
	})

	t.Run("retains_other_subjects_in_ClusterRoleBinding", func(t *testing.T) { t.Parallel() })

	t.Run("does_not_add_the_same_subject_to_ClusterRoleBinding_twice", func(t *testing.T) { t.Parallel() })

	t.Run("changes_configuration_hash_label_when_configuration_changes", func(t *testing.T) { t.Parallel() })

	t.Run("updates_license_secret_when_license_key_changes", func(t *testing.T) { t.Parallel() })

	t.Run("fails_when_infrastructure_agent_ClusterRoleBinding_do_not_exist", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err == nil {
			t.Fatalf("expected mutation to fail")
		}
	})

	t.Run("succeed", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(ctx, p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
		}

		if len(p.Spec.Containers) != 1 {
			t.Fatalf("missing injected container: %v", err)
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
		config.AgentConfig.ExtraEnvVars = map[string]string{"new-key": "new-val"}

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(ctx, p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
		}

		if len(p.Spec.Containers) != 1 {
			t.Fatalf("missing injected container: %v", err)
		}

		found := false
		for _, env := range p.Spec.Containers[0].Env {
			if env.Name == "new-key" && env.Value == "new-val" {
				found = true
			}
		}
		if !found {
			t.Fatalf("extra var not injected: %v", p.Spec.Containers[0].Env)
		}
	})
}

//nolint:funlen,gocognit,cyclop
func Test_Mutation_hash(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	t.Run("does_not_change_when_pod_definition_changes", func(t *testing.T) { t.Parallel() })

	t.Run("changes_when", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		baseHash, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]
		if !ok {
			t.Fatalf("missing injected label")
		}

		cases := map[string]func(*agent.InjectorConfig){
			"resource_prefix": func(c *agent.InjectorConfig) {
				c.ResourcePrefix = "bar"
			},
			"extra_environment_variables": func(c *agent.InjectorConfig) {
				c.AgentConfig.ExtraEnvVars = map[string]string{"foo": "bar"}
			},
			"resources": func(c *agent.InjectorConfig) {
				cpuLimit, err := resource.ParseQuantity("100m")
				if err != nil {
					t.Fatalf("parsing quantity: %v", err)
				}

				c.AgentConfig.ResourceRequirements = &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU: cpuLimit,
					},
				}
			},
			"image_repository": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.Repository = "foo"
			},
			"image_tag":          func(c *agent.InjectorConfig) {},
			"image_pull_policy":  func(c *agent.InjectorConfig) {},
			"image_pull_secrets": func(c *agent.InjectorConfig) {},
			"runnable_user": func(c *agent.InjectorConfig) {
				c.AgentConfig.PodSecurityContext.RunAsUser = 1000
			},
			"runnable_group": func(c *agent.InjectorConfig) {
				c.AgentConfig.PodSecurityContext.RunAsGroup = 1000
			},
		}

		for testCaseName, mutateConfigF := range cases {
			t.Run(fmt.Sprintf("%s_changes", testCaseName), func(t *testing.T) {
				t.Parallel()

				p := getEmptyPod()
				c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
				config := getConfig()

				mutateConfigF(config)

				i, err := config.New(c, c, nil)
				if err != nil {
					t.Fatalf("creating injector: %v", err)
				}

				if err := i.Mutate(ctx, p, req); err != nil {
					t.Fatalf("mutating Pod: %v", err)
				}

				newHash, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]
				if !ok {
					t.Fatalf("missing injected label")
				}

				t.Logf("old hash %q new hash %q", baseHash, newHash)
				if baseHash == newHash {
					t.Fatalf("no hash change detected")
				}
			})
		}
	})

	t.Run("injected_and_different", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB()).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		err = i.Mutate(ctx, p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
		}

		if _, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		p2 := getEmptyPod()
		config2 := getConfig()
		config2.AgentConfig.Image.Repository = "different-repo"

		i2, err := config2.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating second injector: %v", err)
		}

		err = i2.Mutate(ctx, p2, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
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

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}
		err = i.Mutate(ctx, p, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
		}
		if _, ok := p.ObjectMeta.Labels[agent.AgentInjectedLabel]; !ok {
			t.Fatalf("missing injected label")
		}

		p2 := getEmptyPod()
		p2.Labels = map[string]string{"newLabel": "newValue"}
		p2.Spec.Containers = append(p2.Spec.Containers, corev1.Container{Name: "test-container"})

		err = i.Mutate(ctx, p2, req)
		if err != nil || apierrors.IsNotFound(err) {
			t.Fatalf("crb created, should not fail: %v", err)
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

func clusterRoleBindingName() string {
	return fmt.Sprintf("%s%s", agent.DefaultResourcePrefix, agent.ClusterRoleBindingSuffix)
}

func secretName() string {
	return fmt.Sprintf("%s%s", agent.DefaultResourcePrefix, agent.LicenseSecretSuffix)
}

func getCRB() *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName(),
		},
		Subjects: []v1.Subject{},
		RoleRef:  v1.RoleRef{},
	}
}

func getSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(),
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"license": []byte("license"),
		},
	}
}

func getConfig() *agent.InjectorConfig {
	return &agent.InjectorConfig{
		AgentConfig: &agent.InfraAgentConfig{},
		License:     "license",
	}
}

func getEmptyPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNamespace,
		},
		Spec:   corev1.PodSpec{},
		Status: corev1.PodStatus{},
	}
}
