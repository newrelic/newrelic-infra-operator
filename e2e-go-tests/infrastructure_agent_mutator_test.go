// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

//go:build e2e

// Package e2e_test implements e2e tests for operator, which are not related to any specific package.
package e2e_test

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

const (
	testPrefix = "newrelic-infra-operator-tests-"
)

// Test_Infra_agent_injection_webhook test covers the following functionality:
// - Pod gets created after mutating webhook is installed. This verifies:
//   - TLS certificate generation.
// - Pod with exclude mark (label/annotation?) do not get agent injected.
// - Pod without exclude mark get agent sidecar container injected. This verifies:
//   - Operator has ability to grant required permission to pods.
//
//nolint:funlen,cyclop,gocognit
func Test_Infra_agent_injection_webhook(t *testing.T) {
	t.Parallel()

	clientset := testClientset(t)

	ctx := testutil.ContextWithDeadline(t)

	testEndpoints := []string{
		"metrics",
		"metrics/cadvisor",
		"stats/summary",
		"pods",
	}

	testCommands := []string{"set -xe"}

	for _, endpoint := range testEndpoints {
		uri := fmt.Sprintf("/api/v1/nodes/%s/proxy/%s", randomNodeName(ctx, t, clientset), endpoint)
		testCommands = append(testCommands, fmt.Sprintf("kubectl get --raw %s", uri))
	}

	podTemplate := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testPrefix,
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:    "kubectl",
					Image:   "bitnami/kubectl",
					Command: []string{"sh", "-c", strings.Join(testCommands, "\n")},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx",
				},
			},
		},
	}

	podClient := clientset.CoreV1().Pods(withTestNamespace(ctx, t, clientset))

	pod, err := podClient.Create(ctx, &podTemplate, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating pod: %v", err)
	}

	podName := pod.Name

	// To simulate what infrastructure agent will be doing.
	t.Run("allows_mutated_pod_to_fetch_node_metrics_via_kubernetes_API_proxy", func(t *testing.T) {
		t.Parallel()

		for {
			pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting Pod %q: %v", podName, err)
			}

			if len(pod.Status.InitContainerStatuses) == 0 {
				t.Logf("pod has no init container status yet")

				time.Sleep(1 * time.Second)

				continue
			}

			state := pod.Status.InitContainerStatuses[0].State.Terminated
			if state == nil {
				t.Logf("container has not terminated yet %v", pod.Status.InitContainerStatuses[0].State)

				time.Sleep(1 * time.Second)

				continue
			}

			if state.ExitCode != 0 {
				t.Fatalf("test container exited with code %d", state.ExitCode)
			}

			break
		}
	})

	t.Run("adds_infrastructure_agent_sidecar_container_to_pod", func(t *testing.T) {
		t.Parallel()

		found := false

		if len(pod.Spec.Containers) == 1 {
			t.Fatalf("no sidecar container injected to pod")
		}

		expectedName := "newrelic-infrastructure"

		for _, c := range pod.Spec.Containers {
			if strings.Contains(c.Name, expectedName) {
				found = true
			}
		}

		if !found {
			t.Fatalf("no container with name including %q found in injected pod", expectedName)
		}

		deadline, ok := t.Deadline()
		if !ok {
			deadline = time.Now().Add(time.Duration(1<<63 - 1))
		}

		if err := wait.PollImmediate(1*time.Second, time.Until(deadline), func() (done bool, err error) {
			pod, err := podClient.Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("getting Pod %q: %v", podName, err)
			}

			if len(pod.Status.ContainerStatuses) == 0 {
				return false, nil
			}

			for _, cs := range pod.Status.ContainerStatuses {
				if !cs.Ready {
					return false, nil
				}
			}

			return true, nil
		}); err != nil {
			t.Fatalf("waiting for Pod to converge: %v", err)
		}
	})

	t.Run("handles_parallel_Pod_creation_in_multiple_namespaces_without_errors", func(t *testing.T) {
		t.Parallel()

		podTemplate := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testPrefix,
				Labels: map[string]string{
					"no-resources": "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "nginx",
						Image:   "nginx",
						Command: []string{"tail", "-f", "/dev/null"},
					},
				},
			},
		}

		parallelizm := 5

		errs := make(chan error, parallelizm)

		for i := 1; i <= parallelizm; i++ {
			go func() {
				podClient := clientset.CoreV1().Pods(withTestNamespace(ctx, t, clientset))

				_, err := podClient.Create(ctx, &podTemplate, metav1.CreateOptions{})
				errs <- err
			}()
		}

		for i := 1; i <= parallelizm; i++ {
			err := <-errs
			if err != nil {
				t.Fatalf("Pod #%d creation failed: %v", i, err)
			}
		}
	})

	t.Run("handles_parallel_Pod_creation_in_the_same_namespace_without_errors", func(t *testing.T) {
		t.Parallel()

		podTemplate := corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: testPrefix,
				Labels: map[string]string{
					"no-resources": "true",
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:    "nginx",
						Image:   "nginx",
						Command: []string{"tail", "-f", "/dev/null"},
					},
				},
			},
		}

		podClient := clientset.CoreV1().Pods(withTestNamespace(ctx, t, clientset))

		parallelizm := 5

		errs := make(chan error, parallelizm)

		for i := 1; i <= parallelizm; i++ {
			go func() {
				_, err := podClient.Create(ctx, &podTemplate, metav1.CreateOptions{})
				errs <- err
			}()
		}

		for i := 1; i <= parallelizm; i++ {
			err := <-errs
			if err != nil {
				t.Fatalf("Pod #%d creation failed: %v", i, err)
			}
		}
	})
}

func testClientset(t *testing.T) *kubernetes.Clientset {
	t.Helper()

	testEnv := &envtest.Environment{
		// For e2e tests, we use envtest.Environment for consistency with integration tests,
		// but we force them to use existing cluster instead of creating local controlplane,
		// as cluster we test on must have created resources defined in the operator Helm chart.
		//
		// This also allows us to test if the Helm chart configuration is correct (e.g. RBAC rules).
		//
		// With that approach, e2e tests can also be executed against cluster with 'make tilt-up' running.
		//
		// In the future, we may support also optionally creating Helm chart release on behalf of the user.
		UseExistingCluster: pointer.BoolPtr(true),
	}

	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatalf("starting test environment: %v", err)
	}

	t.Cleanup(func() {
		if err := testEnv.Stop(); err != nil {
			t.Logf("stopping test environment: %v", err)
		}
	})

	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("creating clientset: %v", err)
	}

	return clientSet
}

func withTestNamespace(ctx context.Context, t *testing.T, clientset *kubernetes.Clientset) string {
	t.Helper()

	namespaceTemplate := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: testPrefix,
		},
	}

	namespaceClient := clientset.CoreV1().Namespaces()

	ns, err := namespaceClient.Create(ctx, &namespaceTemplate, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("creating namespace: %v", err)
	}

	namespaceName := ns.Name

	t.Cleanup(func() {
		if err := namespaceClient.Delete(ctx, namespaceName, metav1.DeleteOptions{}); err != nil {
			t.Logf("deleting test namespace %q: %v", ns.Name, err)
		}
	})

	return namespaceName
}

func randomNodeName(ctx context.Context, t *testing.T, clientset *kubernetes.Clientset) string {
	t.Helper()

	rand.Seed(time.Now().Unix())

	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("listing nodes: %v", err)
	}

	if len(nodes.Items) == 0 {
		t.Fatalf("no Node objects found")
	}

	return nodes.Items[rand.Intn(len(nodes.Items))].Name
}
