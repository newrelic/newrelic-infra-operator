// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package agent implements injection of infrastructure-agent container into given Pod.
package agent

import (
	"context"
	//nolint:gosec
	"crypto/sha1"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"k8s.io/utils/ptr"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	// DisableInjectionLabel is the name of the label to prevent agent sidecar injection.
	DisableInjectionLabel = "infra-operator.newrelic.com/disable-injection"

	// InjectedLabel is the name of the label injected in pod.
	InjectedLabel = "infra-operator.newrelic.com/agent-injected"

	// ClusterRoleBindingSuffix is the expected suffix on pre-created ClusterRoleBinding. It will be combined
	// with configured resource prefix.
	ClusterRoleBindingSuffix = "-infra-agent"

	// LicenseSecretSuffix is the suffix which will be added to created license Secret objects, combined with
	// configured resource prefix.
	LicenseSecretSuffix = "-config"

	// LicenseSecretKey is the key which under the license key is placed in license Secret object.
	LicenseSecretKey = "license"

	// AgentSidecarName is the name of the container injected.
	AgentSidecarName = "newrelic-infrastructure-sidecar"

	envCustomAttribute        = "NRIA_CUSTOM_ATTRIBUTES"
	envPassthorughEnvironment = "NRIA_PASSTHROUGH_ENVIRONMENT"
	envNodeName               = "NRK8S_NODE_NAME"
	envClusterName            = "CLUSTER_NAME"
	envDisplayName            = "NRIA_DISPLAY_NAME"
	envLicenseKey             = "NRIA_LICENSE_KEY"

	clusterNameAttribute = "clusterName"
)

var errEmpty = errors.New("empty value: ")

// injector holds agent injection configuration.
type injector struct {
	config *InjectorConfig

	// This is the base container that will be used as base for the injection.
	container corev1.Container

	clusterRoleBindingName string
	licenseSecretName      string
	license                []byte
	configHash             string
	client                 client.Client

	// We do not have permissions to list and watch secrets, so we must use uncached
	// client for them.
	noCacheClient client.Client
}

// InjectorConfig of the Injector used to pass the required data to build it.
type InjectorConfig struct {
	AgentConfig    InfraAgentConfig  `json:"agentConfig"`
	ResourcePrefix string            `json:"resourcePrefix"`
	License        string            `json:"-"`
	ClusterName    string            `json:"clusterName"`
	Policies       []InjectionPolicy `json:"policies"`
}

// InjectionPolicy represents injection policy, which defines if given Pod should have agent injected or not.
type InjectionPolicy struct {
	NamespaceName     string                `json:"namespaceName"`
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector"`
	PodSelector       *metav1.LabelSelector `json:"podSelector"`

	namespaceSelector labels.Selector `json:"-"`
	podSelector       labels.Selector `json:"-"`
}

// CustomAttributes represents collection of custom attributes.
type CustomAttributes []CustomAttribute

// CustomAttribute represents single custom attribute which will be reported by infrastructure-agent.
//
// It's value can be taken from Pod label. If label is not present, default value will be used.
//
// If default value is empty as well, error will be returned.
type CustomAttribute struct {
	Name         string `json:"name"`
	DefaultValue string `json:"defaultValue"`
	FromLabel    string `json:"fromLabel"`
}

func (cas CustomAttributes) toString(podLabels map[string]string) (string, error) {
	output := map[string]string{}

	for _, ca := range cas {
		value := ca.DefaultValue

		if l := ca.FromLabel; l != "" {
			if v, ok := podLabels[l]; ok && v != "" {
				value = v
			}
		}

		if value == "" {
			//nolint:err113
			return "", fmt.Errorf("value for custom attribute %q is empty", ca.Name)
		}

		output[ca.Name] = value
	}

	casRaw, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("marshaling attributes: %w", err)
	}

	return string(casRaw), nil
}

// Injector injects New Relic infrastructure-agent into given pod, ensuring that it has all capabilities to run
// like right permissions and access to the New Relic license key.
type Injector interface {
	Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error
}

