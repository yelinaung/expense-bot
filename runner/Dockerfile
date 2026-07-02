# The base image drops to a non-root user via its own entrypoint;
# root is only used here for build-time package installation.
# myoung34/github-runner:ubuntu-noble
# NOSONAR
FROM myoung34/github-runner:2.335.1-ubuntu-noble@sha256:e2e1ff7077ffa67986a5db7dd61b473cc3b0f661a6db8bfee41cb59eb9aab4d1

ARG GO_VERSION=1.26.4
ARG GOLANGCI_LINT_VERSION=2.12.2
ARG GORELEASER_VERSION=2.16.0
ARG GITLEAKS_VERSION=8.30.1
ARG SEMGREP_VERSION=1.167.0
ARG MISE_VERSION=2026.6.11
# renovate: datasource=github-commits depName=golangci/golangci-lint
ARG GOLANGCI_LINT_INSTALL_COMMIT=2c56c9c84fbfa69db9c3fa70b0cc763d6d1a1746
# renovate: datasource=github-commits depName=securego/gosec
ARG GOSEC_INSTALL_COMMIT=5f4eec95fa28ce5dc6cf555de8c242cb57545f01

# Use bash with pipefail for RUN steps that use curl|tar pipelines.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY sources.list /etc/apt/sources.list
COPY tools/gofumpt /tmp/tools/gofumpt
COPY tools/govulncheck /tmp/tools/govulncheck
# hadolint ignore=DL3008
RUN apt-get update && apt-get install -y --no-install-recommends file jq pipx && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p "/opt/hostedtoolcache/go/${GO_VERSION}/x64" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C "/opt/hostedtoolcache/go/${GO_VERSION}/x64" --strip-components=1 && \
    touch "/opt/hostedtoolcache/go/${GO_VERSION}/x64.complete"

ENV PATH="/root/.local/bin:/opt/hostedtoolcache/go/${GO_VERSION}/x64/bin:/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin:${PATH}" \
    GOPATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages" \
    GOBIN="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin"

# hadolint ignore=DL3062
RUN pipx install semgrep=="${SEMGREP_VERSION}" && \
    go -C /tmp/tools/gofumpt install -mod=readonly mvdan.cc/gofumpt && \
    go -C /tmp/tools/govulncheck install -mod=readonly golang.org/x/vuln/cmd/govulncheck && \
    go install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_INSTALL_COMMIT}" && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-x64.tar.gz" | \
    tar -xz -C /usr/local/bin --strip-components=2 mise/bin/mise && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCI_LINT_INSTALL_COMMIT}/install.sh" \
      -o /tmp/golangci-lint-install.sh && \
    sh /tmp/golangci-lint-install.sh -b "${GOBIN}" "v${GOLANGCI_LINT_VERSION}" && \
    rm /tmp/golangci-lint-install.sh && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" | \
    tar -xz -C /usr/local/bin gitleaks && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" | \
    tar -xz -C /usr/local/bin goreleaser && \
    go clean -modcache && \
    echo "=== Installed tool versions ===" && \
    go version && \
    gofumpt --version && \
    govulncheck -version && \
    gosec --version && \
    golangci-lint --version && \
    goreleaser --version && \
    gitleaks version && \
    semgrep --version && \
    mise --version
