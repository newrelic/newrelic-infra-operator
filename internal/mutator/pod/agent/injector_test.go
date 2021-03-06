// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	testNamespace      = "test-namespace"
	testLicense        = "test-license"
	testClusterName    = "test-cluster"
	testResourcePrefix = "test-resource"

	customAttributeFromLabelName  = "fromLabel"
	customAttributeFromLabel      = "custom-attribute-from-label"
	customAttributeFromLabelValue = "test-value"

	customAttributeDefaultValueName = "withDefaultValue"
	customAttributeDefaultValue     = "defaultValue"

	customAttributeWithEmptyValueName         = "withEmptyValue"
	customAttributeWithEmptyValueLabel        = "with-empty-value"
	customAttributeWithEmptyValueDefaultValue = "anotherDefaultValue"
)

//nolint:funlen
func Test_Creating_injector(t *testing.T) {
	t.Parallel()

	t.Run("fails_when", func(t *testing.T) {
		t.Parallel()

		cases := map[string]func(*agent.InjectorConfig){
			"image_tag_is_empty": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.Tag = ""
			},
			"image_repository_is_empty": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.Repository = ""
			},
			"license_is_empty": func(c *agent.InjectorConfig) {
				c.License = ""
			},
			"cluster_name_is_empty": func(c *agent.InjectorConfig) {
				c.ClusterName = ""
			},
			"resource_prefix_is_empty": func(c *agent.InjectorConfig) {
				c.ResourcePrefix = ""
			},
			"clusterName_custom_attribute_is_defined": func(c *agent.InjectorConfig) {
				c.AgentConfig.CustomAttributes = []agent.CustomAttribute{
					{
						Name:         "clusterName",
						DefaultValue: "foo",
					},
				}
			},
			"custom_attribute_has_no_name_set": func(c *agent.InjectorConfig) {
				c.AgentConfig.CustomAttributes = []agent.CustomAttribute{
					{
						DefaultValue: "test-value",
					},
				}
			},
			"custom_attribute_has_no_default_value_or_fromLabel_source_set": func(c *agent.InjectorConfig) {
				c.AgentConfig.CustomAttributes = []agent.CustomAttribute{
					{
						Name: "attributeWithoutValue",
					},
				}
			},
			"there_is_no_injection_policies_defined": func(c *agent.InjectorConfig) {
				c.Policies = nil
			},
			"invalid_namespace_selector_is_configured_for_injection_policy": func(c *agent.InjectorConfig) {
				c.Policies = []agent.InjectionPolicy{
					{
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"_": "bad_value-",
							},
						},
					},
				}
			},
			"invalid_pod_selector_is_configured_for_injection_policy": func(c *agent.InjectorConfig) {
				c.Policies = []agent.InjectionPolicy{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"_": "bad_value-",
							},
						},
					},
				}
			},
			"invalid_pod_selector_is_configured_for_agent_config": func(c *agent.InjectorConfig) {
				c.AgentConfig.ConfigSelectors = []agent.ConfigSelector{
					{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"_": "bad_value-",
							},
						},
					},
				}
			},
		}

		for testCaseName, mutateF := range cases {
			mutateF := mutateF

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				config := getConfig()
				mutateF(config)

				c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()

				i, err := config.New(c, c, logrus.New())
				if err == nil {
					t.Errorf("expected error from creating injector")
				}

				if i != nil {
					t.Errorf("expected to not get injector instance when creation error occurs")
				}
			})
		}

		t.Run("logger_is_empty", func(t *testing.T) {
			t.Parallel()
			config := getConfig()

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()

			i, err := config.New(c, c, nil)
			if err == nil {
				t.Errorf("expected error from creating injector")
			}

			if i != nil {
				t.Errorf("expected to not get injector instance when creation error occurs")
			}
		})
	})

	t.Run("succeeds_when_only_required_config_options_are_set", func(t *testing.T) {
		t.Parallel()

		config := getConfig()

		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()

		if _, err := config.New(c, c, logrus.New()); err != nil {
			t.Fatalf("creating injector: %v", err)
		}
	})
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

		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
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

		t.Run("includes_custom_attribute_with", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			p.Labels = map[string]string{
				customAttributeFromLabel:           customAttributeFromLabelValue,
				customAttributeWithEmptyValueLabel: "",
			}

			config := getConfig()
			config.AgentConfig.CustomAttributes = []agent.CustomAttribute{
				{
					Name:      customAttributeFromLabelName,
					FromLabel: customAttributeFromLabel,
				},
				{
					Name:         customAttributeDefaultValueName,
					DefaultValue: customAttributeDefaultValue,
				},
				{
					Name:         customAttributeWithEmptyValueName,
					FromLabel:    customAttributeWithEmptyValueLabel,
					DefaultValue: customAttributeWithEmptyValueDefaultValue,
				},
			}

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err != nil {
				t.Fatalf("mutating Pod: %v", err)
			}

			cases := map[string]struct {
				key   string
				value string
			}{
				"cluster_name": {
					key:   testClusterName,
					value: "clusterName",
				},
				"value_from_label": {
					key:   customAttributeFromLabelName,
					value: customAttributeFromLabelValue,
				},
				"default_value": {
					key:   customAttributeDefaultValueName,
					value: customAttributeDefaultValue,
				},
				"default_value_if_value_from_label_is_empty": {
					key:   customAttributeWithEmptyValueName,
					value: customAttributeWithEmptyValueDefaultValue,
				},
			}

			for testCaseName, c := range cases {
				c := c

				t.Run(testCaseName, func(t *testing.T) {
					t.Parallel()

					found := false

					infraContainer := infraContainer(t, p)

					for _, envVar := range infraContainer.Env {
						if strings.Contains(envVar.Value, c.key) && strings.Contains(envVar.Value, c.value) {
							found = true
						}
					}

					if !found {
						t.Fatalf("cluster name custom attribute not found in pod\n%s", cmp.Diff(&corev1.Pod{}, p))
					}
				})
			}
		})

		t.Run("adds_pods_ServiceAccount_to_infrastructure_agent_ClusterRoleBinding", func(t *testing.T) {
			t.Parallel()

			key := client.ObjectKey{
				Name: clusterRoleBindingName(testResourcePrefix),
			}

			crb := &rbacv1.ClusterRoleBinding{}

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

		t.Run("and_when_RunAsUser_is_configured_to_non_zero_value_it_gets_set_on_injected_container", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()

			expectedUID := pointer.Int64Ptr(1000)
			config.AgentConfig.PodSecurityContext.RunAsUser = *expectedUID

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err != nil {
				t.Fatalf("mutating Pod: %v", err)
			}

			infraContainer := infraContainer(t, p)

			if *infraContainer.SecurityContext.RunAsUser != *expectedUID {
				t.Fatalf("unexpected RunAsUser value %d, expected %d", infraContainer.SecurityContext.RunAsUser, expectedUID)
			}
		})

		t.Run("and_when_RunAsGroup_is_configured_to_non_zero_value_it_gets_set_on_injected_container", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()

			expectedGID := pointer.Int64Ptr(1000)
			config.AgentConfig.PodSecurityContext.RunAsGroup = *expectedGID

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err != nil {
				t.Fatalf("mutating Pod: %v", err)
			}

			infraContainer := infraContainer(t, p)

			if *infraContainer.SecurityContext.RunAsGroup != *expectedGID {
				t.Fatalf("unexpected RunAsGroup value %d, expected %d", infraContainer.SecurityContext.RunAsGroup, expectedGID)
			}
		})

		t.Run("and_when_Pod_matches_the_first_matching_config_rule_it_gets", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			p.Labels["matching-key"] = "matching-value"

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()
			config.AgentConfig.ConfigSelectors = []agent.ConfigSelector{
				{
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"not-matching-key": "not-matching-value",
						},
					},
				},
				{
					ExtraEnvVars: map[string]string{
						"EXTRA_ENV": "EXTRA_VALUE",
					},
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewScaledQuantity(300, resource.Mega),
						},
					},
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"matching-key": "matching-value",
						},
					},
				},
			}

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err != nil {
				t.Fatalf("mutating Pod: %v", err)
			}

			infraContainer := infraContainer(t, p)

			t.Run("env_var_configured_from_it", func(t *testing.T) {
				t.Parallel()

				found := false
				for _, env := range infraContainer.Env {
					if env.Name == "EXTRA_ENV" && env.Value == "EXTRA_VALUE" {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing expecting env var")
				}
			})

			t.Run("resources_configured_from_it", func(t *testing.T) {
				t.Parallel()

				q := infraContainer.Resources.Limits[corev1.ResourceMemory]
				if q != *resource.NewScaledQuantity(300, resource.Mega) {
					t.Fatalf("not-default CPU limit was expected: %s.", q.String())
				}
			})
		})

		t.Run("and_when_Pod_does_not_match_any_config_rule_it_gets", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()
			config.AgentConfig.ConfigSelectors = []agent.ConfigSelector{
				{
					ResourceRequirements: &corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceMemory: *resource.NewScaledQuantity(100, resource.Mega),
						},
					},
					LabelSelector: metav1.LabelSelector{
						MatchLabels: map[string]string{
							"not-matching-key": "not-matching-value",
						},
					},
				},
			}

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err != nil {
				t.Fatalf("mutating Pod: %v", err)
			}

			infraContainer := infraContainer(t, p)

			t.Run("sidecar_resources_empty", func(t *testing.T) {
				t.Parallel()

				if diff := cmp.Diff(infraContainer.Resources, corev1.ResourceRequirements{}); diff != "" {
					t.Fatalf("unexpected resources diff:\n%s", diff)
				}
			})
		})
	})

	t.Run("when_succeeds_with_dry_run_option", func(t *testing.T) {
		t.Parallel()

		req := webhook.RequestOptions{
			Namespace: testNamespace,
			DryRun:    true,
		}

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
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

		t.Run("does_not_create_license_Secret_for_Pod", func(t *testing.T) {
			t.Parallel()

			secret := &corev1.Secret{}
			if err := c.Get(ctx, secretKey, secret); err == nil || !errors.IsNotFound(err) {
				t.Fatalf("secret found in the cluster or err different from expected one: %v", err)
			}
		})

		t.Run("does_not_add_pods_ServiceAccount_to_infrastructure_agent_ClusterRoleBinding", func(t *testing.T) {
			t.Parallel()

			key := client.ObjectKey{
				Name: clusterRoleBindingName(testResourcePrefix),
			}

			crb := &rbacv1.ClusterRoleBinding{}

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

			if found {
				t.Fatalf("found unexpected subject in ClusterRoleBinding, got: %v", crb)
			}
		})
	})

	t.Run("handles_pods_without_labels", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		p.Labels = nil

		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}
	})

	t.Run("is_idempotent", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
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
		crb := getCRB(testResourcePrefix)
		crb.Subjects = []rbacv1.Subject{
			{
				Name:      "default",
				Namespace: testNamespace,
				Kind:      rbacv1.GroupKind,
			},
			{
				Name:      "nondefault",
				Namespace: testNamespace,
				Kind:      rbacv1.ServiceAccountKind,
			},
			{
				Name:      "default",
				Namespace: "default",
				Kind:      rbacv1.ServiceAccountKind,
			},
		}

		c := fake.NewClientBuilder().WithObjects(crb).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		key := client.ObjectKey{
			Name: crb.Name,
		}

		expectedSubjects := len(crb.Subjects) + 1

		if err := c.Get(ctx, key, crb); err != nil {
			t.Fatalf("getting updated ClusterRoleBinding: %v", err)
		}

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

		crb := getCRB(testResourcePrefix)
		c := fake.NewClientBuilder().WithObjects(crb).Build()

		i, err := getConfig().New(c, c, logrus.New())
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

	t.Run("injects_sidecar_when", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			extraObjects  []client.Object
			podMutateF    func(*corev1.Pod)
			configMutateF func(*agent.InjectorConfig)
			reqMutateF    func(*webhook.RequestOptions)
		}{
			"owner_is_NonExistent_batch/v1": {
				podMutateF: func(p *corev1.Pod) {
					p.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "NonExistent",
							APIVersion: "batch/v1",
						},
					}
				},
			},
			"owner_is_Job_test/v1": {
				podMutateF: func(p *corev1.Pod) {
					p.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "Job",
							APIVersion: "test/v1",
						},
					}
				},
			},
			"pod_selector_matches": {
				podMutateF: func(p *corev1.Pod) {
					p.Labels["foo"] = "test"
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "test",
								},
							},
						},
					}
				},
			},
			"namespace_selector_matches": {
				extraObjects: []client.Object{
					&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNamespace,
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
						},
					}
				},
			},
			"namespace_name_matches": {
				reqMutateF: func(req *webhook.RequestOptions) {
					req.Namespace = "ban"
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceName: "ban",
						},
					}
				},
			},
			"at_least_one_policy_matches": {
				podMutateF: func(p *corev1.Pod) {
					p.Labels["foo"] = "baz"
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceName: "foo",
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "baz",
								},
							},
						},
					}
				},
			},
		}

		for testCaseName, testData := range cases {
			testData := testData

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				config := getConfig()

				if configMutateF := testData.configMutateF; configMutateF != nil {
					configMutateF(config)
				}

				objects := testData.extraObjects
				objects = append(objects, getCRB(testResourcePrefix))

				c := fake.NewClientBuilder().WithObjects(objects...).Build()

				i, err := config.New(c, c, logrus.New())
				if err != nil {
					t.Fatalf("creating injector: %v", err)
				}

				p := getEmptyPod()

				if podMutateF := testData.podMutateF; podMutateF != nil {
					podMutateF(p)
				}

				req := req
				if reqMutateF := testData.reqMutateF; reqMutateF != nil {
					reqMutateF(&req)
				}

				if err := i.Mutate(testutil.ContextWithDeadline(t), p, req); err != nil {
					t.Fatalf("mutating Pod: %v", err)
				}

				expectedContainers := 2

				if len(p.Spec.Containers) != expectedContainers {
					t.Fatalf("expected %d containers, got %d: %v", expectedContainers, len(p.Spec.Containers), p.Spec.Containers)
				}
			})
		}
	})

	t.Run("does_not_inject_sidecar_when", func(t *testing.T) {
		t.Parallel()

		cases := map[string]struct {
			extraObjects  []client.Object
			podMutateF    func(*corev1.Pod)
			configMutateF func(*agent.InjectorConfig)
			reqMutateF    func(*webhook.RequestOptions)
		}{
			"disable_injection_label_is_present": {
				podMutateF: func(p *corev1.Pod) {
					p.Labels[agent.DisableInjectionLabel] = "anyValue"
				},
			},
			"injected_label_is_present": {
				podMutateF: func(p *corev1.Pod) {
					p.Labels[agent.InjectedLabel] = "anyValue"
				},
			},
			"owner_is_Job_batch/v1": {
				podMutateF: func(p *corev1.Pod) {
					p.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "Job",
							APIVersion: "batch/v1",
						},
					}
				},
			},
			"owner_is_Job_batch/v1beta1": {
				podMutateF: func(p *corev1.Pod) {
					p.OwnerReferences = []metav1.OwnerReference{
						{
							Kind:       "Job",
							APIVersion: "batch/v1beta1",
						},
					}
				},
			},
			"there_is_no_policy_matching": {
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceName: "foo",
						},
					}
				},
			},
			"policy_matches_pod_selector_but_not_namespace_name": {
				podMutateF: func(p *corev1.Pod) {
					p.Labels["foo"] = "bah"
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceName: "foo",
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bah",
								},
							},
						},
					}
				},
			},
			"policy_matches_namespace_selector_but_not_pod_selector": {
				extraObjects: []client.Object{
					&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNamespace,
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"foo": "bar",
								},
							},
							PodSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"bar": "bam",
								},
							},
						},
					}
				},
			},
			"namespace_selector_does_not_match": {
				extraObjects: []client.Object{
					&corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: testNamespace,
							Labels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				configMutateF: func(config *agent.InjectorConfig) {
					config.Policies = []agent.InjectionPolicy{
						{
							NamespaceSelector: &metav1.LabelSelector{
								MatchLabels: map[string]string{
									"baz": "bap",
								},
							},
						},
					}
				},
			},
		}

		for testCaseName, testData := range cases {
			testData := testData

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				config := getConfig()

				if configMutateF := testData.configMutateF; configMutateF != nil {
					configMutateF(config)
				}

				objects := testData.extraObjects
				objects = append(objects, getCRB(testResourcePrefix))

				c := fake.NewClientBuilder().WithObjects(objects...).Build()

				i, err := config.New(c, c, logrus.New())
				if err != nil {
					t.Fatalf("creating injector: %v", err)
				}

				p := getEmptyPod()

				if podMutateF := testData.podMutateF; podMutateF != nil {
					podMutateF(p)
				}

				req := req
				if reqMutateF := testData.reqMutateF; reqMutateF != nil {
					reqMutateF(&req)
				}

				if err := i.Mutate(testutil.ContextWithDeadline(t), p, req); err != nil {
					t.Fatalf("mutating Pod: %v", err)
				}

				expectedContainers := 1

				if len(p.Spec.Containers) != expectedContainers {
					t.Fatalf("expected %d containers, got %d", expectedContainers, len(p.Spec.Containers))
				}
			})
		}
	})

	t.Run("updates_license_secret_when_license_key_changes", func(t *testing.T) {
		t.Parallel()

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		secret := &corev1.Secret{}
		if err := c.Get(ctx, secretKey, secret); err != nil {
			t.Fatalf("getting secret: %v", err)
		}

		license, ok := secret.Data[agent.LicenseSecretKey]
		if !ok {
			t.Fatalf("license key %q not found in created secret", license)
		}

		if string(license) != testLicense {
			t.Fatalf("expected license %q, got %q", testLicense, license)
		}

		newLicense := "bar"
		config.License = newLicense

		i, err = config.New(c, c, logrus.New())
		if err != nil {
			t.Fatalf("creating injector: %v", err)
		}

		p = getEmptyPod()

		if err := i.Mutate(ctx, p, req); err != nil {
			t.Fatalf("mutating Pod: %v", err)
		}

		secret = &corev1.Secret{}
		if err := c.Get(ctx, secretKey, secret); err != nil {
			t.Fatalf("getting secret: %v", err)
		}

		license, ok = secret.Data[agent.LicenseSecretKey]
		if !ok {
			t.Fatalf("license key %q not found in created secret", license)
		}

		if string(license) != newLicense {
			t.Fatalf("expected license %q, got %q", newLicense, license)
		}
	})

	t.Run("fails_when", func(t *testing.T) {
		t.Parallel()

		t.Run("the_mutating_Pod_has_already_a_volume_of_the_injected_container", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			p.Spec.Volumes = []corev1.Volume{
				{
					Name: "tmpfs-user-data-injected",
				},
			}

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err == nil {
				t.Fatalf("mutating Pod should fail")
			}
		})

		t.Run("infrastructure_agent_ClusterRoleBinding_do_not_exist", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			c := fake.NewClientBuilder().Build()
			config := getConfig()

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			err = i.Mutate(ctx, p, req)
			if err == nil {
				t.Fatalf("expected mutation to fail")
			}

			if !errors.IsNotFound(err) {
				t.Fatalf("expected not found error, got: %v", err)
			}
		})

		t.Run("value_for_custom_attribute_from_label_is_empty_and_there_is_no_default_value", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			p.Labels[customAttributeFromLabel] = ""

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
			config := getConfig()
			config.AgentConfig.CustomAttributes = []agent.CustomAttribute{
				{
					Name:      customAttributeFromLabelName,
					FromLabel: customAttributeFromLabel,
				},
			}

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err == nil {
				t.Fatalf("expected mutation to fail")
			}
		})

		// Unlikely to happen in real-life scenario, but may occur in tests, it's easy to implement, so let's
		// test it for completeness.
		t.Run("namespace_for_mutated_Pod_does_not_exist_anymore", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()

			c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()

			config := getConfig()
			config.Policies = []agent.InjectionPolicy{
				{
					NamespaceSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"foo": "bar",
						},
					},
				},
			}

			i, err := config.New(c, c, logrus.New())
			if err != nil {
				t.Fatalf("creating injector: %v", err)
			}

			if err := i.Mutate(ctx, p, req); err == nil {
				t.Fatalf("expected mutation to fail")
			}
		})
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
		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
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
			"image_config": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.Repository = "foo"
			},
			"pod_security_context": func(c *agent.InjectorConfig) {
				c.AgentConfig.PodSecurityContext.RunAsUser = 1000
			},
			"matching_config_selector": func(c *agent.InjectorConfig) {
				c.AgentConfig.ConfigSelectors = []agent.ConfigSelector{
					{
						ExtraEnvVars: map[string]string{
							"foo": "bar",
						},
					},
				}
			},
			"cluster_name": func(config *agent.InjectorConfig) {
				config.ClusterName = "baz"
			},
			"custom_attributes_config": func(config *agent.InjectorConfig) {
				config.AgentConfig.CustomAttributes = []agent.CustomAttribute{
					{
						Name:         "far",
						DefaultValue: "ban",
					},
				}
			},
		}

		for testCaseName, mutateConfigF := range cases {
			mutateConfigF := mutateConfigF

			t.Run(fmt.Sprintf("%s/changes", testCaseName), func(t *testing.T) {
				t.Parallel()

				p := getEmptyPod()
				config := getConfig()

				mutateConfigF(config)

				i, err := config.New(c, c, logrus.New())
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
		c := fake.NewClientBuilder().WithObjects(getCRB(testResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c, logrus.New())
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
		p2.Labels["newLabel"] = "newValue"
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

func infraContainer(t *testing.T, pod *corev1.Pod) corev1.Container {
	t.Helper()

	for _, container := range pod.Spec.Containers {
		if container.Name == agent.AgentSidecarName {
			return container
		}
	}

	t.Fatalf("infra container %q not found", agent.AgentSidecarName)

	return corev1.Container{}
}

func clusterRoleBindingName(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, agent.ClusterRoleBindingSuffix)
}

func secretName() string {
	return fmt.Sprintf("%s%s", testResourcePrefix, agent.LicenseSecretSuffix)
}

func getCRB(prefix string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleBindingName(prefix),
		},
	}
}

func getConfig() *agent.InjectorConfig {
	return &agent.InjectorConfig{
		AgentConfig: agent.InfraAgentConfig{
			Image: agent.Image{
				Tag:        "test-tag",
				Repository: "test-repository",
			},
		},
		License:        testLicense,
		ClusterName:    testClusterName,
		ResourcePrefix: testResourcePrefix,
		Policies:       []agent.InjectionPolicy{{}},
	}
}

func getEmptyPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNamespace,
			Labels:    map[string]string{},
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
