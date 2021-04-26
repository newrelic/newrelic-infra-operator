// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +build integration

package operator_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/newrelic/nri-k8s-operator/internal/operator"
)

const (
	certValidityDuration = 1 * time.Hour
	tempPrefix           = "nri-k8s-operator-tests"
)

func Test_Running_operator(t *testing.T) {
	t.Parallel()

	t.Run("exits_gracefully_when_given_context_is_cancelled", func(t *testing.T) {
		t.Parallel()

		ch := make(chan error)

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		testEnv := &envtest.Environment{}

		cfg, err := testEnv.Start()
		if err != nil {
			t.Fatalf("starting test environment: %v", err)
		}

		t.Cleanup(func() {
			cancel()

			if err := testEnv.Stop(); err != nil {
				t.Logf("stopping test environment: %v", err)
			}
		})

		options := operator.Options{
			RestConfig: cfg,
			CertDir:    dirWithCerts(t),
		}

		go func() {
			ch <- operator.Run(ctx, options)
		}()

		err = <-ch
		if err != nil {
			t.Fatalf("Unexpected error from running operator: %v", err)
		}
	})

	t.Run("fails_when", func(t *testing.T) {
		t.Parallel()

		t.Run("there_is_no_kubernetes_credentials_available", func(t *testing.T) {
			ctx := context.Background()

			if err := os.Setenv("KUBECONFIG", "foo"); err != nil {
				t.Fatalf("setting environment variable: %v", err)
			}

			options := operator.Options{
				CertDir: dirWithCerts(t),
			}

			if err := operator.Run(ctx, options); err == nil {
				t.Fatalf("Expected operator to return error")
			}
		})
	})
}

//nolint:funlen,cyclop
func dirWithCerts(t *testing.T) string {
	t.Helper()

	dir, err := ioutil.TempDir("", tempPrefix)
	if err != nil {
		t.Fatalf("creating temporary directory: %v", err)
	}

	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("removing temporary directory %q: %v", dir, err)
		}
	})

	// Generate RSA private key.
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating RSA key: %v", err)
	}

	// Generate serial number for X.509 certificate.
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		t.Fatalf("generating serial number: %v", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"example"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(certValidityDuration),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	// Create X.509 certificate in DER format.
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("creating X.509 certificate: %v", err)
	}

	// Encode X.509 certificate into PEM format.
	var cert bytes.Buffer
	if err := pem.Encode(&cert, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		t.Fatalf("encoding X.509 certificate into PEM format: %v", err)
	}

	// Convert RSA private key into PKCS8 DER format.
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		t.Fatalf("encoding RSA private key into PKCS8 DER format: %v", err)
	}

	// Convert private key from PKCS8 DER format to PEM format.
	var key bytes.Buffer
	if err := pem.Encode(&key, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("encoding RSA private key into PEM format: %v", err)
	}

	path := filepath.Join(dir, "tls.key")
	if err := ioutil.WriteFile(path, key.Bytes(), 0o600); err != nil {
		t.Fatalf("writing private key to %q: %v", path, err)
	}

	path = filepath.Join(dir, "tls.crt")
	if err := ioutil.WriteFile(path, cert.Bytes(), 0o600); err != nil {
		t.Fatalf("writing certificate to %q: %v", path, err)
	}

	return dir
}
