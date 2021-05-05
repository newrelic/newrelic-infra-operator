// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	testNamespace = "test-namespace"
	testLicense   = "test-license"
)

func Test_Creating_injector_fails_when_license_is_empty(t *testing.T) {
	t.Parallel()

	config := getConfig()
	config.License = ""

	c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

	i, err := config.New(c, c, nil)
	if err == nil {
		t.Errorf("expected error from creating injector")
	}

	if i != nil {
		t.Errorf("expected to not get injector instance when creation error occurs")
	}
}

//nolint:funlen,gocognit,cyclop,gocyclo
func Test_Mutate(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	secretKey := client.ObjectKey{
		Namespace: testNamespace,
		Name:      secretName(),
	}

	t.Run("when_succeeds", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		t.Run("adds_infrastructure_agent_sidecar_container_to_given_pod", func(t *testing.T) {
			t.Parallel()

			if len(p.Spec.Containers) != 2 {
				t.Fatalf("expected extra container to be added, got %v", cmp.Diff(getEmptyPod(), p))
			}
		})

		t.Run("adds_pods_ServiceAccount_to_infrastructure_agent_ClusterRoleBinding", func(t *testing.T) {
			t.Parallel()

			key := client.ObjectKey{
				Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
			}

			crb := &v1.ClusterRoleBinding{}

			if err := c.Get(ctx, key, crb); err != nil {
				t.Fatalf("getting ClusterRoleBinding: %v", err)
			}

			found := false

			for _, subject := range crb.Subjects {
				if subject.Name == "default" && subject.Namespace == testNamespace {
					found = true

					break
				}
			}

			if !found {
				t.Fatalf("didn't find expected subject in ClusterRoleBinding, got: %v", crb)
			}
		})

		t.Run("adds_sidecar_configuration_hash_as_pod_label", func(t *testing.T) {
			t.Parallel()

			v, ok := p.Labels[agent.InjectedLabel]
			if !ok {
				t.Fatalf("label %q not found in mutated pod", agent.InjectedLabel)
			}

			if v == "" {
				t.Fatalf("label %q value must not be empty", agent.InjectedLabel)
			}
		})

		t.Run("creates_license_Secret_for_Pod_when_it_does_not_exist", func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{}
			if err := c.Get(ctx, secretKey, secret); err != nil {
				t.Fatalf("getting secret: %v", err)
			}

			license, ok := secret.Data[agent.LicenseSecretKey]
			if !ok {
				t.Fatalf("license key %q not found in created secret", license)
			}

			if diff := cmp.Diff(string(license), testLicense); diff != "" {
				t.Fatalf("unexpected license value: %v", diff)
			}
		})

		t.Run("labels_license_Secret_with_owner_label", func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{}
			if err := c.Get(ctx, secretKey, secret); err != nil {
				t.Fatalf("getting secret: %v", err)
			}

			if _, ok := secret.Labels[agent.OperatorCreatedLabel]; !ok {
				t.Fatalf("expected label %q to be set, got: %v", agent.OperatorCreatedLabel, secret.Labels)
			}
		})
	})

	t.Run("is_idempotent", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
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

	t.Run("retains_other_subjects_in_ClusterRoleBinding", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		crb := getCRB(agent.DefaultResourcePrefix)
		crb.Subjects = []v1.Subject{
			{
				Name: "test",
			},
		}

		c := fake.NewClientBuilder().WithObjects(crb).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		key := client.ObjectKey{
			Name: crb.Name,
		}

		if err := c.Get(ctx, key, crb); err != nil {
			t.Fatalf("getting updated ClusterRoleBinding: %v", err)
		}

		expectedSubjects := 2

		if subjects := len(crb.Subjects); subjects != expectedSubjects {
			t.Fatalf("expected %d subjects, got %d: %v", expectedSubjects, subjects, crb.Subjects)
		}
		mutatedPod := p.DeepCopy()

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		if diff := cmp.Diff(mutatedPod, p); diff != "" {
			t.Fatalf("unexpected Pod diff\n: %v", diff)
		}
	})

	t.Run("does_not_add_the_same_subject_to_ClusterRoleBinding_twice", func(t *testing.T) {
		t.Parallel()

		crb := getCRB(agent.DefaultResourcePrefix)
		c := fake.NewClientBuilder().WithObjects(crb).Build()

		i, err := getConfig().New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req); err != nil {
			t.Fatalf("mutating first time: %v", err)
		}

		if err := i.Mutate(testutil.ContextWithDeadline(t), getEmptyPod(), req); err != nil {
			t.Fatalf("mutating second time: %v", err)
		}

		key := client.ObjectKey{
			Name: crb.Name,
		}

		if err = c.Get(ctx, key, crb); err != nil {
			t.Fatalf("getting ClusterRoleBinding: %v", err)
		}

		expectedSubjects := 1

		if subjects := len(crb.Subjects); subjects != expectedSubjects {
			t.Fatalf("expected %d subjects, got %d: %v", expectedSubjects, subjects, crb.Subjects)
		}
	})

	t.Run("changes_configuration_hash_label_when_configuration_changes", func(t *testing.T) { t.Parallel() })

	t.Run("updates_license_secret_when_license_key_changes", func(t *testing.T) {
		t.Parallel()
	})

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
}

