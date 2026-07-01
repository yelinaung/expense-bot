#!/bin/sh
# Export the whole GitHub Actions run as an OpenTelemetry trace AND its job
# logs to ClickStack. One root span for the run plus a child span per job
# (built from the GitHub Actions API with otel-cli), and one OTLP log record
# per job (the whole job log as the body) stamped with the same trace id +
# that job's span id, so logs and spans correlate in the backend without
# exploding the trace into one span per log line.
#
# This is the self-built counterpart to corentinmusard/otel-cicd-action: the
# action only emits traces and hides the span ids it mints, which makes
# log<->span correlation impossible. Here we force our own ids (mirroring
# scripts/ci/export-otel-trace.sh on GitLab) so we can attach correlated logs.
#
# POSIX sh. Expects in the environment:
#   OTEL_CLI_VERSION              pinned otel-cli release
#   OTEL_EXPORTER_OTLP_ENDPOINT   collector base URL (we append /v1/traces, /v1/logs)
#   OTEL_EXPORTER_OTLP_PROTOCOL   http/protobuf
#   OTEL_EXPORTER_OTLP_HEADERS    ingestion auth header(s), e.g. authorization=<key>
#   OTEL_ENDPOINT_HOST_IP         (optional) IP to map the endpoint host to in /etc/hosts
#   OTEL_LOG_MAX_LINES            (optional) per-job log line cap, default 2000
#   GITHUB_TOKEN                  token with actions:read for the jobs + logs API
#   GITHUB_* predefined Actions variables
set -eu

