// Package secret implements a simple controller for the license secret needed by the infra agent.
package secret

import (
	"bytes"
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LicenseController struct holding the configuration.
type LicenseController struct {
	client client.Client
	key    string
	labels map[string]string
	value  []byte
	name   string
	logger *logrus.Logger
}

// NewLicenseController is the constructor for LicenseController struct.
func NewLicenseController(
	client client.Client,
	name string,
	key string,
	value string,
	labels map[string]string,
	logger *logrus.Logger) *LicenseController {
	ssc := &LicenseController{
		client: client,
		key:    key,
		labels: labels,
		value:  []byte(value),
		name:   name,
		logger: logger,
	}

	return ssc
}

// AssureExistence assures that the license secret exists and it is well configured, otherwise patches the
// existing object or create a new one.
func (lc *LicenseController) AssureExistence(
	ctx context.Context,
	namespace string) (*v1.Secret, error) {
	s := &v1.Secret{}
	err := lc.client.Get(ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      lc.name,
	}, s)

	if apierrors.IsNotFound(err) {
		return lc.createSecret(ctx, namespace)
	} else if err != nil {
		return nil, fmt.Errorf("error while getting secret in the cluster %q/%q : %w", namespace, lc.name, err)
	}

	if value, ok := s.Data[lc.key]; !ok || !bytes.Equal(value, lc.value) {
		return lc.updateSecret(ctx, s)
	}

	return s, nil
}

func (lc *LicenseController) createSecret(ctx context.Context, namespace string) (*v1.Secret, error) {
	s := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      lc.name,
			Namespace: namespace,
			Labels:    lc.labels,
		},
		Data: map[string][]byte{
			lc.key: lc.value,
		},
		Type: v1.SecretTypeOpaque,
	}

	if err := lc.client.Create(ctx, s, &client.CreateOptions{}); err != nil {
		return nil, fmt.Errorf("creating secret %s/%s: %w", s.Namespace, s.Name, err)
	}

	return s, nil
}

func (lc *LicenseController) updateSecret(ctx context.Context, s *v1.Secret) (*v1.Secret, error) {
	s.Data[lc.key] = lc.value
	// we are currently ignoring possible differences in labels and secretType
	if err := lc.client.Update(ctx, s, &client.UpdateOptions{}); err != nil {
		return nil, fmt.Errorf("updating secret: %w", err)
	}

	return s, nil
}
