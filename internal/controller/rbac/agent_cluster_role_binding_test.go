package rbac_test

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/controller/rbac"
)

func Test_RoleBinding_Controller(t *testing.T) {
	t.Parallel()

	logger := logrus.New()

	t.Run("exits_due_to_missing_role_binging", func(t *testing.T) {
		t.Parallel()

		rbc := rbac.NewClusterRoleBindingController(fake.NewClientBuilder().Build(), "", logger)

		_, err := rbc.AssureClusterRoleBindingExistence(
			context.Background(),
			"not-existing",
			"not-existing")
		if err == nil {
			t.Fatalf("The role does not exist, the call should fail : %v", err)
		}
		if !apierrors.IsNotFound(err) {
			t.Fatalf("The expected error is 'resource not found' : %v", err)
		}
	})
	t.Run("succeed", func(t *testing.T) {
		t.Parallel()

		clrb := &v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Subjects: []v1.Subject{},
			RoleRef:  v1.RoleRef{},
		}
		client := fake.NewClientBuilder().WithObjects(clrb).Build()
		rbc := rbac.NewClusterRoleBindingController(client, "test", logger)

		patchedclrb, err := rbc.AssureClusterRoleBindingExistence(
			context.Background(),
			"sa-name",
			"sa-namespace")
		if err != nil {
			t.Fatalf("The role exists, and the call should not fail: %s", err)
		}
		if len(patchedclrb.Subjects) != 1 {
			t.Fatalf("Expecting only one subject: %d", len(patchedclrb.Subjects))
		}
		if patchedclrb.Subjects[0].Name != "sa-name" {
			t.Fatalf("Expecting different sa name, %v", patchedclrb.Subjects[0].Name)
		}
		if patchedclrb.Subjects[0].Namespace != "sa-namespace" {
			t.Fatalf("Expecting different sa namespace, %v", patchedclrb.Subjects[0].Namespace)
		}
	})
}
