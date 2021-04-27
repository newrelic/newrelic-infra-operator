package secret_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/newrelic/newrelic-infra-operator/internal/controller/secret"
)

func Test_RoleBinding_Controller(t *testing.T) {
	t.Parallel()

	t.Run("exits_due_to_missing_secretName", func(t *testing.T) {
		t.Parallel()

		sc := secret.NewLicenseController(fake.NewClientBuilder().Build(), "", "", "", nil, logrus.New())

		_, err := sc.AssureExistence(
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

		sc := secret.NewLicenseController(fake.NewClientBuilder().Build(), "test-secret", "license", "dev", nil, logrus.New())

		secret, err := sc.AssureExistence(
			context.Background(),
			"not-existing")
		if err != nil {
			t.Fatalf("The secret is well formatted, and the call should not fail: %s", err)
		}
		if secret.Name != "test-secret" {
			t.Fatalf("Expecting different secret name, %v", secret.Name)
		}
		if !bytes.Equal(secret.Data["license"], []byte("dev")) {
			t.Fatalf("Expecting different secret data, '%s'!='%s'", secret.Data["license"], []byte("dev"))
		}
	})
}
