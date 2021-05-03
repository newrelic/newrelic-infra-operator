// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/newrelic/newrelic-infra-operator/internal/testutil"
)

//nolint:funlen
func Test_Pod_mutator_handle(t *testing.T) {
	t.Parallel()

	t.Run("returns_no_patches_when_no_mutation_occurs", func(t *testing.T) {
		t.Parallel()

		resp := newHandler(t).Handle(testutil.ContextWithDeadline(t), testRequest())

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
				mutateF: func(_ context.Context, _ client.Client, pod *corev1.Pod, _ string) error {
					pod.Labels = map[string]string{"foo": "bar"}

					return nil
				},
			},
		}

		resp := handler.Handle(testutil.ContextWithDeadline(t), testRequest())

		if resp.Result != nil {
			if resp.Result.Code != http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		}

		if len(resp.Patches) == 0 {
			t.Fatalf("expected at least one patch, got %v", resp)
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

			resp := newHandler(t).Handle(testutil.ContextWithDeadline(t), admissionReq)
			if resp.Result.Code == http.StatusOK {
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		})

		t.Run("one_of_the_mutators_returns_error", func(t *testing.T) {
			handler := newHandler(t)

			handler.mutators = []podMutator{
				&mockMutator{
					mutateF: func(_ context.Context, _ client.Client, _ *corev1.Pod, _ string) error {
						return nil
					},
				},
				&mockMutator{
					mutateF: func(_ context.Context, _ client.Client, _ *corev1.Pod, _ string) error {
						return fmt.Errorf("mutation failed")
					},
				},
			}

			resp := handler.Handle(testutil.ContextWithDeadline(t), testRequest())
			if resp.Result.Code == http.StatusOK || resp.Result.Code == http.StatusBadRequest {
				t.Logf("bad response: %v", resp)
				t.Fatalf("unexpected response code %d", resp.Result.Code)
			}
		})
	})
}

type mockMutator struct {
	mutateF func(ctx context.Context, client client.Client, pod *corev1.Pod, ns string) error
}

func (m *mockMutator) Mutate(ctx context.Context, client client.Client, pod *corev1.Pod, ns string) error {
	return m.mutateF(ctx, client, pod, ns)
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
	}
}
