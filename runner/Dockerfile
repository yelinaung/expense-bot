FROM myoung34/github-runner:ubuntu-jammy

ARG GO_VERSION=1.25.7
ARG GOLANGCI_LINT_VERSION=2.8.0
ARG GORELEASER_VERSION=2.8.2
ARG GITLEAKS_VERSION=8.30.0

USER root

RUN apt-get update && apt-get install -y --no-install-recommends jq && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p "/opt/hostedtoolcache/go/${GO_VERSION}/x64" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C "/opt/hostedtoolcache/go/${GO_VERSION}/x64" --strip-components=1 && \
    touch "/opt/hostedtoolcache/go/${GO_VERSION}/x64.complete"

ENV PATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/bin:/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin:${PATH}" \
    GOPATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages" \
    GOBIN="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin"

RUN go install mvdan.cc/gofumpt@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest && \
    go install github.com/securego/gosec/v2/cmd/gosec@latest && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh" | \
    sh -s -- -b "${GOBIN}" "v${GOLANGCI_LINT_VERSION}" && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/gitleaks/gitleaks/releases/download/v${GITLEAKS_VERSION}/gitleaks_${GITLEAKS_VERSION}_linux_x64.tar.gz" | \
    tar -xz -C /usr/local/bin gitleaks && \
    curl --proto "=https" --tlsv1.2 -sSf -L "https://github.com/goreleaser/goreleaser/releases/download/v${GORELEASER_VERSION}/goreleaser_Linux_x86_64.tar.gz" | \
    tar -xz -C /usr/local/bin goreleaser && \
    go clean -modcache

USER runner