: "${OTEL_EXPORTER_OTLP_ENDPOINT:?OTEL_EXPORTER_OTLP_ENDPOINT not set}"
: "${GITHUB_TOKEN:?GITHUB_TOKEN not set}"
# OTEL_EXPORTER_OTLP_ENDPOINT is the collector BASE URL (no signal path). Both
# signals are derived from it: traces -> ${BASE}/v1/traces (appended by
# otel-cli), logs -> ${BASE}/v1/logs (our raw POST). Tolerate a value that
# accidentally includes a trailing slash or signal path so the two never drift.
BASE=${OTEL_EXPORTER_OTLP_ENDPOINT%/}
BASE=${BASE%/v1/traces}
BASE=${BASE%/v1/logs}
BASE=${BASE%/}
TRACES_ENDPOINT="${BASE}/v1/traces"
LOGS_ENDPOINT="${BASE}/v1/logs"
MAX_LINES=${OTEL_LOG_MAX_LINES:-2000}
API=${GITHUB_API_URL:-https://api.github.com}
ATTEMPT=${GITHUB_RUN_ATTEMPT:-1}

command -v curl >/dev/null 2>&1 || { echo "curl not found"; exit 1; }
command -v jq   >/dev/null 2>&1 || { echo "jq not found"; exit 1; }

# --- install otel-cli into a local dir (pinned + checksum-verified) ---
# Local dir avoids needing root/sudo on the self-hosted runner.
echo "Installing otel-cli v${OTEL_CLI_VERSION}..."
BIN_DIR=$(mktemp -d)
# Clean up the temp dir (binary, checksums, header config, scratch files) so we
# don't accumulate artifacts on long-lived self-hosted runners.
trap 'rm -rf "${BIN_DIR}"' EXIT
GH="https://github.com/equinix-labs/otel-cli/releases/download/v${OTEL_CLI_VERSION}"
TARBALL="otel-cli_${OTEL_CLI_VERSION}_linux_amd64.tar.gz"
curl --proto "=https" -sSfL -o "${BIN_DIR}/${TARBALL}" "${GH}/${TARBALL}"
curl --proto "=https" -sSfL -o "${BIN_DIR}/checksums.txt" "${GH}/checksums.txt"
( cd "${BIN_DIR}" && grep " ${TARBALL}\$" checksums.txt | sha256sum -c - )
tar -xzf "${BIN_DIR}/${TARBALL}" -C "${BIN_DIR}" otel-cli
OTEL_CLI="${BIN_DIR}/otel-cli"

# otel-cli defaults to gRPC; force HTTP and surface export failures. It appends
# the /v1/traces signal path to the base endpoint itself.
OTEL_ARGS="--protocol http/protobuf --endpoint ${BASE} --fail --verbose"
echo "Traces -> ${TRACES_ENDPOINT}, logs -> ${LOGS_ENDPOINT}"

# The runner may resolve via LAN DNS and be unable to resolve the Tailscale
# MagicDNS name. Optionally map the hostname to a reachable IP. This only runs
# when OTEL_ENDPOINT_HOST_IP is set (opt-in), and only escalates via sudo when
# OTEL_HOSTS_SUDO is explicitly enabled — otherwise a non-writable /etc/hosts
# just warns and falls back to DNS. It never fails the job.
OTEL_HOST=${OTEL_EXPORTER_OTLP_ENDPOINT#*://}
OTEL_HOST=${OTEL_HOST%%/*}
OTEL_HOST=${OTEL_HOST%%:*}
if [ -n "${OTEL_ENDPOINT_HOST_IP:-}" ]; then
  if echo "${OTEL_ENDPOINT_HOST_IP} ${OTEL_HOST}" >> /etc/hosts 2>/dev/null; then
    echo "Mapped ${OTEL_HOST} -> ${OTEL_ENDPOINT_HOST_IP} via /etc/hosts"
  elif [ "${OTEL_HOSTS_SUDO:-}" = "1" ] && command -v sudo >/dev/null 2>&1 &&
       echo "${OTEL_ENDPOINT_HOST_IP} ${OTEL_HOST}" | sudo -n tee -a /etc/hosts >/dev/null 2>&1; then
    echo "Mapped ${OTEL_HOST} -> ${OTEL_ENDPOINT_HOST_IP} via /etc/hosts (sudo)"
  else
    echo "WARNING: could not write /etc/hosts; relying on DNS for ${OTEL_HOST}"
  fi
fi

# --- curl config with OTLP auth headers (for our raw /v1/logs POSTs) ---
# Written as a curl -K config file to sidestep quoting/word-splitting.
# Split the comma-separated header list via parameter expansion instead of
# tampering with IFS globally.
HDR_CONF="${BIN_DIR}/otlp-headers.conf"
: > "${HDR_CONF}"
headers=${OTEL_EXPORTER_OTLP_HEADERS:-}
while [ -n "${headers}" ]; do
  case "${headers}" in
    *,*) pair=${headers%%,*}; headers=${headers#*,} ;;
    *)   pair=${headers};     headers= ;;
  esac
  [ -n "${pair}" ] || continue
  key=${pair%%=*}
  val=${pair#*=}
  printf 'header = "%s: %s"\n' "${key}" "${val}" >> "${HDR_CONF}"
done

# Keep all scratch (jobs.json, per-job logs, payloads) inside BIN_DIR so the
# EXIT trap removes it too. Nothing below this point needs the repo checkout.
cd "${BIN_DIR}"

# --- fetch this run's jobs from the GitHub API ---
gh_api() {
  curl -sSfL \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$@"
}
# Paginate: the jobs API caps at 100 per page, so follow pages until we've
# collected total_count, merging each page's .jobs into a single {jobs:[...]}.
JOBS_URL="${API}/repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/attempts/${ATTEMPT}/jobs"
echo '[]' > jobs-acc.json
page=1
while :; do
  gh_api "${JOBS_URL}?per_page=100&page=${page}" > jobs-page.json
  jq -s '.[0] + .[1].jobs' jobs-acc.json jobs-page.json > jobs-acc.tmp
  mv jobs-acc.tmp jobs-acc.json
  total=$(jq -r '.total_count' jobs-page.json)
  got=$(jq -r '.jobs | length' jobs-page.json)
  have=$(jq -r 'length' jobs-acc.json)
  [ "${got}" -eq 0 ] && break
  [ "${have}" -ge "${total}" ] && break
  page=$((page + 1))
done
jq '{jobs: .}' jobs-acc.json > jobs.json
echo "Jobs fetched: $(jq '.jobs | length' jobs.json) across ${page} page(s), completed: $(jq '[.jobs[]|select(.completed_at!=null)]|length' jobs.json)"

# --- run span boundaries from job timestamps ---
RUN_START=$(jq -r '[.jobs[].started_at   | select(. != null)] | min // empty' "${BIN_DIR}/jobs.json")
RUN_END=$(jq   -r '[.jobs[].completed_at | select(. != null)] | max // empty' "${BIN_DIR}/jobs.json")
: "${RUN_START:?no started jobs found - nothing to export}"
: "${RUN_END:=$RUN_START}"
echo "Run window: ${RUN_START} -> ${RUN_END}"

# --- generate our own trace/span ids so children + logs link deterministically
# (W3C: 16-byte trace id, 8-byte span id, as hex). ---
TRACE_ID=$(head -c 16 /dev/urandom | od -An -tx1 | tr -d ' \n')
ROOT_SPAN_ID=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')
SERVICE_NAME="github-ci/${GITHUB_REPOSITORY}"
echo "Trace ID: ${TRACE_ID}  Service: ${SERVICE_NAME}"

# --- root span for the run ---
# shellcheck disable=SC2086
"${OTEL_CLI}" span ${OTEL_ARGS} \
  --service "${SERVICE_NAME}" \
  --name "run #${GITHUB_RUN_ID} (${GITHUB_REF_NAME:-})" \
  --start "${RUN_START}" --end "${RUN_END}" \
  --attrs "ci.run.id=${GITHUB_RUN_ID},ci.run.attempt=${ATTEMPT},ci.repo=${GITHUB_REPOSITORY},ci.commit.ref=${GITHUB_REF_NAME:-},ci.commit.sha=${GITHUB_SHA:-}" \
  --force-trace-id "${TRACE_ID}" --force-span-id "${ROOT_SPAN_ID}"
echo "Root span sent."

# jq program: wrap a whole job log (slurped as one string via -R -s) into a
# single OTLP/HTTP JSON log record carrying trace_id + this job's span_id, so
# each job contributes ONE correlated log entry rather than one span per line.
# $trace/$span/etc. are jq variables, not shell — single quotes are correct.
# shellcheck disable=SC2016
LOGS_JQ='
{ resourceLogs: [ {
    resource: { attributes: [
      { key: "service.name", value: { stringValue: $service } } ] },
    scopeLogs: [ { scope: { name: "github-actions-exporter" },
      logRecords: [ {
        timeUnixNano: $nanos,
        severityText: $severity,
        severityNumber: ($sevnum | tonumber),
        body: { stringValue: . },
        traceId: $trace, spanId: $span,
        attributes: [
          { key: "ci.job.name",   value: { stringValue: $jobname } },
          { key: "ci.job.id",     value: { stringValue: $jobid } },
          { key: "ci.job.status", value: { stringValue: $jobstatus } } ] } ] } ] } ] }
'

# --- one child span + correlated logs per completed job ---
jq -r '.jobs[] | select(.started_at != null and .completed_at != null)
        | [.name, (.conclusion // .status), .started_at, .completed_at, (.id|tostring)] | @tsv' "${BIN_DIR}/jobs.json" |
while IFS="$(printf '\t')" read -r name status started finished id; do
  echo "  span: ${name} [${status}] ${started} -> ${finished}"
  SPAN_ID=$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')
  # shellcheck disable=SC2086
  "${OTEL_CLI}" span ${OTEL_ARGS} \
    --service "${SERVICE_NAME}" \
    --name "${name}" \
    --start "${started}" --end "${finished}" \
    --attrs "ci.job.id=${id},ci.job.status=${status}" \
    --force-trace-id "${TRACE_ID}" --force-span-id "${SPAN_ID}" \
    --force-parent-span-id "${ROOT_SPAN_ID}"

  # Ship this job's log as a single correlated record. Best-effort: a failure
  # here must not abort the remaining jobs' export.
  case "${status}" in
    failure)                        sev_text=ERROR; sev_num=17 ;;
    cancelled|timed_out|action_required) sev_text=WARN; sev_num=13 ;;
    *)                              sev_text=INFO;  sev_num=9  ;;
  esac
  # Each fallible step is guarded individually with `if` so errexit stays on
  # for the rest of the loop: a genuine bug still aborts, but a per-job log
  # fetch/build/upload failure just warns and moves on.
  if ! gh_api "${API}/repos/${GITHUB_REPOSITORY}/actions/jobs/${id}/logs" -o job.log; then
    echo "    WARNING: could not fetch logs for job ${id}"
  elif [ -s job.log ]; then
    nanos=$(jq -rn --arg ts "${finished}" \
      '($ts[0:19] + "Z" | fromdateiso8601 | tostring) + "000000000"')
    # Normalize CRLF -> LF so the body carries clean \n line breaks, then emit
    # compact JSON (-c) so the payload's only real newlines are the in-string
    # \n escapes. A failed build skips the upload rather than sending garbage.
    if tr -d '\r' < job.log | tail -n "${MAX_LINES}" | jq -R -s -c \
        --arg trace "${TRACE_ID}" --arg span "${SPAN_ID}" \
        --arg jobname "${name}" --arg jobid "${id}" --arg jobstatus "${status}" \
        --arg service "${SERVICE_NAME}" --arg nanos "${nanos}" \
        --arg severity "${sev_text}" --arg sevnum "${sev_num}" \
        "${LOGS_JQ}" > logs-payload.json; then
      # --data-binary (not --data): --data strips newlines from file input,
      # which would concatenate the log lines; --data-binary sends bytes as-is.
      if curl -sS --fail -K "${HDR_CONF}" \
           -H "Content-Type: application/json" \
           -X POST "${LOGS_ENDPOINT}" --data-binary @logs-payload.json >/dev/null; then
        echo "    log sent (1 record, $(wc -l < job.log | tr -d ' ') lines, capped ${MAX_LINES})"
      else
        echo "    WARNING: log export failed for job ${id}"
      fi
    else
      echo "    WARNING: could not build log payload for job ${id}"
    fi
  fi
done
echo "Trace + log export complete."
