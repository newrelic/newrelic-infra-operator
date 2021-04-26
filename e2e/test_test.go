// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +build e2e

package e2e_test

import (
	"context"
	"os"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
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
