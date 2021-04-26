[![Community Plus header](https://github.com/newrelic/opensource-website/raw/master/src/images/categories/Community_Plus.png)](https://opensource.newrelic.com/oss-category/#community-plus)

# New Relic Infrastructure Operator for Kubernetes

This operator automates injection of the New Relic Infrastructure container.

## Installation

> TBD

## Getting Started

> TBD

## Usage

> TBD

## Building

### Prerequisites

For the development process [kind](https://kind.sigs.k8s.io) and [tilt](https://tilt.dev/) tools are used.

* [Install kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
* [Install Tilt](https://docs.tilt.dev/install.html)

### Configuration

If you want to use kind cluster for testing, configure Tilt using the command below:

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

### Creating kind cluster

If you want to use local kind cluster for testing, create it with command below:

```sh
make kind
```

### Run

If you use kind cluster, simply run:

```sh
make tilt-up
```

If you deploy on external cluster, run the command below, pointing `TILT_KUBECONFIG` to your `kubeconfig` file:

```sh
TILT_KUBECONFIG=~/.kube/config make tilt-down
```

Now, when you do changes to the code, operator binary will be locally built, copied to the Pod and executed.

## Testing

> TBD

## Support

> TBD

## Contribute

We encourage your contributions to improve newrelic-infra-operator! Keep in mind that when you submit your pull request, you'll need to sign the CLA via the click-through using CLA-Assistant. You only have to sign the CLA one time per project.

If you have any questions, or to execute our corporate CLA (which is required if your contribution is on behalf of a company), drop us an email at opensource@newrelic.com.

**A note about vulnerabilities**

As noted in our [security policy](../../security/policy), New Relic is committed to the privacy and security of our customers and their data. We believe that providing coordinated disclosure by security researchers and engaging with the security community are important means to achieve our security goals.

If you believe you have found a security vulnerability in this project or any of New Relic's products or websites, we welcome and greatly appreciate you reporting it to New Relic through [HackerOne](https://hackerone.com/newrelic).

If you would like to contribute to this project, review [these guidelines](./CONTRIBUTING.md).

To all contributors, we thank you!  Without your contribution, this project would not be what it is today.  We also host a community project page dedicated to newrelic-infra-operator(<LINK TO https://opensource.newrelic.com/projects/... PAGE>).

## License

newrelic-infra-operator is licensed under the [Apache 2.0](http://apache.org/licenses/LICENSE-2.0.txt) License.

> The newrelic-infra-operator also uses source code from third-party libraries. You can find full details on which libraries are used and the terms under which they are licensed in the third-party notices document.
