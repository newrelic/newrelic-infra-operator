// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package agent implements injection of infrastructure-agent container into given Pod.
package agent

import (
	"context"
	"crypto/sha1"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

const (
	// AgentInjectedLabel is the name of the label injected in pod.
	AgentInjectedLabel = "newrelic/agent-injected"

	// DefaultImageRepository is the default repository from where infrastructure-agent will be pulled from.
	DefaultImageRepository = "newrelic/infrastructure-k8s"

	// DefaultImageTag is the default tag which will be pulled for infrastructure-agent image.
	DefaultImageTag = "2.4.0-unprivileged"

	// DefaultResourcePrefix is the default prefix which will be used for touched side-effect resources
	// like ClusterRoleBinding or Secrets.
	DefaultResourcePrefix = "newrelic-infra-operator"

	// ClusterRoleBindingSuffix is the expected suffix on pre-created ClusterRoleBinding. It will be combined
	// with configured resource prefix.
	ClusterRoleBindingSuffix = "-infra-agent"

	// LicenseSecretSuffix is the suffix which will be added to created license Secret objects, combined with
	// configured resource prefix.
	LicenseSecretSuffix = "-config"

	agentSidecarName = "newrelic-infrastructure-sidecar"

	computeTypeServerless = "serverless"

	envCustomAttribute        = "NRIA_CUSTOM_ATTRIBUTES"
	envPassthorughEnvironment = "NRIA_PASSTHROUGH_ENVIRONMENT"
	envNodeName               = "NRK8S_NODE_NAME"
	envClusterName            = "CLUSTER_NAME"
	envDisplayName            = "NRIA_DISPLAY_NAME"
	envLicenseKey             = "NRIA_LICENSE_KEY"
	licenseSecretKey          = "license"
)

// injector holds agent injection configuration.
type injector struct {
	*InjectorConfig

	// This is the base container that will be used as base for the injection.
	container corev1.Container

	clusterRoleBindingName string
	licenseSecretName      string
	license                []byte
	client                 client.Client

	// We do not have permissions to list and watch secrets, so we must use uncached
	// client for them.
	noCacheClient client.Client
}

// InjectorConfig of the Injector used to pass the required data to build it.
type InjectorConfig struct {
	AgentConfig    *InfraAgentConfig
	ResourcePrefix string
	License        string
}

// Injector injects New Relic infrastructure-agent into given pod, ensuring that it has all capabilities to run
// like right permissions and access to the New Relic license key.
type Injector interface {
	Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error
}

// New function is the constructor for the injector struct.
func (config InjectorConfig) New(client, noCacheClient client.Client, logger *logrus.Logger) (Injector, error) {
	if config.ResourcePrefix == "" {
		config.ResourcePrefix = DefaultResourcePrefix
	}

	if config.AgentConfig == nil {
		config.AgentConfig = &InfraAgentConfig{}
	}

	i := injector{
		clusterRoleBindingName: fmt.Sprintf("%s%s", config.ResourcePrefix, ClusterRoleBindingSuffix),
		licenseSecretName:      fmt.Sprintf("%s%s", config.ResourcePrefix, LicenseSecretSuffix),
		InjectorConfig:         &config,
		license:                []byte(config.License),
		client:                 client,
		noCacheClient:          noCacheClient,
	}

	i.container.Env = append(i.container.Env, standardEnvVar(i.licenseSecretName)...)
	i.container.Env = append(i.container.Env, extraEnvVar(config.AgentConfig)...)

	if config.AgentConfig.ResourceRequirements != nil {
		i.container.Resources = *config.AgentConfig.ResourceRequirements
	}

	i.container.SecurityContext = &corev1.SecurityContext{
		ReadOnlyRootFilesystem:   pointer.BoolPtr(true),
		AllowPrivilegeEscalation: pointer.BoolPtr(false),
	}

	if config.AgentConfig.PodSecurityContext.RunAsUser != 0 {
		i.container.SecurityContext.RunAsUser = &config.AgentConfig.PodSecurityContext.RunAsUser
	}

	if config.AgentConfig.PodSecurityContext.RunAsGroup != 0 {
		i.container.SecurityContext.RunAsGroup = &config.AgentConfig.PodSecurityContext.RunAsGroup
	}

	i.container.VolumeMounts = append(i.container.VolumeMounts, standardVolumes()...)
	i.container.Image = fmt.Sprintf("%v:%v", config.AgentConfig.Image.Repository, config.AgentConfig.Image.Tag)
	i.container.Name = agentSidecarName

	return &i, nil
}

// Mutate mutates given Pod object by injecting infrastructure-agent container into it with all dependencies.
func (i *injector) Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error {
	containerToInject := i.container

	if !i.shouldInjectContainer(ctx, pod, requestOptions.Namespace) {
		return nil
	}

	if err := i.verifyContainerInjectability(ctx, pod, requestOptions.Namespace); err != nil {
		return fmt.Errorf("verifying container injectability: %w", err)
	}

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}

	containerHash, err := hashContainer(containerToInject)
	if err != nil {
		return fmt.Errorf("computing hash to add in the label: %w", err)
	}

	pod.Labels[AgentInjectedLabel] = containerHash

	containerToInject.Env = append(containerToInject.Env,
		corev1.EnvVar{
			Name: envCustomAttribute,
			Value: fmt.Sprintf(`{"clusterName":"%s", "computeType":"%s", "fargateProfile":"%s"}`,
				os.Getenv("CLUSTER_NAME"), computeTypeServerless, pod.Labels["eks.amazonaws.com/fargate-profile"]),
		})

	pod.Spec.Containers = append(pod.Spec.Containers, containerToInject)

	volumes := []corev1.Volume{}
	for _, v := range i.container.VolumeMounts {
		// TODO We should check if the volume is already present
		volumes = append(volumes, corev1.Volume{
			Name: v.Name,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	pod.Spec.Volumes = append(pod.Spec.Volumes, volumes...)

	return nil
}

func (i *injector) shouldInjectContainer(ctx context.Context, pod *corev1.Pod, namespace string) bool {
	_, ok := pod.Labels[AgentInjectedLabel]

	// TODO
	// We should check the different labels/namespaces
	// We should check if we want to inject it (es: job, newrelic agent, et)
	// We should check if it is already injected
	return !ok
}

func (i *injector) verifyContainerInjectability(ctx context.Context, pod *corev1.Pod, namespace string) error {
	if err := i.ensureLicenseSecretExistence(ctx, namespace); err != nil {
		return fmt.Errorf("ensuring Secret presence: %w", err)
	}

	if err := i.ensureClusterRoleBindingSubject(ctx, pod.Spec.ServiceAccountName, namespace); err != nil {
		return fmt.Errorf("ensuring ClusterRoleBinding subject: %w", err)
	}

	return nil
}

func standardVolumes() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "tmpfs-data",
			MountPath: "/var/db/newrelic-infra/data",
		},
		{
			Name:      "tmpfs-user-data",
			MountPath: "/var/db/newrelic-infra/user_data",
		},
		{
			Name:      "tmpfs-tmp",
			MountPath: "/tmp",
		},
		{
			Name:      "tmpfs-cache",
			MountPath: "/var/cache/nr-kubernetes",
		},
	}
}

func standardEnvVar(secretName string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: envLicenseKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secretName,
					},
					Key: licenseSecretKey,
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
			Value: os.Getenv(envClusterName),
		},
		{
			Name:  envPassthorughEnvironment,
			Value: getAgentPassthroughEnvironment(),
		},
	}
}

func extraEnvVar(s *InfraAgentConfig) []corev1.EnvVar {
	extraEnv := []corev1.EnvVar{}
	for k, v := range s.ExtraEnvVars {
		extraEnv = append(extraEnv,
			corev1.EnvVar{
				Name:  k,
				Value: v,
			})
	}

	return extraEnv
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

func hashContainer(c corev1.Container) (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("computing hash: %w", err)
	}

	h := sha1.New()
	h.Write(b)

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
