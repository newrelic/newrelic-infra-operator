# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased
### bugfix
- Fix bug reported in [NR-153839](https://issues.newrelic.com/browse/NR-153839) by @juanjjaramillo in [#319](https://github.com/newrelic/newrelic-infra-operator/pull/319)
### enhancement
- Update k8s versions in CI by @xqi-nr in [#323](https://github.com/newrelic/newrelic-infra-operator/pull/323)
- Refactor `changelog` workflow to use reusable workflow by @juanjjaramillo in [#339](https://github.com/newrelic/newrelic-infra-operator/pull/339)

## [0.10.2]

### What's Changed
* Update CHANGELOG.md by @juanjjaramillo in https://github.com/newrelic/newrelic-infra-operator/pull/292
* Bump versions by @juanjjaramillo in https://github.com/newrelic/newrelic-infra-operator/pull/293
* chore(deps): bump aquasecurity/trivy-action from 0.10.0 to 0.11.0 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/294
* chore(deps): bump github.com/sirupsen/logrus from 1.9.2 to 1.9.3 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/295
* chore(deps): update newrelic/infrastructure-k8s docker tag to v2.13.10 by @renovate in https://github.com/newrelic/newrelic-infra-operator/pull/296
* chore(deps): bump alpine from 3.18.0 to 3.18.2 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/299
* chore(deps): bump k8s.io/apimachinery from 0.27.2 to 0.27.3 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/301
* chore(deps): bump aquasecurity/trivy-action from 0.11.0 to 0.11.2 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/297
* Upgrade Go Version by @xqi-nr in https://github.com/newrelic/newrelic-infra-operator/pull/302

### New Contributors
* @xqi-nr made their first contribution in https://github.com/newrelic/newrelic-infra-operator/pull/302

**Full Changelog**: https://github.com/newrelic/newrelic-infra-operator/compare/v0.10.1...v0.10.2

## [0.10.1]

### What's Changed
* Bump app and chart versions by @juanjjaramillo in https://github.com/newrelic/newrelic-infra-operator/pull/284
* chore(deps): bump newrelic/infrastructure-k8s from 2.13.6-unprivileged to 2.13.7-unprivileged by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/285
* Update Helm unit test reference by @juanjjaramillo in https://github.com/newrelic/newrelic-infra-operator/pull/286
* chore(deps): bump github.com/sirupsen/logrus from 1.9.0 to 1.9.2 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/288
* chore(deps): bump k8s.io/apimachinery from 0.27.1 to 0.27.2 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/290
* chore(deps): bump alpine from 3.17.3 to 3.18.0 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/287

**Full Changelog**: https://github.com/newrelic/newrelic-infra-operator/compare/v0.10.0...v0.10.1

## [0.10.0]

### Changed
* Bump chart version by @htroisi in https://github.com/newrelic/newrelic-infra-operator/pull/267
* Update Renovate configs by @htroisi in https://github.com/newrelic/newrelic-infra-operator/pull/268
* chore(deps): update helm release common-library to v1.1.1 by @renovate in https://github.com/newrelic/newrelic-infra-operator/pull/269
* Bump infrastructure-k8s version from 2.13.5 to 2.13.6 by @htroisi in https://github.com/newrelic/newrelic-infra-operator/pull/271
* chore(deps): bump helm/chart-testing-action from 2.3.1 to 2.4.0 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/270
* Bump chart version by @htroisi in https://github.com/newrelic/newrelic-infra-operator/pull/272
* chore(deps): bump alpine from 3.17.2 to 3.17.3 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/274
* chore(deps): bump sigs.k8s.io/controller-runtime from 0.14.5 to 0.14.6 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/275
* Fix helm unittest by @htroisi in https://github.com/newrelic/newrelic-infra-operator/pull/282
* chore(deps): bump aquasecurity/trivy-action from 0.9.2 to 0.10.0 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/280
* chore(deps): bump actions/github-script from 6.4.0 to 6.4.1 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/276
* chore(deps): bump k8s.io/apimachinery from 0.26.3 to 0.27.1 by @dependabot in https://github.com/newrelic/newrelic-infra-operator/pull/279
* chore(deps): update newrelic/infrastructure-k8s docker tag to v2.13.7 by @renovate in https://github.com/newrelic/newrelic-infra-operator/pull/281

## [0.9.0]

### Changed

* Bump dependencies
* Update Kubernetes image registry (#264)

## [0.8.0]

### Changed

* bump go version and dependencies (#229)

## [0.7.0]

### Changed

* Bumped dependencies

## [0.6.0]

### Changed

* Bumped dependencies

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

* Moving CustomAttributes in the agentConfig

## [0.1.0]

### Added

* Initial release

<!-- [0.1.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.1.0
[0.2.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.2.0
[0.3.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.3.0
[0.4.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.4.0
[0.5.0]: https://github.com/newrelic/newrelic-infra-operator/releases/tag/v0.5.0 -->
