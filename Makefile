# Use ?= for variable assignment so they can be overwritten with environment variables.
GO_PACKAGES ?= ./...
GO_TESTS ?= ^.*$
GO_CMD ?= go
GO_TEST ?= $(GO_CMD) test -mod=vendor -covermode=atomic -run $(GO_TESTS)
CGO_ENABLED ?= 0
LD_FLAGS ?= "-extldflags '-static'"
GO_BUILD ?= CGO_ENABLED=$(CGO_ENABLED) $(GO_CMD) build -mod=vendor -v -buildmode=exe -ldflags $(LD_FLAGS)

GOLANGCI_LINT_CONFIG_FILE ?= .golangci.yml

DOCKER_CMD ?= docker
IMAGE_REPO ?= newrelic/nri-k8s-operator

TILT_CMD ?= tilt
TEST_KUBECONFIG ?= $(shell realpath kubeconfig)

KIND_CMD ?= kind
KIND_SCRIPT ?= hack/kind-with-registry.sh
KIND_IMAGE ?= kindest/node:v1.19.7

.PHONY: build
build: ## Compiles operator binary.
	$(GO_BUILD) .

.PHONY: build-test
build-test: ## Compiles unit tests.
	$(GO_TEST) -run=nonexistent -tags integration,e2e $(GO_PACKAGES)

.PHONY: test
test: ## Runs all unit tests.
	$(GO_TEST) $(GO_PACKAGES)

.PHONY: test-integration
test-integration: ## Runs all integration tests.
	KUBECONFIG=$(TEST_KUBECONFIG) USE_EXISTING_CLUSTER=true $(GO_TEST) -tags integration $(GO_PACKAGES)

.PHONY: test-e2e
test-e2e: ## Runs all e2e tests. Expects operator to be installed on the cluster using Helm chart.
	KUBECONFIG=$(TEST_KUBECONFIG) $(GO_TEST) -tags e2e $(GO_PACKAGES)

.PHONY: vendor
vendor: ## Updates vendor directory.
	$(GO_CMD) mod vendor

.PHONY: ci
ci: check-vendor check-tidy build test ## Runs checks performed by CI without external dependencies required (e.g. golangci-lint).

.PHONY: check-working-tree-clean
check-working-tree-clean: ## Checks if working directory is clean.
	@test -z "$$(git status --porcelain)" || (echo "Commit all changes before running this target"; exit 1)

.PHONY: check-vendor
check-vendor: check-working-tree-clean vendor ## Checks if vendor directory is up to date.
	@test -z "$$(git status --porcelain)" || (echo "Please run 'make vendor' and commit generated changes."; git status; exit 1)

.PHONY: check-tidy
check-tidy: check-working-tree-clean ## Checks if Go module files are clean.
	go mod tidy
	@test -z "$$(git status --porcelain)" || (echo "Please run 'go mod tidy' and commit generated changes."; git status; exit 1)

.PHONY: check-update-linters
check-update-linters: check-working-tree-clean update-linters ## Checks if list of enabled golangci-lint linters is up to date.
	@test -z "$$(git status --porcelain)" || (echo "Linter configuration outdated. Run 'make update-linters' and commit generated changes to fix."; exit 1)

.PHONY: update-linters
update-linters: ## Updates list of enabled golangci-lint linters.
	# Remove all enabled linters.
	sed -i '/^  enable:/q0' $(GOLANGCI_LINT_CONFIG_FILE)
	# Then add all possible linters to config.
	golangci-lint linters | grep -E '^\S+:' | cut -d: -f1 | sort | sed 's/^/    - /g' | grep -v -E "($$(sed -e '1,/^  disable:$$/d' .golangci.yml  | grep -E '    - \S+$$' | awk '{print $$2}' | tr \\n '|' | sed 's/|$$//g'))" >> $(GOLANGCI_LINT_CONFIG_FILE)

.PHONY: lint
lint: build build-test ## Runs golangci-lint.
	golangci-lint run $(GO_PACKAGES)

.PHONY: codespell
codespell: CODESPELL_BIN := codespell
codespell: ## Runs spell checking.
	which $(CODESPELL_BIN) >/dev/null 2>&1 || (echo "$(CODESPELL_BIN) binary not found, skipping spell checking"; exit 0)
	$(CODESPELL_BIN)

.PHONY: image
image: ## Builds operator Docker image.
	$(DOCKER_CMD) build --rm=true -t $(IMAGE_REPO) .

.PHONY: image-push
image-push: image ## Builds and pushes operator Docker image.
	$(DOCKER_CMD) push $(IMAGE_REPO)

.PHONY: kind
kind: ## Creates local Kind cluster for development.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(KIND_SCRIPT)

.PHONY: update-kind
update-kind: ## Updates hack/kind-with-registry.sh file.
	wget https://kind.sigs.k8s.io/examples/kind-with-registry.sh -O $(KIND_SCRIPT)
	sed -i 's|kind create cluster|kind create cluster --image=$(KIND_IMAGE)|g' $(KIND_SCRIPT)
	chmod +x $(KIND_SCRIPT)

.PHONY: kind-down
kind-down: ## Cleans up local Kind cluster.
	$(KIND_CMD) delete cluster

.PHONY: tilt-up
tilt-up: ## Builds project and deploys it to local Kind cluster.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(TILT_CMD) up

.PHONY: tilt-down
tilt-down: ## Cleans up resources created by Tilt.
	env KUBECONFIG=$(TEST_KUBECONFIG) $(TILT_CMD) down

.PHONY: help
help: ## Prints help message.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
