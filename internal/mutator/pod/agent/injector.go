// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// Package agent implements injection of infrastructure-agent container into given Pod.
package agent

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	agentSidecarName        = "newrelic-infrastructure-sidecar"
	agentSidecarLicenseKey  = "license"
	agentInjectedLabel      = "newrelic/agent-injected"
	agentInjectedLabelValue = "true"
	computeTypeServerless   = "serverless"

	envCustomAttribute        = "NRIA_CUSTOM_ATTRIBUTES"
	envPassthorughEnvironment = "NRIA_PASSTHROUGH_ENVIRONMENT"
	envNodeNAme               = "NRK8S_NODE_NAME"
	envClusterName            = "CLUSTER_NAME"
	envDisplayName            = "NRIA_DISPLAY_NAME"
	envLicenseKey             = "NRIA_LICENSE_KEY"
)

// Injector holds agent injection configuration.
type Injector struct {
	Config

	secretController *LicenseController
	rbacController   *ClusterRoleBindingController

	// This is the base container that will be used as base for the injection
	container corev1.Container
}

// Config of the Injector used to pass the required data to build it.
type Config struct {
	Logger      *logrus.Logger
	Client      client.Client
	AgentConfig InfraAgentConfig
}

// New function is the constructor for the injector struct.
func New(config Config) (*Injector, error) {
	i := Injector{}
	i.Config = config

	i.container.Env = append(i.container.Env, standardEnvVar(config.AgentConfig.ReleaseName)...)
	i.container.Env = append(i.container.Env, extraEnvVar(config.AgentConfig)...)

	resources, err := computeResourceRequirements(config.AgentConfig)
	if err != nil {
		return nil, fmt.Errorf("parsing ResourceRequirements: %w", err)
	}

	if resources != nil {
		i.container.Resources = *resources
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

	i.secretController = NewLicenseController(
		config.Client,
		getLicenseSecretName(config.AgentConfig.ReleaseName),
		agentSidecarLicenseKey,
		config.AgentConfig.LicenseKey,
		nil,
		config.Logger)
	i.rbacController = NewClusterRoleBindingController(config.Client,
		getRBACName(config.AgentConfig.ReleaseName), config.Logger)

	return &i, nil
}

func computeResourceRequirements(s InfraAgentConfig) (*corev1.ResourceRequirements, error) {
	if s.ResourceRequirements == nil {
		// TODO we should set default values anyway
		return nil, nil
	}

	resources := corev1.ResourceRequirements{}

	memoryLimit, err := resource.ParseQuantity(s.ResourceRequirements.Limits.Memory)
	if err != nil {
		return nil, fmt.Errorf("parsing memoryLimit: %w", err)
	}

	memoryRequest, err := resource.ParseQuantity(s.ResourceRequirements.Requests.Memory)
	if err != nil {
		return nil, fmt.Errorf("parsing memoryRequest: %w", err)
	}

	CPULimit, err := resource.ParseQuantity(s.ResourceRequirements.Limits.CPU)
	if err != nil {
		return nil, fmt.Errorf("parsing CPULimit: %w", err)
	}

	CPURequest, err := resource.ParseQuantity(s.ResourceRequirements.Requests.CPU)
	if err != nil {
		return nil, fmt.Errorf("parsing CPURequest: %w", err)
	}

	resources = corev1.ResourceRequirements{
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: memoryLimit,
			corev1.ResourceCPU:    CPULimit,
		},
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: memoryRequest,
			corev1.ResourceCPU:    CPURequest,
		},
	}

	return &resources, nil
}

// Mutate mutates given Pod object by injecting infrastructure-agent container into it with all dependencies.
func (i *Injector) Mutate(ctx context.Context, pod *corev1.Pod, namespace string) error {
	containerToInject := i.container

	if !i.shouldInjectContainer(ctx, pod, namespace) {
		return nil
	}

	if err := i.verifyContainerInjectability(ctx, pod, namespace); err != nil {
		return fmt.Errorf("verifying container injectability: %w", err)
	}

	if pod.Labels == nil {
		pod.Labels = map[string]string{}
	}

	pod.Labels[agentInjectedLabel] = agentInjectedLabelValue

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

func (i *Injector) shouldInjectContainer(ctx context.Context, pod *corev1.Pod, namespace string) bool {
	// TODO
	// We should check the different labels/namespaces
	// We should check if we want to inject it (es: job, newrelic agent, et)
	// We should check if it is already injected
	return true
}

func (i *Injector) verifyContainerInjectability(
	ctx context.Context,
	pod *corev1.Pod,
	namespace string) error {
	if err := i.secretController.AssureExistence(ctx, namespace); err != nil {
		i.Logger.Warnf("It is not possible to inject the container since we cannot assure the existence of secret")

		return fmt.Errorf("assuring secret presence: %w", err)
	}

	err := i.rbacController.AssureClusterRoleBindingExistence(ctx, pod.Spec.ServiceAccountName, namespace)
	if err != nil {
		i.Logger.Warnf("It is not possible to inject the container since the cannot assure the existence of crb")

		return fmt.Errorf("assuring clusterrolebinding presence: %w", err)
	}

	return nil
}

func getRBACName(releaseName string) string {
	return fmt.Sprintf("%s-infra-agent", releaseName)
}

func getLicenseSecretName(releaseName string) string {
	return fmt.Sprintf("%s-config", releaseName)
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

func standardEnvVar(releaseName string) []corev1.EnvVar {
	return []corev1.EnvVar{
		{
			Name: envLicenseKey,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: getLicenseSecretName(releaseName),
					},
					Key: agentSidecarLicenseKey,
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

func extraEnvVar(s InfraAgentConfig) []corev1.EnvVar {
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
