// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

// +build integration

package operator_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/newrelic/newrelic-infra-operator/internal/operator"
)

const (
	certValidityDuration = 1 * time.Hour
	tempPrefix           = "newrelic-infra-operator-tests"
	kubeconfigEnv        = "KUBECONFIG"
	testHost             = "127.0.0.1"
)

//nolint:funlen,gocognit,cyclop,gocyclo
func Test_Running_operator(t *testing.T) {
	t.Parallel()

	t.Run("exits_gracefully_when_given_context_is_cancelled", func(t *testing.T) {
		t.Parallel()

		ch := make(chan error)

		ctxWithDeadline := contextWithDeadline(t)

		ctx, cancel := context.WithTimeout(ctxWithDeadline, 1*time.Second)

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

		select {
		case <-ch:
			if err != nil {
				t.Fatalf("Unexpected error from running operator: %v", err)
			}
		case <-ctxWithDeadline.Done():
			t.Fatalf("Timed out waiting for operator to shutdown: %v", err)
		}
	})

	t.Run("listens_on_default_port_for_health_checks", func(t *testing.T) {
		t.Parallel()

		ctx, _, _ := runOperator(t, func(o *operator.Options) { o.HealthProbeBindAddress = "" })

		url := fmt.Sprintf("http://%s%s/readyz", testHost, operator.DefaultHealthProbeBindAddress)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			t.Fatalf("creating request: %v", err)
		}

		retryUntilFinished(func() bool {
			resp, err := http.DefaultClient.Do(req) //nolint:bodyclose

			defer closeResponseBody(t, resp)

			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
					t.Fatalf("test timed out: %v", err)
				}

				t.Logf("fetching readiness probe: %v", err)

				time.Sleep(1 * time.Second)

				return false
			}

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("got %q response code, expected %q: %v", resp.StatusCode, http.StatusOK, resp)
			}

			return true
		})
	})

	t.Run("responds_to", func(t *testing.T) {
		ctx, options, ca := runOperator(t, nil)

		t.Run("readiness_probe", func(t *testing.T) {
			t.Parallel()

			url := fmt.Sprintf("http://%s/readyz", options.HealthProbeBindAddress)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}

			retryUntilFinished(func() bool {
				resp, err := http.DefaultClient.Do(req) //nolint:bodyclose

				defer closeResponseBody(t, resp)

				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						t.Fatalf("test timed out: %v", err)
					}

					t.Logf("fetching readiness probe: %v", err)

					time.Sleep(1 * time.Second)

					return false
				}

				if resp.StatusCode != 200 {
					t.Fatalf("got non 200 response code: %v", resp)
				}

				return true
			})
		})

		t.Run("liveness_probe", func(t *testing.T) {
			t.Parallel()

			url := fmt.Sprintf("http://%s/healthz", options.HealthProbeBindAddress)
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}

			retryUntilFinished(func() bool {
				resp, err := http.DefaultClient.Do(req) //nolint:bodyclose

				defer closeResponseBody(t, resp)

				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						t.Fatalf("test timed out: %v", err)
					}

					t.Logf("fetching readiness probe: %v", err)

					time.Sleep(1 * time.Second)

					return false
				}

				if resp.StatusCode != 200 {
					t.Fatalf("got non 200 response code: %v", resp)
				}

				return true
			})
		})

		t.Run("pod_mutation_request", func(t *testing.T) {
			t.Parallel()

			admissionReq := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "foo",
        "namespace": "default"
    },
    "spec": {
        "containers": [
            {
                "image": "bar:v2",
                "name": "bar"
            }
        ]
    }
}`),
					},
				},
			}

			reqBytes, err := json.Marshal(admissionReq)
			if err != nil {
				t.Fatalf("encoding admission request as JSON: %v", err)
			}

			url := fmt.Sprintf("https://%s:%d%s", testHost, options.Port, operator.PodMutateEndpoint)

			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBytes))
			if err != nil {
				t.Fatalf("creating request: %v", err)
			}

			req.Header = http.Header{"Content-Type": []string{"application/json"}}

			pool := x509.NewCertPool()

			if ok := pool.AppendCertsFromPEM(ca); !ok {
				t.Fatalf("adding CA certificate to pool")
			}

			client := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: pool,
					},
				},
			}

			retryUntilFinished(func() bool {
				resp, err := client.Do(req) //nolint:bodyclose

				defer closeResponseBody(t, resp)

				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						t.Fatalf("test timed out: %v", err)
					}

					t.Logf("patching pod: %v", err)

					time.Sleep(1 * time.Second)

					return false
				}

				if resp.StatusCode != 200 {
					t.Fatalf("got non 200 response code: %v", resp)
				}

				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Fatalf("reading error response body: %v", err)
				}

				response := &admissionv1.AdmissionReview{}

				if err := json.Unmarshal(bodyBytes, response); err != nil {
					t.Fatalf("decoding admission response: %v", err)
				}

				if result := response.Response.Result; result != nil && result.Code != 200 {
					t.Fatalf("got bad response with code %d: %v", result.Code, result.Message)
				}

				return true
			})
		})
	})

	// We may touch environment variables in those tests, which are global, so run serially.
	//
	//nolint:paralleltest
	t.Run("fails_when", func(t *testing.T) {
		t.Run("there_is_no_kubernetes_credentials_available", func(t *testing.T) {
			ctx := context.Background()

			kubeconfig := os.Getenv(kubeconfigEnv)

			if err := os.Setenv(kubeconfigEnv, "foo"); err != nil {
				t.Fatalf("setting environment variable: %v", err)
			}

			t.Cleanup(func() {
				if err := os.Setenv(kubeconfigEnv, kubeconfig); err != nil {
					t.Logf("Resetting environment variable: %v", err)
				}
			})

			options := operator.Options{
				CertDir: dirWithCerts(t),
			}

			if err := operator.Run(ctx, options); err == nil {
				t.Fatalf("Expected operator to return error")
			}
		})

		t.Run("manager_configuration_is_wrong", func(t *testing.T) {
			testEnv := &envtest.Environment{}

			cfg, err := testEnv.Start()
			if err != nil {
				t.Fatalf("starting test environment: %v", err)
			}

			t.Cleanup(func() {
				if err := testEnv.Stop(); err != nil {
					t.Logf("stopping test environment: %v", err)
				}
			})

			options := operator.Options{
				RestConfig:             cfg,
				CertDir:                dirWithCerts(t),
				HealthProbeBindAddress: "1111",
			}

			if err := operator.Run(contextWithDeadline(t), options); err == nil {
				t.Fatalf("Expected operator to return error")
			}
		})
	})
}

func runOperator(t *testing.T, mutateOptions func(*operator.Options)) (context.Context, operator.Options, []byte) {
	t.Helper()

	ctxWithDeadline := contextWithDeadline(t)
	ctx, cancel := context.WithCancel(ctxWithDeadline)

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

	certDir, ca := dirWithCertsAndCA(t)

	options := operator.Options{
		RestConfig:             cfg,
		CertDir:                certDir,
		HealthProbeBindAddress: fmt.Sprintf("%s:%d", testHost, randomUnprivilegedPort(t)),
		Port:                   randomUnprivilegedPort(t),
	}

	if mutateOptions != nil {
		mutateOptions(&options)
	}

	go func() {
		if err := operator.Run(ctx, options); err != nil {
			fmt.Printf("running operator: %v\n", err) //nolint:forbidigo
			t.Fail()
		}
	}()

	return ctx, options, ca
}

func retryUntilFinished(f func() bool) {
	for {
		if f() {
			break
		}
	}
}

func closeResponseBody(t *testing.T, resp *http.Response) {
	t.Helper()

	if resp == nil || resp.Body == nil {
		return
	}

	if err := resp.Body.Close(); err != nil {
		t.Logf("closing response body: %v", err)
	}
}

func randomUnprivilegedPort(t *testing.T) int {
	t.Helper()

	min := 1024
	max := 65535

	i, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		t.Fatalf("generating random port: %v", err)
	}

	return int(i.Int64()) + min
}

func contextWithDeadline(t *testing.T) context.Context {
	t.Helper()

	deadline, ok := t.Deadline()
	if !ok {
		return context.Background()
	}

	// Arbitrary amount of time to let tests exit cleanly before main process terminates.
	timeoutGracePeriod := 10 * time.Second

	ctx, cancel := context.WithDeadline(context.Background(), deadline.Truncate(timeoutGracePeriod))

	t.Cleanup(cancel)

	return ctx
}

func dirWithCerts(t *testing.T) string {
	t.Helper()

	dir, _ := dirWithCertsAndCA(t)

	return dir
}

//nolint:funlen,cyclop
func dirWithCertsAndCA(t *testing.T) (string, []byte) {
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
		IsCA:                  true,
		IPAddresses:           []net.IP{net.ParseIP(testHost)},
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

	return dir, cert.Bytes()
}
