// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"bytes"
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// OperatorCreatedLabel is the name of the label injected to the secrets created by the operator.
	OperatorCreatedLabel = "newrelic/infra-operator-created"
	// OperatorCreatedLabelValue is the value of the label injected to the secrets created by the operator.
	OperatorCreatedLabelValue = "true"
)

// ensureLicenseSecretExistence assures that the license secret exists and it is well configured, otherwise patches the
// existing object or create a new one.
func (i *injector) ensureLicenseSecretExistence(ctx context.Context, namespace string) error {
	s := &v1.Secret{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      i.licenseSecretName,
	}

	err := i.noCacheClient.Get(ctx, key, s)

	if apierrors.IsNotFound(err) {
		return i.createSecret(ctx, namespace)
	}

	if err != nil {
		return fmt.Errorf("getting secret in the cluster %s/%s: %w", namespace, i.licenseSecretName, err)
	}

	if value, ok := s.Data[LicenseSecretKey]; !ok || !bytes.Equal(value, i.license) {
		return i.updateSecret(ctx, s)
	}

	return nil
}

func (i *injector) createSecret(ctx context.Context, namespace string) error {
	s := &v1.Secret{
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
		Type: v1.SecretTypeOpaque,
	}

	if err := i.noCacheClient.Create(ctx, s, &client.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("creating secret %s/%s: %w", s.Namespace, s.Name, err)
	}

	return nil
}

func (i *injector) updateSecret(ctx context.Context, s *v1.Secret) error {
	// When we update we should not add the label since likely the user or a different newrelic installation created
	// such secret.
	s.Data[LicenseSecretKey] = i.license
	if err := i.noCacheClient.Update(ctx, s, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating secret: %w", err)
	}

	return nil
}
