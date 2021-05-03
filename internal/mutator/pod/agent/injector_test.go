//nolint:testpackage
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

	if h1 != "bfab307e438e91e8cea1f8e5425bda592231ff80b097c907b37da955901cf13b" {
		t.Fatalf("hash mutated: %s", h1)
	}
}
