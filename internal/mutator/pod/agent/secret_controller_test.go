package agent_test

import (
	"bytes"
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
)

func Test_Secret_Controller(t *testing.T) {
	t.Parallel()

	t.Run("exits_due_to_missing_secretName", func(t *testing.T) {
		t.Parallel()

		sc := agent.NewLicenseController(fake.NewClientBuilder().Build(), "", "", "", nil, logrus.New())

		err := sc.AssureExistence(
			context.Background(),
			"not-existing")
		if err == nil {
			t.Fatalf("The secret schema is wrong, the call should fail : %v", err)
		}
		if !apierrors.IsInvalid(err) {
			t.Fatalf("The expected error is 'bad request' : %v", err)
		}
	})
	t.Run("succeed", func(t *testing.T) {
		t.Parallel()
		c := fake.NewClientBuilder().Build()

		sc := agent.NewLicenseController(c, "name", "license", "dev", nil, logrus.New())

		err := sc.AssureExistence(
			context.Background(),
			"namespace")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %s", err.Error())
		}

		secret := &v1.Secret{}
		key := client.ObjectKey{
			Name:      "name",
			Namespace: "namespace",
		}
		err = c.Get(context.Background(), key, secret)
		if err != nil {
			t.Fatalf("Expecting the secret to be retrieved: %s", err.Error())
		}
		if secret.Name != "name" {
			t.Fatalf("Expecting different secret name, %v", secret.Name)
		}
		if !bytes.Equal(secret.Data["license"], []byte("dev")) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}
	})
}
