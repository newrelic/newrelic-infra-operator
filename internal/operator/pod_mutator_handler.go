// Copyright 2022 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/newrelic/newrelic-infra-operator/internal/webhook"
)

type podMutator interface {
	Mutate(ctx context.Context, pod *corev1.Pod, requestOptions webhook.RequestOptions) error
}

type podMutatorHandler struct {
	decoder              admission.Decoder
	mutators             []podMutator
	ignoreMutationErrors bool
	logger               *logrus.Logger
}

// Handle is in charge of handling the request received involving new pods.
func (a *podMutatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	if err := a.decoder.Decode(req, pod); err != nil {
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
			if a.ignoreMutationErrors {
				a.logger.Warnf("Pod %s/%s mutation failed: %v", pod.Name, req.Namespace, err)
				// Return the original unmodified pod without mutation
				return admission.PatchResponseFromRaw(req.Object.Raw, req.Object.Raw)
			}

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
func (a *podMutatorHandler) InjectDecoder(d admission.Decoder) error {
	a.decoder = d
	return nil
}