// New function is the constructor for the injector struct.
//
//nolint:ireturn
func (config InjectorConfig) New(client, noCacheClient client.Client, logger logr.Logger) (Injector, error) {
	if !logger.Enabled() {
		//nolint:err113
		return nil, fmt.Errorf("logger is not initialized")
	}

	config.AgentConfig.CustomAttributes = append(config.AgentConfig.CustomAttributes, CustomAttribute{
		Name:         clusterNameAttribute,
		DefaultValue: config.ClusterName,
	})

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validating configuration: %w", err)
	}

	licenseSecretName := fmt.Sprintf("%s%s", config.ResourcePrefix, LicenseSecretSuffix)

	containerToInject := config.container(licenseSecretName)

	configHash := &configHash{
		CustomAttributes:   config.AgentConfig.CustomAttributes,
		ResourcePrefix:     config.ResourcePrefix,
		ClusterName:        config.ClusterName,
		Image:              config.AgentConfig.Image,
		PodSecurityContext: config.AgentConfig.PodSecurityContext,
		Container:          containerToInject,
	}

	hash, err := configHash.calculate()
	if err != nil {
		return nil, fmt.Errorf("calculating config hash: %w", err)
	}

	logger.Info("label value for Pod with no config selector", "InjectedLabel", InjectedLabel, "hash", hash)

	if err := config.buildPolicies(); err != nil {
		return nil, fmt.Errorf("building policies: %w", err)
	}

	if err := config.buildConfigSelectors(containerToInject, logger); err != nil {
		return nil, fmt.Errorf("building config selectors: %w", err)
	}

	return &injector{
		clusterRoleBindingName: fmt.Sprintf("%s%s", config.ResourcePrefix, ClusterRoleBindingSuffix),
		licenseSecretName:      licenseSecretName,
		license:                []byte(config.License),
		client:                 client,
		noCacheClient:          noCacheClient,
		container:              containerToInject,
		config:                 &config,
		configHash:             hash,
	}, nil
}

func (config *InjectorConfig) buildPolicies() error {
	for i, policy := range config.Policies {
		if policy.NamespaceSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.NamespaceSelector)
			if err != nil {
				return fmt.Errorf("parsing namespace selector from policy %d: %w", i, err)
			}

			config.Policies[i].namespaceSelector = selector
		}

		if policy.PodSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(policy.PodSelector)
			if err != nil {
				return fmt.Errorf("parsing pod selector from policy %d: %w", i, err)
			}

			config.Policies[i].podSelector = selector
		}
	}

	return nil
}

//nolint:cyclop,gocyclo
func (config InjectorConfig) validate() error {
	if config.License == "" {
		return fmt.Errorf("%w: %s", errEmpty, "license key")
	}

	if config.ClusterName == "" {
		return fmt.Errorf("%w: %s", errEmpty, "cluster name")
	}

	if config.AgentConfig.Image.Tag == "" {
		return fmt.Errorf("%w: %s", errEmpty, "config.infraAgentInjection.agentConfig.Image.Tag")
	}

	if config.AgentConfig.Image.Repository == "" {
		return fmt.Errorf("%w: %s", errEmpty, "config.infraAgentInjection.agentConfig.Image.Repository")
	}

	if config.ResourcePrefix == "" {
		return fmt.Errorf("%w: %s", errEmpty, "config.infraAgentInjection.ResourcePrefix")
	}

	customAttributeNames := map[string]struct{}{}

	for i, ca := range config.AgentConfig.CustomAttributes {
		if ca.Name == "" {
			//nolint:err113
			return fmt.Errorf("custom attribute %d has empty name", i)
		}

		if ca.DefaultValue == "" && ca.FromLabel == "" {
			//nolint:err113
			return fmt.Errorf("custom attribute %q has no value defined", ca.Name)
		}

		if _, ok := customAttributeNames[ca.Name]; ok {
			//nolint:err113
			return fmt.Errorf("duplicate custom attribute %q defined", ca.Name)
		}

		customAttributeNames[ca.Name] = struct{}{}
	}

	if len(config.Policies) == 0 {
		//nolint:err113
		return fmt.Errorf("at least one injection policy must be configured")
	}

	return nil
}

