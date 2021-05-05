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
	agentSidecarName = "newrelic-infrastructure-sidecar"

	// AgentInjectedLabel is the name of the label injected in pod.
	AgentInjectedLabel    = "newrelic/agent-injected"
	computeTypeServerless = "serverless"

	envCustomAttribute        = "NRIA_CUSTOM_ATTRIBUTES"
	envPassthorughEnvironment = "NRIA_PASSTHROUGH_ENVIRONMENT"
	envNodeNAme               = "NRK8S_NODE_NAME"
	envClusterName            = "CLUSTER_NAME"
	envDisplayName            = "NRIA_DISPLAY_NAME"
	envLicenseKey             = "NRIA_LICENSE_KEY"
)

// injector holds agent injection configuration.
type injector struct {
	*Config

	// This is the base container that will be used as base for the injection.
	container corev1.Container
}

// Config of the Injector used to pass the required data to build it.
type Config struct {
	Logger *logrus.Logger
	Client client.Client

	AgentConfig *InfraAgentConfig

	ResourcePrefix string

	LicenseSecretName  string
	LicenseSecretKey   string
	LicenseSecretValue []byte

	ClusterRoleBindingName string
}

// New function is the constructor for the injector struct.
// nolint:revive,golint
func New(config *Config) (*injector, error) {
	if config == nil {
		config = &Config{}
	}

	if config.AgentConfig == nil {
		config.AgentConfig = &InfraAgentConfig{}
	}

	i := injector{}
	i.Config = config

	i.container.Env = append(i.container.Env, standardEnvVar(config.ResourcePrefix)...)
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

	i.container.ImagePullPolicy = corev1.PullIfNotPresent
	if i.container.ImagePullPolicy != "" {
		i.container.ImagePullPolicy = corev1.PullPolicy(config.AgentConfig.Image.PullPolicy)
	}

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

	containerHash, err := computeHash(containerToInject)
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
	// TODO
	// We should check the different labels/namespaces
	// We should check if we want to inject it (es: job, newrelic agent, et)
	// We should check if it is already injected
	return true
}

func (i *injector) verifyContainerInjectability(ctx context.Context, pod *corev1.Pod, namespace string) error {
	if err := i.ensureLicenseSecretExistence(ctx, namespace); err != nil {
		return fmt.Errorf("assuring secret presence: %w", err)
	}

	if err := i.ensureClusterRoleBindingSubject(ctx, pod.Spec.ServiceAccountName, namespace); err != nil {
		return fmt.Errorf("assuring clusterrolebinding presence: %w", err)
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

func standardEnvVar(resourcePrefix string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: envLicenseKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: GetLicenseSecretName(resourcePrefix),
					},
					Key: GetRBACName(resourcePrefix),
				},
			},
		},
		{
			Name: envNodeNAme,
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

func computeHash(c corev1.Container) (string, error) {
	b, err := yaml.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("computing hash: %w", err)
	}

	h := sha1.New()
	h.Write(b)

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// GetRBACName computes clusterRoleBindingName.
func GetRBACName(resourcePrefix string) string {
	return fmt.Sprintf("%s-infra-agent", resourcePrefix)
}

// GetLicenseSecretName computes secret name for license.
func GetLicenseSecretName(resourcePrefix string) string {
	return fmt.Sprintf("%s-config", resourcePrefix)
}