//nolint:funlen,gocognit,cyclop
func Test_Mutation_hash(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	req := webhook.RequestOptions{
		Namespace: testNamespace,
	}

	// This also tests that all configurable knobs are included included into container as we hash
	// entire sidecar configuration.
	t.Run("changes_when", func(t *testing.T) {
		t.Parallel()

		basePod := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, basePod, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		baseHash, ok := basePod.ObjectMeta.Labels[agent.InjectedLabel]
		if !ok {
			t.Fatalf("missing injected label")
		}

		cases := map[string]func(*agent.InjectorConfig){
			"resource_prefix": func(config *agent.InjectorConfig) {
				config.ResourcePrefix = "bar"

				if err := c.Create(ctx, getCRB(config.ResourcePrefix), &client.CreateOptions{}); err != nil {
					t.Fatalf("creating ClusterRoleBinding with custom prefix: %v", err)
				}
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
			"image_tag": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.Tag = "bar"
			},
			"image_pull_policy": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.PullPolicy = corev1.PullAlways
			},
			/*
				"image_pull_secrets": func(c *agent.InjectorConfig) {
					c.AgentConfig.Image.PullSecrets = []corev1.LocalObjectReference{
						{
							Name: "foo",
						},
					}
				},*/
			"runnable_user": func(c *agent.InjectorConfig) {
				c.AgentConfig.PodSecurityContext.RunAsUser = 1000
			},
			"runnable_group": func(c *agent.InjectorConfig) {
				c.AgentConfig.PodSecurityContext.RunAsGroup = 1000
			},
		}

		for testCaseName, mutateConfigF := range cases {
			mutateConfigF := mutateConfigF

			t.Run(fmt.Sprintf("%s_changes", testCaseName), func(t *testing.T) {
				t.Parallel()

				p := getEmptyPod()
				config := getConfig()

				mutateConfigF(config)

				i, err := config.New(c, c, nil)
				if err != nil {
					t.Fatalf("creating injector: %v", err)
				}

				if err := i.Mutate(ctx, p, req); err != nil {
					t.Fatalf("mutating Pod: %v", err)
				}

				newHash, ok := p.ObjectMeta.Labels[agent.InjectedLabel]
				if !ok {
					t.Fatalf("missing injected label")
				}

				if baseHash == newHash {
					t.Fatalf("no hash change detected")
				}
			})
		}
	})

	t.Run("does_not_change_when_pod_definition_changes", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, nil)
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating first pod: %v", err)
		}

		firstHash, ok := p.ObjectMeta.Labels[agent.InjectedLabel]
		if !ok {
			t.Fatalf("missing injected label")
		}

		p2 := getEmptyPod()
		p2.Labels = map[string]string{"newLabel": "newValue"}
		p2.Spec.Containers = append(p2.Spec.Containers, corev1.Container{Name: "test-container"})

		if err := i.Mutate(ctx, p2, req); err != nil {
			t.Fatalf("mutating second pod: %v", err)
		}

		secondHash, ok := p2.ObjectMeta.Labels[agent.InjectedLabel]
		if !ok {
			t.Fatalf("missing injected label")
		}

		if firstHash != secondHash {
			t.Fatalf("expected labels to match, got %q and %q", firstHash, secondHash)
		}
	})
}

func clusterRoleBindingName(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, agent.ClusterRoleBindingSuffix)
}

func secretName() string {
	return fmt.Sprintf("%s%s", agent.DefaultResourcePrefix, agent.LicenseSecretSuffix)
}

func getCRB(prefix string) *v1.ClusterRoleBinding {
	return &v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName(prefix),
		},
	}
}

func getSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName(),
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"license": []byte(testLicense),
		},
	}
}

func getConfig() *agent.InjectorConfig {
	return &agent.InjectorConfig{
		AgentConfig: &agent.InfraAgentConfig{},
		License:     testLicense,
	}
}

func getEmptyPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "foo",
					Image: "nginx",
				},
			},
		},
		Status: corev1.PodStatus{},
	}
}