func (config InjectorConfig) container(licenseSecretName string) corev1.Container {
	c := corev1.Container{
		Image:           fmt.Sprintf("%s:%s", config.AgentConfig.Image.Repository, config.AgentConfig.Image.Tag),
		Name:            AgentSidecarName,
		ImagePullPolicy: config.AgentConfig.Image.PullPolicy,
		Env:             standardEnvVar(licenseSecretName, config.ClusterName),
		VolumeMounts:    standardVolumes(),
		SecurityContext: &corev1.SecurityContext{
			ReadOnlyRootFilesystem:   ptr.To[bool](true),
			AllowPrivilegeEscalation: ptr.To[bool](false),
		},
	}

	if config.AgentConfig.PodSecurityContext.RunAsUser != 0 {
		c.SecurityContext.RunAsUser = &config.AgentConfig.PodSecurityContext.RunAsUser
	}

	if config.AgentConfig.PodSecurityContext.RunAsGroup != 0 {
		c.SecurityContext.RunAsGroup = &config.AgentConfig.PodSecurityContext.RunAsGroup
	}

	return c
}

// Mutate mutates given Pod object by injecting infrastructure-agent container into it with all dependencies.
func (i *injector) Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error {
	containerToInject := i.container

	injectContainer, err := i.shouldInjectContainer(ctx, pod, requestOptions.Namespace)
	if err != nil {
		return fmt.Errorf("checking if agent container should be injected: %w", err)
	}

	if !injectContainer {
		return nil
	}

	if err := i.canInjectContainer(pod, containerToInject); err != nil {
		return fmt.Errorf("checking if agent container can be injected: %w", err)
	}

	if err := i.ensureSidecarDependencies(ctx, pod, requestOptions); err != nil {
		return fmt.Errorf("ensuring sidecar dependencies: %w", err)
	}

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}

	pod.Labels[InjectedLabel] = i.applyAgentConfig(pod.Labels, &containerToInject)

	customAttributes, err := i.config.AgentConfig.CustomAttributes.toString(pod.Labels)
	if err != nil {
		return fmt.Errorf("creating custom attributes: %w", err)
	}

	containerToInject.Env = append(containerToInject.Env,
		corev1.EnvVar{
			Name:  envCustomAttribute,
			Value: customAttributes,
		},
	)

	pod.Spec.Containers = append(pod.Spec.Containers, containerToInject)
	pod.Spec.Volumes = append(pod.Spec.Volumes, toEmptyDirVolumes(containerToInject.VolumeMounts)...)

	return nil
}

func (i *injector) shouldInjectContainer(ctx context.Context, pod *corev1.Pod, namespace string) (bool, error) {
	if _, hasInjectedLabel := pod.Labels[InjectedLabel]; hasInjectedLabel {
		return false, nil
	}

	if _, hasDisableInjectionLabel := pod.Labels[DisableInjectionLabel]; hasDisableInjectionLabel {
		return false, nil
	}

	// In case the pods has been created by a Job we do not inject the Pod.
	for _, o := range pod.GetOwnerReferences() {
		// Notice that also CronJobs are excluded since they creates Jobs that then create and own Pods.
		if o.Kind == "Job" && (o.APIVersion == "batch/v1" || o.APIVersion == "batch/v1beta1") {
			return false, nil
		}
	}

	ns, err := i.policyNamespace(ctx, namespace)
	if err != nil {
		return false, fmt.Errorf("getting Namespace %q for policy matching: %w", namespace, err)
	}

	return matchPolicies(pod, ns, i.config.Policies), nil
}

