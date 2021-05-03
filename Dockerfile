FROM alpine:3.13

# Set by Docker automatically
# If building with `docker build`, make sure to set GOOS/GOARCH explicitly when calling make:
# `make compile GOOS=something GOARCH=something`
# Otherwise the makefile will not append them to the binary name and docker build will fail.
ARG TARGETOS
ARG TARGETARCH

# Some old-ish versions of Docker do not support adding and renaming in the same line, and will instead
# interpret the second argument as a new folder to create and place the original file inside.
# For this reason, we workaround with regular ADD and then run mv inside the container.
ADD --chmod=755 newrelic-infra-operator-${TARGETOS}-${TARGETARCH} ./
RUN mv newrelic-infra-operator-${TARGETOS}-${TARGETARCH} /usr/local/bin/newrelic-infra-operator

ENTRYPOINT ["/usr/local/bin/newrelic-infra-operator"]
