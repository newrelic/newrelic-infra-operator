package agent

import (
	"context"
	"fmt"

	v1rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultServiceAccount = "default"

// EnsureSubject ensures that the clusterRolebinding exists and it is well configured, otherwise
// patches the existing object.
func (i *injector) EnsureSubject(
	ctx context.Context,
	serviceAccountName string,
	serviceAccountNamespace string) error {
	crb := &v1rbac.ClusterRoleBinding{}
	key := client.ObjectKey{
		Name: i.ClusterRoleBindingName,
	}

	err := i.Client.Get(ctx, key, crb)
	if err != nil {
		return fmt.Errorf("getting clusterRoleBindings %q : %w", i.ClusterRoleBindingName, err)
	}

	if hasSubject(crb, serviceAccountName, serviceAccountNamespace) {
		return nil
	}

	if serviceAccountName == "" {
		serviceAccountName = defaultServiceAccount
	}

	return i.updateClusterRoleBinding(ctx, crb, serviceAccountName, serviceAccountNamespace)
}

func (i *injector) updateClusterRoleBinding(
	ctx context.Context,
	crb *v1rbac.ClusterRoleBinding,
	serviceAccountName string,
	serviceAccountNamespace string) error {
	crb.Subjects = append(crb.Subjects, v1rbac.Subject{
		Kind:      v1rbac.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: serviceAccountNamespace,
	})

	if err := i.Client.Update(ctx, crb, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating ClusteRroleBinding: %w", err)
	}

	return nil
}

func hasSubject(crb *v1rbac.ClusterRoleBinding, namespace string, serviceAccountName string) bool {
	for _, s := range crb.Subjects {
		if s.Name == serviceAccountName && s.Namespace == namespace && s.Kind == v1rbac.ServiceAccountKind {
			return true
		}
	}

	return false
}
