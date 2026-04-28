# The base image drops to a non-root user via its own entrypoint;
# root is only used here for build-time package installation.
# myoung34/github-runner:ubuntu-noble
# NOSONAR
FROM myoung34/github-runner:2.334.0-ubuntu-noble@sha256:54ff99a42b92c94421046dc9a9c5e3ae7eeb63590d2d871faeea16c4d41e7868

ARG GO_VERSION=1.26.2
ARG GOFUMPT_VERSION=0.9.2
ARG GOVULNCHECK_VERSION=1.2.0
ARG GOSEC_VERSION=2.25.0
ARG GOLANGCI_LINT_VERSION=2.11.4
ARG GORELEASER_VERSION=2.15.4
ARG GITLEAKS_VERSION=8.30.1
ARG SEMGREP_VERSION=1.161.0
ARG MISE_VERSION=2026.4.18
# renovate: datasource=github-commits depName=golangci/golangci-lint
ARG GOLANGCI_LINT_INSTALL_COMMIT=870ddc133592c3609f26af3b6f05ce2dd4a7afda

# Use bash with pipefail for RUN steps that use curl|tar pipelines.
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY sources.list /etc/apt/sources.list
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

RUN pipx install semgrep=="${SEMGREP_VERSION}" && \
    go install "mvdan.cc/gofumpt@v${GOFUMPT_VERSION}" && \
    go install "golang.org/x/vuln/cmd/govulncheck@v${GOVULNCHECK_VERSION}" && \
    go install "github.com/securego/gosec/v2/cmd/gosec@v${GOSEC_VERSION}" && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://github.com/jdx/mise/releases/download/v${MISE_VERSION}/mise-v${MISE_VERSION}-linux-x64.tar.gz" | \
    tar -xz -C /usr/local/bin --strip-components=2 mise/bin/mise && \
    curl --proto "=https" --tlsv1.2 -sSf -L \
      "https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCI_LINT_INSTALL_COMMIT}/install.sh" | \
    sh -s -- -b "${GOBIN}" "v${GOLANGCI_LINT_VERSION}" && \
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
