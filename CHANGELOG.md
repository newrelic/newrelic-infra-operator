# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).


## [0.8.0]

### Changed
* bump go version and dependencies (#229)

## [0.7.0]
### Changed
- Bumped dependencies

## [0.6.0]

### Changed
- Bumped dependencies

## [0.5.0]

### Changed
Adds Kubernetes 1.22 dependencies updates and some cleanups.

## [0.4.0]

### Changed

#### Modified the volumes names in order to avoid collisions

This change was needed in order to be able to instrument nri-kube-events.

## [0.3.0]

### Changed

First release packaged in the `newrelic-infra-operator` chart

#### Configure in which pods the sidecar should be injected

Policies are available in order to configure in which pods the sidecar should be injected.
Each policy is evaluated independently and if at least one policy matches the operator will inject the sidecar.

Policies are composed by `namespaceSelector` checking the labels of the Pod namespace, `podSelector` checking
the labels of the Pod and `namespace` checking the namespace name. Each of those, if specified, are ANDed.

By default, the policies are configured in order to inject the sidecar in each pod belonging to a Fargate profile.

>Moreover, it is possible to add the label `infra-operator.newrelic.com/disable-injection` to Pods to exclude injection
for a single Pod that otherwise would be selected by the policies.

Please make sure to configure policies correctly to avoid injecting sidecar for pods running on EC2 nodes
already monitored by the infrastructure DaemonSet.

#### Configure the sidecar with labelsSelectors

It is also possible to configure `resourceRequirements` and `extraEnvVars` based on the labels of the mutating Pod.

The current configuration increases the resource requirements for sidecar injected on `KSM` instances. Moreover, 
injectes disable the `DISABLE_KUBE_STATE_METRICS` environment variable for Pods not running on `KSM` instances
to decrease the load on the API server.

#### Hash computed for each configWithSelectors

Right now, we hash an injected container without environment variables
or resource requirements. This commit improves that and add all other
configuration options to the mix, which make sense, like cluster name,
resource prefix etc.

Also now, each config selector will have it's own hash, with specific
value, so when config selector configuration changes, only affected pods
will need to be re-created.

Alternatively, we could cache entire configuration struct, however, that
would give a lot of false positives, as in any configuration change, all
pods would have to be re-created.


## [0.2.0]

### Changed

- Moving CustomAttributes in the agentConfig

## [0.1.0]

### Added

- Initial release

[0.1.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.1.0
[0.2.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.2.0
[0.3.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.3.0
[0.4.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.4.0
[0.5.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.5.0
