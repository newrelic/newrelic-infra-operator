// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

//nolint:funlen,cyclop
func Test_Pod_mutator_handle(t *testing.T) {
	t.Parallel()

	ctx := testutil.ContextWithDeadline(t)

	t.Run("returns_no_patches_when_no_mutation_occurs", func(t *testing.T) {
		t.Parallel()

		resp := newHandler(t).Handle(ctx, testRequest())

		if resp.Result != nil {
			if resp.Result.Code != http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		}

		if len(resp.Patches) != 0 {
			t.Fatalf("unexpected patches received: %v", resp)
		}
	})

	t.Run("returns_patch_when_mutation_occurs", func(t *testing.T) {
		t.Parallel()

		handler := newHandler(t)

		handler.mutators = []podMutator{
			&mockMutator{
				mutateF: func(_ context.Context, pod *corev1.Pod, _ webhook.RequestOptions) error {
					pod.Labels = map[string]string{"foo": "bar"}

					return nil
				},
			},
		}

		resp := handler.Handle(ctx, testRequest())

		if resp.Result != nil {
			if resp.Result.Code != http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		}

		if len(resp.Patches) == 0 {
			t.Fatalf("expected at least one patch, got %v", resp)
		}
	})

	t.Run("returns_empty_patch_when_mutation_error_occurs_and_ignoring_errors_is_enabled", func(t *testing.T) {
		t.Parallel()

		handler := newHandler(t)
		handler.ignoreMutationErrors = true

		handler.mutators = []podMutator{
			&mockMutator{
				mutateF: func(_ context.Context, pod *corev1.Pod, _ webhook.RequestOptions) error {
					return fmt.Errorf("test error")
				},
			},
		}

		resp := handler.Handle(ctx, testRequest())

		if resp.Result != nil {
			if resp.Result.Code != http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		}

		if len(resp.Patches) != 0 {
			t.Fatalf("unexpected patches received: %v", resp)
		}
	})

	t.Run("returns_error_when", func(t *testing.T) {
		t.Parallel()

		t.Run("request_subject_is_malformed", func(t *testing.T) {
			t.Parallel()

			admissionReq := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Object: runtime.RawExtension{
						Raw: []byte(`{`),
					},
				},
			}

			resp := newHandler(t).Handle(ctx, admissionReq)
			if resp.Result.Code == http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		})

		t.Run("one_of_the_mutators_returns_error", func(t *testing.T) {
			handler := newHandler(t)

			handler.mutators = []podMutator{
				&mockMutator{
					mutateF: func(_ context.Context, _ *corev1.Pod, _ webhook.RequestOptions) error {
						return nil
					},
				},
				&mockMutator{
					mutateF: func(_ context.Context, _ *corev1.Pod, _ webhook.RequestOptions) error {
						return fmt.Errorf("mutation failed")
					},
				},
			}

			resp := handler.Handle(ctx, testRequest())
			if resp.Result.Code == http.StatusOK || resp.Result.Code == http.StatusBadRequest {
				t.Logf("bad response: %v", resp)
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		})
	})
}

type mockMutator struct {
	mutateF func(ctx context.Context, pod *corev1.Pod, reqOptions webhook.RequestOptions) error
}

func (m *mockMutator) Mutate(ctx context.Context, pod *corev1.Pod, reqOptions webhook.RequestOptions) error {
	return m.mutateF(ctx, pod, reqOptions)
}

func testRequest() admission.Request {
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{
				Raw: []byte(`{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "foo",
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
}

func newHandler(t *testing.T) *podMutatorHandler {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("adding corev1 to scheme: %v", err)
	}

	d, err := admission.NewDecoder(scheme)
	if err != nil {
		t.Fatalf("creating decoder: %v", err)
	}

	return &podMutatorHandler{
		decoder: d,
		logger:  logrus.New(),
	}
}
