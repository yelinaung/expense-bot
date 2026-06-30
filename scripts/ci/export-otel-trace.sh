#!/bin/sh
# Export the whole GitLab pipeline as an OpenTelemetry trace to ClickStack.
# One root span for the pipeline plus a child span per job, built from the
# GitLab API and shipped with otel-cli. Mirrors the GitHub Actions
# otel-cicd-action integration.
#
# Runs inside the alpine/busybox CI image (POSIX sh, not bash). Expects these
# to be present in the environment (GitLab predefined vars + CI/CD variables):
#   OTEL_CLI_VERSION              pinned otel-cli release
#   OTEL_EXPORTER_OTLP_ENDPOINT   collector base URL (otel-cli appends /v1/traces)
#   OTEL_EXPORTER_OTLP_PROTOCOL   http/protobuf
#   OTEL_EXPORTER_OTLP_HEADERS    ingestion auth header(s), e.g. authorization=<key>
#   OTEL_ENDPOINT_HOST_IP         (optional) IP to map the endpoint host to in /etc/hosts
#   GITLAB_API_TOKEN              (optional) read_api token if CI_JOB_TOKEN can't read jobs
#   CI_* / OTEL_* predefined GitLab variables
set -eu

apk add --no-cache curl >/dev/null

# --- install otel-cli (pinned + checksum-verified) ---
echo "Installing otel-cli v${OTEL_CLI_VERSION}..."
BASE="https://github.com/equinix-labs/otel-cli/releases/download/v${OTEL_CLI_VERSION}"
TARBALL="otel-cli_${OTEL_CLI_VERSION}_linux_amd64.tar.gz"
curl --proto "=https" -sSfL -o "${TARBALL}" "${BASE}/${TARBALL}"
curl --proto "=https" -sSfL -o checksums.txt "${BASE}/checksums.txt"
grep " ${TARBALL}\$" checksums.txt | sha256sum -c -
tar -xzf "${TARBALL}" -C /usr/local/bin otel-cli

# otel-cli defaults to gRPC; force HTTP and surface export failures.
# --fail makes a failed export return non-zero; --verbose logs to stderr.
OTEL_ARGS="--protocol http/protobuf --endpoint ${OTEL_EXPORTER_OTLP_ENDPOINT} --fail --verbose"
echo "Endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT} (otel-cli appends /v1/traces)"

# The runner resolves via LAN DNS and can't resolve the Tailscale MagicDNS
# name. Map the hostname to a reachable IP (set the OTEL_ENDPOINT_HOST_IP
# CI/CD variable to the Tailscale/LAN IP of the collector host) so we still
# connect by name and the TLS cert matches.
OTEL_HOST=${OTEL_EXPORTER_OTLP_ENDPOINT#*://}
OTEL_HOST=${OTEL_HOST%%/*}
OTEL_HOST=${OTEL_HOST%%:*}
if [ -n "${OTEL_ENDPOINT_HOST_IP:-}" ]; then
  echo "${OTEL_ENDPOINT_HOST_IP} ${OTEL_HOST}" >> /etc/hosts
  echo "Mapped ${OTEL_HOST} -> ${OTEL_ENDPOINT_HOST_IP} via /etc/hosts"
else
  echo "WARNING: OTEL_ENDPOINT_HOST_IP not set; relying on DNS for ${OTEL_HOST}"
fi

# --- fetch this pipeline's jobs from the GitLab API ---
# CI_JOB_TOKEN may lack access to the jobs API; set GITLAB_API_TOKEN
# (read_api) as a CI/CD variable if so.
if [ -n "${GITLAB_API_TOKEN:-}" ]; then
  AUTH_HEADER="PRIVATE-TOKEN: ${GITLAB_API_TOKEN}"
else
  AUTH_HEADER="JOB-TOKEN: ${CI_JOB_TOKEN}"
fi
curl -sSf --header "${AUTH_HEADER}" \
  "${CI_API_V4_URL}/projects/${CI_PROJECT_ID}/pipelines/${CI_PIPELINE_ID}/jobs?per_page=100" \
  > jobs.json
echo "Jobs fetched: $(jq 'length' jobs.json), completed: $(jq '[.[]|select(.finished_at!=null)]|length' jobs.json)"

# --- pipeline span boundaries from job timestamps ---
PIPELINE_START=$(jq -r '[.[].started_at  | select(. != null)] | min // empty' jobs.json)
PIPELINE_END=$(jq   -r '[.[].finished_at | select(. != null)] | max // empty' jobs.json)
: "${PIPELINE_START:?no started jobs found - nothing to export}"
: "${PIPELINE_END:=$PIPELINE_START}"
echo "Pipeline window: ${PIPELINE_START} -> ${PIPELINE_END}"

# --- generate our own trace/span ids so children link to the root
# deterministically (W3C: 16-byte trace id, 8-byte span id, as hex). ---
TRACE_ID=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
ROOT_SPAN_ID=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')
# Per-project service name (CI_PROJECT_PATH is namespace/project), kept under
# a common "gitlab-ci/" prefix so all CI is groupable but each project is
# distinguishable. No per-project edits needed.
SERVICE_NAME="gitlab-ci/${CI_PROJECT_PATH}"
echo "Trace ID: ${TRACE_ID}  Service: ${SERVICE_NAME}"

# --- root span for the pipeline ---
# OTEL_ARGS holds several flags that must word-split into separate args.
# shellcheck disable=SC2086
otel-cli span ${OTEL_ARGS} \
  --service "${SERVICE_NAME}" \
  --name "pipeline #${CI_PIPELINE_ID} (${CI_COMMIT_REF_NAME})" \
  --start "${PIPELINE_START}" --end "${PIPELINE_END}" \
  --attrs "ci.pipeline.id=${CI_PIPELINE_ID},ci.project.path=${CI_PROJECT_PATH},ci.commit.ref=${CI_COMMIT_REF_NAME},ci.commit.sha=${CI_COMMIT_SHA}" \
  --force-trace-id "${TRACE_ID}" --force-span-id "${ROOT_SPAN_ID}"
echo "Root span sent."

# --- one child span per completed job, parented to the root ---
jq -r '.[] | select(.started_at != null and .finished_at != null)
        | [.name, .status, .stage, .started_at, .finished_at, (.id|tostring)] | @tsv' jobs.json |
while IFS="$(printf '\t')" read -r name status stage started finished id; do
  echo "  span: ${name} [${status}] ${started} -> ${finished}"
  # shellcheck disable=SC2086
  otel-cli span ${OTEL_ARGS} \
    --service "${SERVICE_NAME}" \
    --name "${name}" \
    --start "${started}" --end "${finished}" \
    --attrs "ci.job.id=${id},ci.job.status=${status},ci.job.stage=${stage}" \
    --force-trace-id "${TRACE_ID}" --force-parent-span-id "${ROOT_SPAN_ID}"
done
echo "Trace export complete."
