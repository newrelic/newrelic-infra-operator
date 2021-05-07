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

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/pod/agent"
	"github.com/newrelic/newrelic-infra-operator/internal/operator"
	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

const (
	certValidityDuration = 1 * time.Hour
	kubeconfigEnv        = "KUBECONFIG"
	testHost             = "127.0.0.1"
	testPrefix           = "newrelic-infra-operator-test"
	testLicense          = "test-license"
	testClusterName      = "test-cluster"
)

//nolint:funlen,gocognit,cyclop,gocyclo
func Test_Running_operator(t *testing.T) {
	t.Parallel()

	t.Run("exits_gracefully_when_given_context_is_cancelled", func(t *testing.T) {
		t.Parallel()

		ch := make(chan error)

		ctxWithDeadline := testutil.ContextWithDeadline(t)

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

		options := testOptions(t, cfg, dirWithCerts(t))

		go func() {
			ch <- operator.Run(ctx, options)
		}()

		select {
		case err = <-ch:
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

		createClusterRoleBinding(ctx, t, options)

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

				if resp.StatusCode != http.StatusOK {
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

				if resp.StatusCode != http.StatusOK {
					t.Fatalf("got non 200 response code: %v", resp)
				}

				return true
			})
		})

		t.Run("pod_mutation_request", func(t *testing.T) {
			t.Parallel()

			admissionReq := admissionv1.AdmissionReview{
				Request: &admissionv1.AdmissionRequest{
					Namespace: "default",
					Object: runtime.RawExtension{
						Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "foo",
        "namespace": "default",
        "creationTimestamp": "2021-04-29T11:15:14Z"
    },
    "spec": {
        "containers": [
            {
                "image": "bar:v2",
                "name": "bar",
                "resources": {}
            }
        ]
    },
    "status": {}
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

			c := &http.Client{
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{
						RootCAs: pool,
					},
				},
			}

			retryUntilFinished(func() bool {
				resp, err := c.Do(req) //nolint:bodyclose

				defer closeResponseBody(t, resp)

				if err != nil {
					if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
						t.Fatalf("test timed out: %v", err)
					}

					t.Logf("patching pod: %v", err)

					time.Sleep(1 * time.Second)

					return false
				}

				if resp.StatusCode != http.StatusOK {
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

				if result := response.Response.Result; result != nil && result.Code != http.StatusOK {
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
			ctx := testutil.ContextWithDeadline(t)

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
				Logger:  logrus.New(),
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
				Logger:                 logrus.New(),
				RestConfig:             cfg,
				CertDir:                dirWithCerts(t),
				HealthProbeBindAddress: "1111",
			}

			if err := operator.Run(testutil.ContextWithDeadline(t), options); err == nil {
				t.Fatalf("Expected operator to return error")
			}
		})
	})
}

func runOperator(t *testing.T, mutateOptions func(*operator.Options)) (context.Context, operator.Options, []byte) {
	t.Helper()

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

	certDir, ca := dirWithCertsAndCA(t)

	options := testOptions(t, cfg, certDir)

	if mutateOptions != nil {
		mutateOptions(&options)
	}

	ctxWithDeadline := testutil.ContextWithDeadline(t)

	// Run operator briefly to verify it starts without errors.
	testCtx, cancel := context.WithDeadline(ctxWithDeadline, time.Now().Add(1*time.Second))

	t.Cleanup(cancel)

	if err := operator.Run(testCtx, options); err != nil {
		t.Fatalf("starting operator: %v", err)
	}

	ctx, cancel := context.WithCancel(ctxWithDeadline)

	t.Cleanup(cancel)

	go func() {
		if err := operator.Run(ctx, options); err != nil {
			fmt.Printf("running operator: %v\n", err) //nolint:forbidigo
			t.Fail()
		}
	}()

	return ctx, options, ca
}

func testOptions(t *testing.T, cfg *rest.Config, certDir string) operator.Options {
	t.Helper()

	return operator.Options{
		Logger:                 logrus.New(),
		RestConfig:             cfg,
		CertDir:                certDir,
		HealthProbeBindAddress: fmt.Sprintf("%s:%d", testHost, randomUnprivilegedPort(t)),
		MetricsBindAddress:     fmt.Sprintf("%s:%d", testHost, randomUnprivilegedPort(t)),
		Port:                   randomUnprivilegedPort(t),
		InfraAgentInjection: agent.InjectorConfig{
			ResourcePrefix: testPrefix,
			License:        testLicense,
			ClusterName:    testClusterName,
		},
	}
}

func createClusterRoleBinding(ctx context.Context, t *testing.T, options operator.Options) {
	t.Helper()

	c, err := client.New(options.RestConfig, client.Options{})
	if err != nil {
		t.Fatalf("initializing client: %v", err)
	}

	crb := v1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s%s", testPrefix, agent.ClusterRoleBindingSuffix),
		},
		RoleRef: v1.RoleRef{
			// Note that we are not interested into having the real role bound.
			Name: "view",
			Kind: "ClusterRole",
		},
	}

	// Making sure that clusterRoleBinding exists to run tests.
	if err := c.Create(ctx, &crb, &client.CreateOptions{}); err != nil {
		t.Fatalf("creating ClusterRoleBinding: %v", err)
	}

	t.Cleanup(func() {
		if err := c.Delete(ctx, &crb, &client.DeleteOptions{}); err != nil {
			t.Logf("removing ClusterRoleBinding: %v", err)
		}
	})
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

func dirWithCerts(t *testing.T) string {
	t.Helper()

	dir, _ := dirWithCertsAndCA(t)

	return dir
}

//nolint:funlen
func dirWithCertsAndCA(t *testing.T) (string, []byte) {
	t.Helper()

	dir := t.TempDir()

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
