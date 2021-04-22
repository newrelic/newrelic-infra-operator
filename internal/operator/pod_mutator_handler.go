// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package operator

import (
	"context"
	"encoding/json"
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type podMutator interface {
	Mutate(ctx context.Context, client client.Client, pod *corev1.Pod, ns string) error
}

type podMutatorHandler struct {
	Client   client.Client
	decoder  *admission.Decoder
	mutators []podMutator
}

func (a *podMutatorHandler) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}

	err := a.decoder.Decode(req, pod)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	for _, m := range a.mutators {
		if err := m.Mutate(ctx, a.Client, pod, req.Namespace); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

func (a *podMutatorHandler) InjectDecoder(d *admission.Decoder) error {
	a.decoder = d

	return nil
}
