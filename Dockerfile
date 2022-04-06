FROM alpine:3.15.4

# Set by Docker automatically.
# If building with `docker build` directly, make sure to set GOOS/GOARCH explicitly when calling make:
# `make build GOOS=linux GOARCH=amd64`
# Otherwise, make will not add suffixes to the binary name and Docker will not be able to find it.
# Alternatively, `make image` can also take care of producing the binary with the correct name and then running
# `docker build` for you.
ARG TARGETOS
ARG TARGETARCH

# Some old-ish versions of Docker do not support adding and renaming in the same line, and will instead
# interpret the second argument as a new folder to create and place the original file inside.
# For this reason, we workaround with regular ADD and then run mv inside the container.
ADD --chmod=755 newrelic-infra-operator-${TARGETOS}-${TARGETARCH} ./
RUN mv newrelic-infra-operator-${TARGETOS}-${TARGETARCH} /usr/local/bin/newrelic-infra-operator

ENTRYPOINT ["/usr/local/bin/newrelic-infra-operator"]
