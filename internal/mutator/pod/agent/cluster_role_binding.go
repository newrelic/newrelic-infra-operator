// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultServiceAccount = "default"
)

// ensureClusterRoleBindingSubject ensures that the clusterRolebinding exists and it is well configured, otherwise
// patches the existing object.
func (i *injector) ensureClusterRoleBindingSubject(
	ctx context.Context,
	serviceAccountName string,
	serviceAccountNamespace string,
) error {
	crb := &rbacv1.ClusterRoleBinding{}
	key := client.ObjectKey{
		Name: i.clusterRoleBindingName,
	}

	if err := i.client.Get(ctx, key, crb); err != nil {
		return fmt.Errorf("getting ClusterRoleBinding %q: %w", i.clusterRoleBindingName, err)
	}

	if serviceAccountName == "" {
		serviceAccountName = defaultServiceAccount
	}

	if hasSubject(crb, serviceAccountName, serviceAccountNamespace) {
		return nil
	}

	return i.updateClusterRoleBinding(ctx, crb, serviceAccountName, serviceAccountNamespace)
}

func (i *injector) updateClusterRoleBinding(
	ctx context.Context,
	crb *rbacv1.ClusterRoleBinding,
	serviceAccountName string,
	serviceAccountNamespace string,
) error {
	crb.Subjects = append(crb.Subjects, rbacv1.Subject{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: serviceAccountNamespace,
	})

	if err := i.client.Update(ctx, crb, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating ClusterRoleBinding: %w", err)
	}

	return nil
}

func hasSubject(crb *rbacv1.ClusterRoleBinding, serviceAccountName string, namespace string) bool {
	for _, s := range crb.Subjects {
		if s.Name == serviceAccountName && s.Namespace == namespace && s.Kind == rbacv1.ServiceAccountKind {
			return true
		}
	}

	return false
}
