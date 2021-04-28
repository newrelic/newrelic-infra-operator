package operator

import (
	"testing"
)

func TestInterpretLabel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		label    string
		expected [2]bool
	}{
		{
			"",
			[2]bool{false, false},
		},
		{
			"false",
			[2]bool{false, true},
		},
		{
			"0",
			[2]bool{false, true},
		},
		{
			"1",
			[2]bool{true, false},
		},
		{
			"yes",
			[2]bool{true, false},
		},
		{
			"aaaaaaaaaaaa",
			[2]bool{true, false},
		},
		{
			"fargate",
			[2]bool{true, false},
		},
	}

	for _, c := range cases {
		r1, r2 := interpretLabel(c.label)
		if r1 != c.expected[0] || r2 != c.expected[1] {
			t.Fatalf(
				"expected (%v, %v) for label %s but got (%v, %v)",
				c.expected[0], c.expected[1], c.label, r1, r2,
			)
		}
	}
}
