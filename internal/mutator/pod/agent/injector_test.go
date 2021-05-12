// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	testNamespace   = "test-namespace"
	testLicense     = "test-license"
	testClusterName = "test-cluster"

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
			"license_is_empty": func(c *agent.InjectorConfig) {
				c.License = ""
			},
			"cluster_name_is_empty": func(c *agent.InjectorConfig) {
				c.ClusterName = ""
			},
			"clusterName_custom_attribute_is_defined": func(c *agent.InjectorConfig) {
				c.CustomAttributes = []agent.CustomAttribute{
					{
						Name:         "clusterName",
						DefaultValue: "foo",
					},
				}
			},
			"custom_attribute_has_no_name_set": func(c *agent.InjectorConfig) {
				c.CustomAttributes = []agent.CustomAttribute{
					{
						DefaultValue: "test-value",
					},
				}
			},
			"custom_attribute_has_no_default_value_or_fromLabel_source_set": func(c *agent.InjectorConfig) {
				c.CustomAttributes = []agent.CustomAttribute{
					{
						Name: "attributeWithoutValue",
					},
				}
			},
			"there_is_no_injection_policies_defined": func(c *agent.InjectorConfig) {
				c.Policies = nil
			},
			"invalid_namespace_selector_is_configured": func(c *agent.InjectorConfig) {
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
			"invalid_pod_selector_is_configured": func(c *agent.InjectorConfig) {
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
		}

		for testCaseName, mutateF := range cases {
			mutateF := mutateF

			t.Run(testCaseName, func(t *testing.T) {
				t.Parallel()

				config := getConfig()
				mutateF(config)

				c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

				i, err := config.New(c, c)
				if err == nil {
					t.Errorf("expected error from creating injector")
				}

				if i != nil {
					t.Errorf("expected to not get injector instance when creation error occurs")
				}
			})
		}
	})

	t.Run("succeeds_when_only_required_config_options_are_set", func(t *testing.T) {
		t.Parallel()

		config := getConfig()
		config.AgentConfig = nil

		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

		if _, err := config.New(c, c); err != nil {
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c)
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

					for _, container := range p.Spec.Containers {
						for _, envVar := range container.Env {
							if strings.Contains(envVar.Value, c.key) && strings.Contains(envVar.Value, c.value) {
								found = true
							}
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
				Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
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
	})

	t.Run("when_succeeds_with_dry_run_option", func(t *testing.T) {
		t.Parallel()

		req := webhook.RequestOptions{
			Namespace: testNamespace,
			DryRun:    true,
		}

		p := getEmptyPod()
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c)
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
				Name: clusterRoleBindingName(agent.DefaultResourcePrefix),
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

		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()
		config.CustomAttributes = nil

		i, err := config.New(c, c)
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c)
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

		i, err := config.New(c, c)
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

		crb := getCRB(agent.DefaultResourcePrefix)
		c := fake.NewClientBuilder().WithObjects(crb).Build()

		i, err := getConfig().New(c, c)
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
				objects = append(objects, getCRB(agent.DefaultResourcePrefix))

				c := fake.NewClientBuilder().WithObjects(objects...).Build()

				i, err := config.New(c, c)
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
				objects = append(objects, getCRB(agent.DefaultResourcePrefix))

				c := fake.NewClientBuilder().WithObjects(objects...).Build()

				i, err := config.New(c, c)
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c)
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

		i, err = config.New(c, c)
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

		t.Run("infrastructure_agent_ClusterRoleBinding_do_not_exist", func(t *testing.T) {
			t.Parallel()

			p := getEmptyPod()
			c := fake.NewClientBuilder().Build()
			config := getConfig()

			i, err := config.New(c, c)
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

			c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
			config := getConfig()

			i, err := config.New(c, c)
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

			c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()

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

			i, err := config.New(c, c)
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
		c := fake.NewClientBuilder().WithObjects(getCRB(agent.DefaultResourcePrefix)).Build()
		config := getConfig()

		i, err := config.New(c, c)
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
				c.AgentConfig.ExtraEnvVars = map[string]string{"foo": "baz"}
			},
			"resources": func(c *agent.InjectorConfig) {
				cpuLimit := *resource.NewScaledQuantity(100, resource.Milli)

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
				c.AgentConfig.Image.Tag = "baz"
			},
			"image_pull_policy": func(c *agent.InjectorConfig) {
				c.AgentConfig.Image.PullPolicy = corev1.PullAlways
			},
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

				i, err := config.New(c, c)
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

		i, err := config.New(c, c)
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

func clusterRoleBindingName(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, agent.ClusterRoleBindingSuffix)
}

func secretName() string {
	return fmt.Sprintf("%s%s", agent.DefaultResourcePrefix, agent.LicenseSecretSuffix)
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
		AgentConfig: &agent.InfraAgentConfig{},
		License:     testLicense,
		ClusterName: testClusterName,
		Policies:    []agent.InjectionPolicy{{}},
		CustomAttributes: []agent.CustomAttribute{
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
		},
	}
}

func getEmptyPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: testNamespace,
			Labels: map[string]string{
				customAttributeFromLabel:           customAttributeFromLabelValue,
				customAttributeWithEmptyValueLabel: "",
			},
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
