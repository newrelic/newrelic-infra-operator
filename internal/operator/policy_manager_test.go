package operator_test

import (
	"testing"

	"github.com/newrelic/newrelic-infra-operator/internal/operator"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// nolint:funlen
func TestPolicy_Match(t *testing.T) {
	t.Parallel()

	testPods := []v1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with fargate label",
				Labels: map[string]string{
					"eks.amazonaws.com/fargate-profile": "whatever",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with 'customLabel: true'",
				Labels: map[string]string{
					"customLabel": "true",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with 'customLabel: false'",
				Labels: map[string]string{
					"customLabel": "false",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with fargate and customLabel: false",
				Labels: map[string]string{
					"eks.amazonaws.com/fargate-profile": "whatever",
					"customLabel":                       "false",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with dummy labels",
				Labels: map[string]string{
					"foo": "true",
					"bar": "false",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with negative new relic label",
				Labels: map[string]string{
					"newrelic.com/inject": "false",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "Pod with positive new relic label",
				Labels: map[string]string{
					"newrelic.com/inject": "true",
				},
			},
		},
	}

	defaultPolicy := operator.Policy{
		Fargate: true,
	}
	podPolicyTable(t, &defaultPolicy, "defaultPolicy", testPods, []bool{
		true, false, false, true, false, false, true,
	})

	unlabeledButCustom := operator.Policy{
		Fargate:   true,
		Unlabeled: true,
		Labels:    []string{"customLabel"},
	}
	podPolicyTable(t, &unlabeledButCustom, "unlabeledButCustom", testPods, []bool{
		true, true, false, false, true, false, true,
	})

	customNoFargate := operator.Policy{
		Labels: []string{"customLabel"},
	}
	podPolicyTable(t, &customNoFargate, "customNoFargate", testPods, []bool{
		false, true, false, false, false, false, true,
	})
}

func podPolicyTable(t *testing.T, p *operator.Policy, policyName string, pods []v1.Pod, expected []bool) {
	t.Helper()

	if len(pods) != len(expected) {
		t.Fatalf("internal error: mismatched length of pods and expectations")
	}

	for i := range pods {
		if ret := p.Match(&pods[i]); ret != expected[i] {
			t.Fatalf(
				`policy "%s" returned %v for pod "%s", expected %v`,
				policyName, ret, pods[i].Name, expected[i],
			)
		}
	}
}

type mockMatcher bool

func (m mockMatcher) Match(_ *v1.Pod) bool {
	return bool(m)
}

func TestPolicyNsMatcher_Match(t *testing.T) {
	t.Parallel()

	matchAll := operator.NewNamespaceMatcher(map[string]operator.PodMatcher{
		"*": mockMatcher(true),
	})

	if !matchAll.Match(nil, "unknown") {
		t.Fatalf("did not match for unknown ns when default policy was match")
	}

	matchCustom := operator.NewNamespaceMatcher(map[string]operator.PodMatcher{
		"*":      mockMatcher(false),
		"custom": mockMatcher(true),
	})

	if matchCustom.Match(nil, "unknown") {
		t.Fatalf("matched for unknown ns when default policy was not match")
	}

	if !matchCustom.Match(nil, "custom") {
		t.Fatalf("did not match for specified ns when policy was match")
	}

	matchNoDefault := operator.NewNamespaceMatcher(map[string]operator.PodMatcher{
		"custom": mockMatcher(true),
	})

	if matchNoDefault.Match(nil, "unknown") {
		t.Fatalf("matched for unknown ns when no default policy was set")
	}

	if !matchNoDefault.Match(nil, "custom") {
		t.Fatalf("did not match for specified ns when policy was match")
	}
}
