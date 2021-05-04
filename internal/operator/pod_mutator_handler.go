// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/newrelic/newrelic-infra-operator/internal/mutator/webhook"
)

type podMutator interface {
	Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error
}

type podMutatorHandler struct {
	decoder  *admission.Decoder
	mutators []podMutator
}

// Handle is in charge of handling the request received involving new pods.
func (a *podMutatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	requestOptions := webhook.RequestOptions{
		Namespace: req.Namespace,
	}

	if req.DryRun != nil {
		requestOptions.DryRun = *req.DryRun
	}

	for _, m := range a.mutators {
		if err := m.Mutate(ctx, pod, requestOptions); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder injects the decoder and is useful to respect the DecoderInjector interface.
func (a *podMutatorHandler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d

	return nil
}
