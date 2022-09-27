// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kubeletCMName = "newrelic-kubelet-scraper-config"
)

// ensureConfigMapExistence assures that the kubelet configMap exists and it is well configured, otherwise patches the
// existing object or create a new one.
func (i *injector) ensureConfigMapExistence(ctx context.Context, namespace string) error {
	cm := &corev1.ConfigMap{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      kubeletCMName,
	}

	err := i.noCacheClient.Get(ctx, key, cm)

	if apierrors.IsNotFound(err) {
		return i.createConfigMap(ctx, namespace)
	}

	if err != nil {
		return fmt.Errorf("getting cm in the cluster %s/%s: %w", namespace, kubeletCMName, err)
	}

	if !reflect.DeepEqual(cm.BinaryData, i.kubeletConfig) {
		return i.updateConfigMap(ctx, cm)
	}

	return nil
}

func (i *injector) createConfigMap(ctx context.Context, namespace string) error {
	s := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      i.licenseSecretName,
			Namespace: namespace,
			Labels: map[string]string{
				OperatorCreatedLabel: OperatorCreatedLabelValue,
			},
		},
		Data: map[string][]byte{
			LicenseSecretKey: i.license,
		},
		Type: corev1.SecretTypeOpaque,
	}

	if err := i.noCacheClient.Create(ctx, s, &client.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating secret %s/%s: %w", s.Namespace, s.Name, err)
	}

	return nil
}

func (i *injector) updateConfigMap(ctx context.Context, s *corev1.ConfigMap) error {
	// When we update we should not add the label since likely the user or a different newrelic installation created
	// such secret.
	s.BinaryData = i.kubeletConfig
	if err := i.noCacheClient.Update(ctx, s, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating secret: %w", err)
	}

	return nil
}
