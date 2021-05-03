package agent_test

import (
	"context"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
)

//nolint:funlen
func Test_RoleBinding(t *testing.T) {
	t.Parallel()

	t.Run("exits_due_to_missing_role_binging", func(t *testing.T) {
		t.Parallel()

		i, err := agent.New(&agent.Config{
			Logger:                 logrus.New(),
			Client:                 fake.NewClientBuilder().Build(),
			ClusterRoleBindingName: agent.GetRBACName("fake-release"),
		})
		if err != nil {
			t.Fatalf("No error expected creating injector : %v", err)
		}

		err = i.EnsureSubject(
			context.Background(),
			"not-existing",
			"not-existing")
		if err == nil {
			t.Fatalf("The role does not exist, the call should fail")
		}
		if !apierrors.IsNotFound(err) {
			t.Fatalf("The expected error is 'resource not found' : %v", err)
		}
	})
	t.Run("updates_subjects", func(t *testing.T) {
		t.Parallel()

		clrb := &v1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: agent.GetRBACName("fake-release"),
			},
			Subjects: []v1.Subject{},
			RoleRef:  v1.RoleRef{},
		}
		c := fake.NewClientBuilder().WithObjects(clrb).Build()
		i, err := agent.New(&agent.Config{
			Logger:                 logrus.New(),
			Client:                 c,
			ClusterRoleBindingName: agent.GetRBACName("fake-release"),
		})
		if err != nil {
			t.Fatalf("No error expected creating injector : %v", err)
		}
		err = i.EnsureSubject(
			context.Background(),
			"sa-name",
			"sa-namespace")
		if err != nil {
			t.Fatalf("The role exists, and the call should not fail: %s", err.Error())
		}

		crb := &v1.ClusterRoleBinding{}
		key := client.ObjectKey{
			Name: agent.GetRBACName("fake-release"),
		}

		err = c.Get(context.Background(), key, crb)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}
		if len(crb.Subjects) != 1 {
			t.Fatalf("Expecting only one subject: %d", len(crb.Subjects))
		}
		if crb.Subjects[0].Name != "sa-name" {
			t.Fatalf("Expecting different sa name, %v", crb.Subjects[0].Name)
		}
		if crb.Subjects[0].Namespace != "sa-namespace" {
			t.Fatalf("Expecting different sa namespace, %v", crb.Subjects[0].Namespace)
		}
	})
}
