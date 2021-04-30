package operator

import (
	"log"
	"strconv"

	v1 "k8s.io/api/core/v1"
)

// defaultNewRelicLabel is added by default to the "labels" section of policies.
const defaultNewRelicLabel = "newrelic.com/inject"

// PodMatcher is anything capable of deciding if a pod matches some criteria.
type PodMatcher interface {
	Match(pod *v1.Pod) bool
}

// fargateLabels returns a list of labels known to be applied by EKS to pods scheduled in fargate.
func fargateLabels() []string {
	return []string{
		"eks.amazonaws.com/fargate-profile",
	}
}

// Policy models a label-matching policy.
type Policy struct {
	Unlabeled bool // Whether this policy should match implicitly if no label says otherwise
	Fargate   bool // Syntax sugar for default fargate labels
	// Labels to check. Policy will match if at least one of these have a truthy value and no one has a falsy value
	Labels []string
}

// Match takes a pod and checks if it matches based on its labels.
func (p *Policy) Match(pod *v1.Pod) bool {
	match := p.Unlabeled

	if pod == nil {
		return match
	}

	extraLabels := []string{defaultNewRelicLabel}
	if p.Fargate {
		extraLabels = append(extraLabels, fargateLabels()...)
	}

	for _, label := range append(p.Labels, extraLabels...) {
		truthy, falsy := interpretLabel(pod.Labels[label])
		// We have a positive match, but they're not absolute
		if truthy {
			match = true
		}

		// A negative match overrides everything
		if falsy {
			return false
		}
	}

	return match
}

// interpretLabel takes a label value and returns if it is both conclusively truthy and conclusively false.
func interpretLabel(label string) (truthy bool, falsy bool) {
	parsedBool, err := strconv.ParseBool(label)
	// If it has a bool-parsable value, return it
	if err == nil {
		return parsedBool, !parsedBool
	}

	// If cannot be parsed as bool, return truthy if non-empty, inconclusive otherwise
	return len(label) > 0, false
}

const defaultPolicyName = "*"

// NamespaceMatcher is an object able to check whether a pod should be injected, given the pod and the namespace.
type NamespaceMatcher struct {
	matchers map[string]PodMatcher // Map of string (namespace name) to matchers that apply to it
}

// NewNamespaceMatcher returns a NamespaceMatcher given a map of matchers indexed by namespace.
func NewNamespaceMatcher(matchers map[string]PodMatcher) *NamespaceMatcher {
	return &NamespaceMatcher{
		matchers: matchers,
	}
}

// Apply instructs a NamespaceMatcher to apply a given Matcher to the specified namespace
// Any existing matcher for that namespace will be overwritten
// A special namespace name, `*`, can be used as a catch-all for namespaces without a specific matcher.
func (pm *NamespaceMatcher) Apply(namespace string, m PodMatcher) {
	pm.matchers[namespace] = m
}

// Match returns true if the given pod matches the matcher for the configured namespace.
// namespaces without a specific matcher will use the matcher for the special entry `*`.
func (pm *NamespaceMatcher) Match(pod *v1.Pod, namespace string) bool {
	// Try to fetch policy for specific namespace
	if pol := pm.matchers[namespace]; pol != nil {
		return pol.Match(pod)
	}

	// Otherwise get default policy
	pol, found := pm.matchers[defaultPolicyName]
	if !found {
		podname := "nil pod"
		if pod != nil {
			podname = pod.Name
		}

		log.Printf("cannot match %s/%s as neither specific nor default policy was found", namespace, podname)

		return false
	}

	return pol.Match(pod)
}
