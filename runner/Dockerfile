# The base image drops to a non-root user via its own entrypoint;
# root is only used here for build-time package installation.
FROM myoung34/github-runner:ubuntu-jammy@sha256:5ba54aa260e83e387af9e8a8e255b8e8c8853b568aaf055dc63d53e0d4db057a

ARG NODE20_VERSION=20.20.1
ARG NODE24_VERSION=24.14.0
ARG GO_VERSION=1.26.1
ARG GOFUMPT_VERSION=0.9.2
ARG GOVULNCHECK_VERSION=1.1.4
ARG GOSEC_VERSION=2.24.7
ARG GOLANGCI_LINT_VERSION=2.11.3
ARG GORELEASER_VERSION=2.14.3
ARG GITLEAKS_VERSION=8.30.0
ARG SEMGREP_VERSION=1.155.0

# Use bash with pipefail for RUN steps that use curl|tar pipelines.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends jq file && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p "/opt/hostedtoolcache/go/${GO_VERSION}/x64" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C "/opt/hostedtoolcache/go/${GO_VERSION}/x64" --strip-components=1 && \
    touch "/opt/hostedtoolcache/go/${GO_VERSION}/x64.complete"

ENV PATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/bin:/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin:${PATH}" \
    GOPATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages" \
    GOBIN="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin"

# Install Node.js externals for GitHub Actions container jobs.
# The runner mounts these into containers as /__e/; Alpine (musl) builds
# are required for actions running inside Alpine-based containers.
# Both node20 and node24 are needed during the transition period.
RUN mkdir -p /actions-runner/externals/node20/bin && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://nodejs.org/dist/v${NODE20_VERSION}/node-v${NODE20_VERSION}-linux-x64.tar.gz" | \
    tar -xz -C /actions-runner/externals/node20 --strip-components=1 && \
    mkdir -p /actions-runner/externals/node20_alpine/bin && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://unofficial-builds.nodejs.org/download/release/v${NODE20_VERSION}/node-v${NODE20_VERSION}-linux-x64-musl.tar.gz" | \
    tar -xz -C /actions-runner/externals/node20_alpine --strip-components=1 && \
    mkdir -p /actions-runner/externals/node24/bin && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://nodejs.org/dist/v${NODE24_VERSION}/node-v${NODE24_VERSION}-linux-x64.tar.gz" | \
    tar -xz -C /actions-runner/externals/node24 --strip-components=1 && \
    mkdir -p /actions-runner/externals/node24_alpine/bin && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://unofficial-builds.nodejs.org/download/release/v${NODE24_VERSION}/node-v${NODE24_VERSION}-linux-x64-musl.tar.gz" | \
    tar -xz -C /actions-runner/externals/node24_alpine --strip-components=1 && \
    pip3 install --no-cache-dir semgrep=="${SEMGREP_VERSION}" && \
    go install "mvdan.cc/gofumpt@v${GOFUMPT_VERSION}" && \
    go install "golang.org/x/vuln/cmd/govulncheck@v${GOVULNCHECK_VERSION}" && \
    go install "github.com/securego/gosec/v2/cmd/gosec@v${GOSEC_VERSION}" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh" | \
    sh -s -- -b "${GOBIN}" "v${GOLANGCI_LINT_VERSION}" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" | \
    tar -xz -C /usr/local/bin gitleaks && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" | \
    tar -xz -C /usr/local/bin goreleaser && \
    go clean -modcache
