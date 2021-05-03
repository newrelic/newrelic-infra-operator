// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +build e2e

package e2e_test

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	appv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
)

// This test should cover the following functionality:
// - Pod gets created after mutating webhook is installed. This verifies:
//   - TLS certificate generation.
// - Pod with exclude mark (label/annotation?) do not get agent injected.
// - Pod without exclude mark get agent sidecar container injected. This verifies:
//   - Operator has ability to grant required permission to pods?
func Test_Example_test(t *testing.T) {
	t.Parallel()

	// use the current context in kubeconfig
	config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		t.Fatalf("building config from kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("preparing clientset: %v", err)
	}

	pods, err := clientset.CoreV1().Pods("").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("setting list of pods: %v", err)
	}

	for _, pod := range pods.Items {
		t.Logf("pod name: %q", pod.Name)
	}
}

//nolint:funlen,cyclop
func Test_roleBinding_Update(t *testing.T) {
	t.Parallel()

	t.Run("succeed", func(t *testing.T) {
		t.Parallel()
		// use the current context in kubeconfig

		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			t.Fatalf("building config from kubeconfig: %v", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			t.Fatalf("preparing clientset: %v", err)
		}

		// Crating disposable namespace
		namespaceName := RandStringRunes(8)
		namespace := v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		_, err = clientset.CoreV1().Namespaces().Create(context.Background(), &namespace, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating namespace: %v", err)
		}
		//nolint:errcheck
		defer clientset.CoreV1().Namespaces().Delete(context.Background(), namespaceName, metav1.DeleteOptions{})

		serviceAccountName := RandStringRunes(8)

		_, err = clientset.CoreV1().ServiceAccounts(namespaceName).Create(context.Background(),
			&v1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: serviceAccountName}}, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating service account: %v", err)
		}

		// Pod to be created
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testpod",
				Namespace: namespaceName,
				Labels: map[string]string{
					"eks.amazonaws.com/fargate-profile": "testprofile",
				},
			},
			Spec: v1.PodSpec{
				ServiceAccountName: serviceAccountName,
				Containers: []v1.Container{
					{
						Name:  "test-nginx",
						Image: "nginx",
					},
				},
			},
		}
		_, err = clientset.CoreV1().Pods(namespaceName).Create(context.Background(), &pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating pod: %v", err)
		}

		crb, err := clientset.RbacV1().ClusterRoleBindings().
			Get(context.Background(), "newrelic-infra-operator-infra-agent", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("getting crb: %v", err)
		}
		found := false
		for _, s := range crb.Subjects {
			if s.Name == serviceAccountName && s.Namespace == namespaceName {
				found = true
			}
		}
		if !found {
			t.Fatalf("crb does not contain the pod service account")
		}
	})
}

//nolint:funlen
func Test_injection_pod(t *testing.T) {
	t.Parallel()

	t.Run("succeed", func(t *testing.T) {
		t.Parallel()
		// use the current context in kubeconfig

		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			t.Fatalf("building config from kubeconfig: %v", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			t.Fatalf("preparing clientset: %v", err)
		}

		// Crating disposable namespace
		namespaceName := RandStringRunes(8)
		namespace := v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		_, err = clientset.CoreV1().Namespaces().Create(context.Background(), &namespace, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating namespace: %v", err)
		}
		//nolint:errcheck
		defer clientset.CoreV1().Namespaces().Delete(context.Background(), namespaceName, metav1.DeleteOptions{})

		// Pod to be created
		pod := v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "testpod",
				Namespace: namespaceName,
				Labels: map[string]string{
					"eks.amazonaws.com/fargate-profile": "testprofile",
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test-nginx",
						Image: "nginx",
					},
				},
			},
		}
		_, err = clientset.CoreV1().Pods(namespaceName).Create(context.Background(), &pod, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating pod: %v", err)
		}

		pCreated, err := clientset.CoreV1().Pods(namespaceName).Get(context.TODO(), "testpod", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("getting pod: %v", err)
		}
		if len(pCreated.Spec.Containers) != 2 {
			t.Fatalf("expecting 2 containers: %v", err)
		}
		if len(pCreated.Spec.Volumes) != 5 {
			t.Fatalf("expecting 4 volumes: %v", err)
		}
		if _, ok := pCreated.ObjectMeta.Labels["newrelic/agent-injected"]; !ok {
			t.Fatalf("expecting newrelic/agent-injected label to be injected")
		}
	})
}

//nolint:funlen
func Test_injection_Deployment(t *testing.T) {
	t.Parallel()

	t.Run("succeed", func(t *testing.T) {
		t.Parallel()
		// use the current context in kubeconfig

		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		if err != nil {
			t.Fatalf("building config from kubeconfig: %v", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			t.Fatalf("preparing clientset: %v", err)
		}

		// Crating disposable namespace
		namespaceName := RandStringRunes(8)
		namespace := v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}}
		_, err = clientset.CoreV1().Namespaces().Create(context.Background(), &namespace, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating namespace: %v", err)
		}
		//nolint:errcheck
		defer clientset.CoreV1().Namespaces().Delete(context.Background(), namespaceName, metav1.DeleteOptions{})

		// Deploy to be created
		deploy := appv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deploy",
				Namespace: namespaceName,
				Labels: map[string]string{
					"app": "nginx",
				},
			},
			Spec: appv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(2),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "nginx",
					},
				},
				Template: v1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "nginx",
						},
					},
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name:  "test-nginx",
								Image: "nginx",
							},
						},
					},
				},
			},
		}
		_, err = clientset.AppsV1().Deployments(namespaceName).Create(context.Background(), &deploy, metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("creating deployment: %v", err)
		}

		pList, err := clientset.CoreV1().Pods(namespaceName).List(context.Background(), metav1.ListOptions{})
		if err != nil {
			t.Fatalf("getting list of pods: %v", err)
		}
		for _, p := range pList.Items {
			if len(p.Spec.Containers) != 2 {
				t.Fatalf("expecting 2 containers: %v", err)
			}
			if len(p.Spec.Volumes) != 5 {
				t.Fatalf("expecting 4 volumes: %v", err)
			}
			if _, ok := p.ObjectMeta.Labels["newrelic/agent-injected"]; !ok {
				t.Fatalf("expecting newrelic/agent-injected label to be injected")
			}
		}
	})
}

func RandStringRunes(n int) string {
	letterRunes := []rune("abcdefghijklmnopqrstuvwxyz")

	rand.Seed(time.Now().UnixNano())

	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}

	return string(b)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}
