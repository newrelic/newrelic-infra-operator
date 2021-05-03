package agent

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	v1rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const defaultServiceAccount = "default"

// ClusterRoleBindingController struct holding the configuration.
type ClusterRoleBindingController struct {
	clusterRoleBindingName string
	logger                 *logrus.Logger
	client                 client.Client
}

// NewClusterRoleBindingController is the constructor for ClusterRoleBindingController struct.
func NewClusterRoleBindingController(
	client client.Client,
	clusterRoleBindingName string,
	logger *logrus.Logger) *ClusterRoleBindingController {
	ssc := &ClusterRoleBindingController{
		clusterRoleBindingName: clusterRoleBindingName,
		logger:                 logger,
		client:                 client,
	}

	return ssc
}

// EnsureSubject ensures that the clusterRolebinding exists and it is well configured, otherwise
// patches the existing object.
func (se *ClusterRoleBindingController) EnsureSubject(
	ctx context.Context,
	serviceAccountName string,
	serviceAccountNamespace string) error {
	crb := &v1rbac.ClusterRoleBinding{}
	key := client.ObjectKey{
		Name: se.clusterRoleBindingName,
	}

	err := se.client.Get(ctx, key, crb)
	if err != nil {
		return fmt.Errorf("getting clusterRoleBindings %q : %w", se.clusterRoleBindingName, err)
	}

	if hasSubject(crb, serviceAccountName, serviceAccountNamespace) {
		return nil
	}

	if serviceAccountName == "" {
		serviceAccountName = defaultServiceAccount
	}

	return se.updateClusterRoleBinding(ctx, crb, serviceAccountName, serviceAccountNamespace)
}

func hasSubject(crb *v1rbac.ClusterRoleBinding, namespace string, serviceAccountName string) bool {
	for _, s := range crb.Subjects {
		if s.Name == serviceAccountName && s.Namespace == namespace && s.Kind == v1rbac.ServiceAccountKind {
			return true
		}
	}

	return false
}

func (se *ClusterRoleBindingController) updateClusterRoleBinding(
	ctx context.Context,
	crb *v1rbac.ClusterRoleBinding,
	serviceAccountName string,
	serviceAccountNamespace string) error {
	crb.Subjects = append(crb.Subjects, v1rbac.Subject{
		Kind:      v1rbac.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: serviceAccountNamespace,
	})

	if err := se.client.Update(ctx, crb, &client.UpdateOptions{}); err != nil {
		return fmt.Errorf("updating ClusteRroleBinding: %w", err)
	}

	return nil
}