func (i *injector) canInjectContainer(pod *corev1.Pod, containerToInject corev1.Container) error {
	volumes := pod.Spec.Volumes
	duplicateVolumeNames := getDuplicateVolumeNames(append(volumes, toEmptyDirVolumes(containerToInject.VolumeMounts)...))

	// Checking if there is any overlapping with the volumes we want to mount and the volumes already present.
	if len(duplicateVolumeNames) > 0 {
		//nolint:err113
		return fmt.Errorf("injecting agent would produce duplicate Pod volumes: %s",
			strings.Join(duplicateVolumeNames, ","))
	}

	return nil
}

func getDuplicateVolumeNames(volumes []corev1.Volume) []string {
	duplicates := []string{}
	unique := map[string]struct{}{}

	for _, v := range volumes {
		if _, ok := unique[v.Name]; ok {
			duplicates = append(duplicates, v.Name)
		}

		unique[v.Name] = struct{}{}
	}

	return duplicates
}

// policyNamespace returns Namespace object suitable for policy matching. If there is at least one policy
// using namespaceSelector, full Namespace object is fetched, otherwise just stub object with filled name
// is returned.
func (i *injector) policyNamespace(ctx context.Context, namespace string) (*corev1.Namespace, error) {
	for _, policy := range i.config.Policies {
		if policy.namespaceSelector != nil {
			return i.getNamespace(ctx, namespace)
		}
	}

	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, nil
}

// getNamespace fetches namespace object by name.
func (i *injector) getNamespace(ctx context.Context, namespace string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}

	key := client.ObjectKey{
		Name: namespace,
	}

	if err := i.client.Get(ctx, key, ns); err != nil {
		return nil, fmt.Errorf("getting Namespace %q: %w", namespace, err)
	}

	return ns, nil
}

// matchPolicies checks if given Pod matches any of given policies.
func matchPolicies(pod *corev1.Pod, ns *corev1.Namespace, policies []InjectionPolicy) bool {
	for _, policy := range policies {
		policy := policy

		if matchPolicy(pod, ns, &policy) {
			return true
		}
	}

	return false
}

// matchPolicy checks if given Pod is matching given policy.
func matchPolicy(pod *corev1.Pod, ns *corev1.Namespace, policy *InjectionPolicy) bool {
	if policy.NamespaceName != "" && ns.Name != policy.NamespaceName {
		return false
	}

	if policy.podSelector != nil && !policy.podSelector.Matches(fields.Set(pod.Labels)) {
		return false
	}

	if policy.namespaceSelector != nil && !policy.namespaceSelector.Matches(fields.Set(ns.Labels)) {
		return false
	}

	return true
}

func (i *injector) ensureSidecarDependencies(ctx context.Context, pod *corev1.Pod, options webhook.RequestOptions) error {
	if options.DryRun {
		return nil
	}

	if err := i.ensureLicenseSecretExistence(ctx, options.Namespace); err != nil {
		return fmt.Errorf("ensuring Secret presence: %w", err)
	}

	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return i.ensureClusterRoleBindingSubject(ctx, pod.Spec.ServiceAccountName, options.Namespace)
	}); err != nil {
		return fmt.Errorf("ensuring ClusterRoleBinding subject: %w", err)
	}

	return nil
}

func (i *injector) applyAgentConfig(podLabels map[string]string, container *corev1.Container) string {
	for _, r := range i.config.AgentConfig.ConfigSelectors {
		if r.selector.Matches(labels.Set(podLabels)) {
			if r.ResourceRequirements != nil {
				container.Resources = *r.ResourceRequirements
			}

			for k, v := range r.ExtraEnvVars {
				container.Env = append(container.Env, corev1.EnvVar{Name: k, Value: v})
			}

			return r.hash
		}
	}

	return i.configHash
}

type configHash struct {
	CustomAttributes     CustomAttributes
	ResourcePrefix       string
	ClusterName          string
	Image                Image
	PodSecurityContext   PodSecurityContext
	ResourceRequirements *corev1.ResourceRequirements
	ExtraEnvVars         map[string]string
	Container            corev1.Container
}

