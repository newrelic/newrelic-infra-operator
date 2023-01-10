[![Community header](https://github.com/newrelic/opensource-website/raw/main/src/images/categories/Community_Project.png)](https://opensource.newrelic.com/oss-category/#community-project)

# New Relic Infrastructure Operator for Kubernetes

This operator automates the injection of the New Relic Infrastructure sidecar on Pods matching the configured policies.

The `newrelic-infra-operator` sets up a `mutatingWebhookConfiguration`, which allows it to modify the Pod objects that are
about to be created in the cluster.

On this event, and when the Pod being created matches the user’s configured policies the operator will:

 - Add a sidecar container to the Pod, containing the New Relic Kubernetes Integration.
 - If a secret containing license key doesn't exist, create it in the same Namespace as the pod,
   since it is needed for the integration to report data.
 - Add the Pod’s service account to a `ClusterRoleBinding` previously created by the operator chart, which will grant this
   sidecar the required permissions to access the Kubernetes metrics endpoints. 
   

The ClusterRoleBinding grants the following permissions to the Pod being injected:
```yaml
- apiGroups: [""]
  resources:
    - "nodes"
    - "nodes/metrics"
    - "nodes/stats"
    - "nodes/proxy"
    - "pods"
    - "services"
      verbs: ["get", "list"]
- nonResourceURLs: ["/metrics"]
  verbs: ["get"]
```

In order to get the sidecar injected on Pods deployed before the operator has been installed, you need to manually
perform a rollout (restart) of the workloads. New Relic has chosen not to do this automatically in order to prevent 
unexpected service disruptions and resource usage spikes.

## Installation

You can install this chart using [`nri-bundle`](https://github.com/newrelic/helm-charts/tree/master/charts/nri-bundle) located in the
[helm-charts repository](https://github.com/newrelic/helm-charts) or directly from this repository by adding this Helm repository:

```shell
helm repo add newrelic-infra-operator https://newrelic.github.io/newrelic-infra-operator
helm upgrade --install newrelic-infra-operator/newrelic-infra-operator -f your-custom-values.yaml
```

Once deployed, it will automatically inject the sidecar in the Pod matching the policy specified.
Only Pods created after the deployment of the monitoring solution will be injected with the configuration and agent.

For further information regarding the installation refer to the official docs and to the `README.md` 
and the `values.yaml` of the [chart](https://github.com/newrelic/newrelic-infra-operator/tree/master/charts/newrelic-infra-operator).

### Develop, Test and Run Locally

For the development process [kind](https://kind.sigs.k8s.io) and [tilt](https://tilt.dev/) tools are used.

* [Install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [Install Tilt](https://docs.tilt.dev/install.html)

#### Building

To build the image:
```sh
GOOS=linux make image
```

To build the binary:
```sh
GOOS=linux make build
```

#### Configure Tilt

If you want to use a `kind` cluster for testing, configure Tilt using the command below:

```sh
cat <<EOF > tilt_option.json
{
  "default_registry": "localhost:5000"
}
EOF
```

If you want to use existing Kubernetes cluster, create `tilt_option.json` file with content similar to below:

```json
{
  "default_registry": "quay.io/<your username>",
  "allowed_contexts": "<kubeconfig context to use>"
}
```

##### Helm chart location

To deploy the operator, the current Tilt configuration expects the New Relic 
[helm-charts](https://github.com/newrelic/helm-charts) repository to be
cloned as a sibling to this repository under the name `helm-charts-newrelic`
This repository is an authoritative source of the deployment manifests for the operator.

If you have `helm-charts` repository cloned into a different path, you can configure Tilt to use it by adding the
following key-value pair to your local `tilt_option.json` file:

```
  "chart_path": "../../helm-charts-newrelic/charts/newrelic-infra-operator/"
```

#### Creating kind cluster

If you want to use a local `kind` cluster for testing, create it with command below:

```sh
make kind-up
```

#### Run

If you use a `kind` cluster, simply run:

```sh
make tilt-up
```

If you deploy on external cluster, run the command below, pointing `TILT_KUBECONFIG` to your `kubeconfig` file:

```sh
TILT_KUBECONFIG=~/.kube/config make tilt-down
```

Now, when you make changes to the code, the operator binary will be built locally, copied to the Pod, and then executed.

#### Testing

In order to run unit tests run:
```sh
make test
```
In order to run unit tests run:

```sh
make test-integration
make test-e2e
```

Notice that in order to run both integration tests and E2e, you will need a working environment available with the
`newrelic-infra-operator` running. 
Both installing the `newrelic-infra-operator` chart or spinning up the environment with `make tilt-up` are possible options.

It is also possible to run such tests against any cluster you have access to by setting the environment variable
`TEST_KUBECONFIG=/your/kube/config/path`. 

## Support

Should you need assistance with New Relic products, you are in good hands with several support diagnostic tools and support channels.

If the issue has been confirmed as a bug or is a feature request, file a GitHub issue.

**Support Channels**

* [New Relic Documentation](https://docs.newrelic.com): Comprehensive guidance for using our platform
* [New Relic Community](https://discuss.newrelic.com/t/eks-fargate-integration/148947): The best place to engage in troubleshooting questions
* [New Relic Developer](https://developer.newrelic.com/): Resources for building a custom observability applications
* [New Relic University](https://learn.newrelic.com/): A range of online training for New Relic users of every level
* [New Relic Technical Support](https://support.newrelic.com/) 24/7/365 ticketed support. Read more about our [Technical Support Offerings](https://docs.newrelic.com/docs/licenses/license-information/general-usage-licenses/support-plan).

## Contribute

We encourage your contributions to improve the newrelic-infra-operator! Keep in mind that when you submit your pull request,
you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per 
project.

If you have any questions, or to execute our corporate CLA (which is required if your contribution is on behalf of a
company), drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our
customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with
the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites,
we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

If you would like to contribute to this project, review [these guidelines](./CONTRIBUTING.md).

To all contributors, we thank you!  Without your contribution, this project would not be what it is today.

## License

newrelic-infra-operator is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.

> The newrelic-infra-operator also uses source code from third-party libraries. 
> You can find full details on which libraries are used, and the terms under which they are licensed in the third-party 
> notices document.
