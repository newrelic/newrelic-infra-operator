// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"testing"

	v1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
)

func Test_Compute_Hash(t *testing.T) {
	t.Parallel()

	c1 := v1.Container{
		Name: "test-hash",
		SecurityContext: &v1.SecurityContext{
			Privileged: nil,
		},
	}

	h1, err := computeHash(c1)
	if err != nil {
		t.Fatalf("computing h1 hash : %v", err)
	}

	c2 := v1.Container{
		Name: "test-hash",
		SecurityContext: &v1.SecurityContext{
			Privileged: pointer.BoolPtr(true),
		},
	}

	h2, err := computeHash(c2)
	if err != nil {
		t.Fatalf("computing h2 hash : %v", err)
	}

	if h1 == h2 {
		t.Fatalf("hash should be different")
	}
}