// The logr.Logger type is an interface.
// In Go, interfaces are typically passed by value because they are already reference types under the hood.
func (config *InjectorConfig) buildConfigSelectors(container corev1.Container, logger logr.Logger) error {
	for i, r := range config.AgentConfig.ConfigSelectors {
		selector, err := metav1.LabelSelectorAsSelector(&r.LabelSelector)
		if err != nil {
			return fmt.Errorf("creating selector from label selector: %w", err)
		}

		config.AgentConfig.ConfigSelectors[i].selector = selector

		configHash := &configHash{
			CustomAttributes:     config.AgentConfig.CustomAttributes,
			ResourcePrefix:       config.ResourcePrefix,
			ClusterName:          config.ClusterName,
			Image:                config.AgentConfig.Image,
			PodSecurityContext:   config.AgentConfig.PodSecurityContext,
			ResourceRequirements: r.ResourceRequirements,
			ExtraEnvVars:         r.ExtraEnvVars,
			Container:            container,
		}

		hash, err := configHash.calculate()
		if err != nil {
			return fmt.Errorf("calculating config hash: %w", err)
		}

		logger.Info("label value for Pod with config selector", "InjectedLabel", InjectedLabel,
			"idx", i, "hash", hash)

		config.AgentConfig.ConfigSelectors[i].hash = hash
	}

	return nil
}

func standardVolumes() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "tmpfs-data-injected",
			MountPath: "/var/db/newrelic-infra/data",
		},
		{
			Name:      "tmpfs-user-data-injected",
			MountPath: "/var/db/newrelic-infra/user_data",
		},
		{
			Name:      "tmpfs-tmp-injected",
			MountPath: "/tmp",
		},
		{
			Name:      "tmpfs-cache-injected",
			MountPath: "/var/cache/nr-kubernetes",
		},
	}
}

func toEmptyDirVolumes(volumeMounts []corev1.VolumeMount) []corev1.Volume {
	volumes := []corev1.Volume{}
	for _, v := range volumeMounts {
		volumes = append(volumes, corev1.Volume{
			Name: v.Name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	return volumes
}

func standardEnvVar(secretName string, clusterName string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: envLicenseKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: LicenseSecretKey,
				},
			},
		},
		{
			Name: envNodeName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "spec.nodeName",
				},
			},
		},
		{
			Name: envDisplayName,
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					APIVersion: "v1",
					FieldPath:  "spec.nodeName",
				},
			},
		},
		{
			Name:  envClusterName,
			Value: clusterName,
		},
		{
			Name:  envPassthorughEnvironment,
			Value: getAgentPassthroughEnvironment(),
		},
	}
}

func getAgentPassthroughEnvironment() string {
	flags := []string{
		"KUBERNETES_SERVICE_HOST", "KUBERNETES_SERVICE_PORT", "CLUSTER_NAME", "CADVISOR_PORT",
		"NRK8S_NODE_NAME", "KUBE_STATE_METRICS_URL", "KUBE_STATE_METRICS_POD_LABEL", "TIMEOUT", "ETCD_TLS_SECRET_NAME",
		"ETCD_TLS_SECRET_NAMESPACE", "API_SERVER_SECURE_PORT", "KUBE_STATE_METRICS_SCHEME", "KUBE_STATE_METRICS_PORT",
		"SCHEDULER_ENDPOINT_URL", "ETCD_ENDPOINT_URL", "CONTROLLER_MANAGER_ENDPOINT_URL", "API_SERVER_ENDPOINT_URL",
		"DISABLE_KUBE_STATE_METRICS", "DISCOVERY_CACHE_TTL",
	}

	return strings.Join(flags, ",")
}

//nolint:gosec
func (c configHash) calculate() (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshaling input: %w", err)
	}

	h := sha1.New() // nosemgrep
	h.Write(b)

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
