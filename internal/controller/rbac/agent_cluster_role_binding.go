// Package rbac implements a simple controller for the clusterRoleBinding needed by the infra agent.
package rbac

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	v1rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

// AssureClusterRoleBindingExistence assures that the clusterRolebinding exists and it is well configured, otherwise
// patches the existing object.
func (se *ClusterRoleBindingController) AssureClusterRoleBindingExistence(
	ctx context.Context,
	serviceAccountName string,
	serviceAccountNamespace string) (*v1rbac.ClusterRoleBinding, error) {
	crb := &v1rbac.ClusterRoleBinding{}

	err := se.client.Get(ctx, client.ObjectKey{
		Name: se.clusterRoleBindingName,
	}, crb)
	if err != nil {
		return nil, fmt.Errorf("error while getting clusterRoleBindings %q : %w", se.clusterRoleBindingName, err)
	}

	if hasSubject(crb, serviceAccountName, serviceAccountNamespace) {
		return crb, nil
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
	serviceAccountNamespace string) (*v1rbac.ClusterRoleBinding, error) {
	crb.Subjects = append(crb.Subjects, v1rbac.Subject{
		Kind:      v1rbac.ServiceAccountKind,
		Name:      serviceAccountName,
		Namespace: serviceAccountNamespace,
	})

	if err := se.client.Update(ctx, crb, &client.UpdateOptions{}); err != nil {
		return nil, fmt.Errorf("updating clusterrolebinding: %w", err)
	}

	return crb, nil
}
