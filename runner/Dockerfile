FROM myoung34/github-runner:latest

ARG GO_VERSION=1.25.7
ARG GOLANGCI_LINT_VERSION=2.8.0

RUN mkdir -p /opt/hostedtoolcache/go/${GO_VERSION}/x64 && \
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" | \
    tar -xz -C /opt/hostedtoolcache/go/${GO_VERSION}/x64 --strip-components=1 && \
    touch /opt/hostedtoolcache/go/${GO_VERSION}/x64.complete

ENV PATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/bin:/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin:${PATH}" \
    GOPATH="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages" \
    GOBIN="/opt/hostedtoolcache/go/${GO_VERSION}/x64/packages/bin"

RUN go install mvdan.cc/gofumpt@latest && \
    go install golang.org/x/vuln/cmd/govulncheck@latest && \
    curl -fsSL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | \
    sh -s -- -b "${GOBIN}" "v${GOLANGCI_LINT_VERSION}" && \
    curl -fsSL https://github.com/gitleaks/gitleaks/releases/latest/download/gitleaks_8.30.0_linux_x64.tar.gz | \
    tar -xz -C /usr/local/bin gitleaks && \
    go clean -modcache
