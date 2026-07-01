#!/bin/sh
# Export the whole GitHub Actions run as an OpenTelemetry trace AND its job
# logs to ClickStack. One root span for the run plus a child span per job
# (built from the GitHub Actions API with otel-cli), and every job's log lines
# shipped as OTLP log records stamped with the same trace id + that job's span
# id, so logs and spans correlate in the backend.
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
BASE=${OTEL_EXPORTER_OTLP_ENDPOINT%/}
MAX_LINES=${OTEL_LOG_MAX_LINES:-2000}
API=${GITHUB_API_URL:-https://api.github.com}
ATTEMPT=${GITHUB_RUN_ATTEMPT:-1}

command -v curl >/dev/null 2>&1 || { echo "curl not found"; exit 1; }
command -v jq   >/dev/null 2>&1 || { echo "jq not found"; exit 1; }

# --- install otel-cli into a local dir (pinned + checksum-verified) ---
# Local dir avoids needing root/sudo on the self-hosted runner.
echo "Installing otel-cli v${OTEL_CLI_VERSION}..."
BIN_DIR=$(mktemp -d)
GH="https://github.com/equinix-labs/otel-cli/releases/download/v${OTEL_CLI_VERSION}"
TARBALL="otel-cli_${OTEL_CLI_VERSION}_linux_amd64.tar.gz"
curl --proto "=https" -sSfL -o "${BIN_DIR}/${TARBALL}" "${GH}/${TARBALL}"
curl --proto "=https" -sSfL -o "${BIN_DIR}/checksums.txt" "${GH}/checksums.txt"
( cd "${BIN_DIR}" && grep " ${TARBALL}\$" checksums.txt | sha256sum -c - )
tar -xzf "${BIN_DIR}/${TARBALL}" -C "${BIN_DIR}" otel-cli
OTEL_CLI="${BIN_DIR}/otel-cli"

# otel-cli defaults to gRPC; force HTTP and surface export failures.
OTEL_ARGS="--protocol http/protobuf --endpoint ${OTEL_EXPORTER_OTLP_ENDPOINT} --fail --verbose"
echo "Endpoint: ${OTEL_EXPORTER_OTLP_ENDPOINT} (traces -> /v1/traces, logs -> /v1/logs)"

# The runner may resolve via LAN DNS and be unable to resolve the Tailscale
# MagicDNS name. Optionally map the hostname to a reachable IP (best-effort;
# needs write access to /etc/hosts, so try sudo and never fail the job).
OTEL_HOST=${OTEL_EXPORTER_OTLP_ENDPOINT#*://}
OTEL_HOST=${OTEL_HOST%%/*}
OTEL_HOST=${OTEL_HOST%%:*}
if [ -n "${OTEL_ENDPOINT_HOST_IP:-}" ]; then
  if echo "${OTEL_ENDPOINT_HOST_IP} ${OTEL_HOST}" >> /etc/hosts 2>/dev/null; then
    echo "Mapped ${OTEL_HOST} -> ${OTEL_ENDPOINT_HOST_IP} via /etc/hosts"
  elif command -v sudo >/dev/null 2>&1 &&
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

# --- fetch this run's jobs from the GitHub API ---
gh_api() {
  curl -sSfL \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${GITHUB_TOKEN}" \
    -H "X-GitHub-Api-Version: 2022-11-28" \
    "$@"
}
gh_api "${API}/repos/${GITHUB_REPOSITORY}/actions/runs/${GITHUB_RUN_ID}/attempts/${ATTEMPT}/jobs?per_page=100" \
  > jobs.json
echo "Jobs fetched: $(jq '.jobs | length' jobs.json), completed: $(jq '[.jobs[]|select(.completed_at!=null)]|length' jobs.json)"

# --- run span boundaries from job timestamps ---
RUN_START=$(jq -r '[.jobs[].started_at   | select(. != null)] | min // empty' jobs.json)
RUN_END=$(jq   -r '[.jobs[].completed_at | select(. != null)] | max // empty' jobs.json)
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

# jq program: turn a plaintext GitHub job log into an OTLP/HTTP JSON logs
# payload, one log record per line, each carrying trace_id + this job's span_id.
# GitHub prefixes every line with an RFC3339 timestamp we reuse for the record.
# $trace/$span/etc. are jq variables, not shell — single quotes are correct.
# shellcheck disable=SC2016
LOGS_JQ='
def nanos($ts):
  ($ts[0:19] + "Z" | fromdateiso8601 | tostring)
  + ((( ($ts | capture("\\.(?<f>[0-9]+)Z$") | .f) // "") + "000000000")[0:9]);
def parse($line):
  if ($line | test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z "))
  then ($line | capture("^(?<ts>\\S+) (?<msg>.*)$"))
  else {ts: null, msg: $line} end;
def sev($msg):
  if   ($msg | test("##\\[error\\]"))   then {t:"ERROR", n:17}
  elif ($msg | test("##\\[warning\\]")) then {t:"WARN",  n:13}
  else {t:"INFO", n:9} end;
split("\n")
| map(select(length > 0))
| map(parse(.) as $p | sev($p.msg) as $s
    | { timeUnixNano: (if $p.ts then nanos($p.ts) else $defaultNanos end),
        severityText: $s.t, severityNumber: $s.n,
        body: { stringValue: $p.msg },
        traceId: $trace, spanId: $span,
        attributes: [
          { key: "ci.job.name", value: { stringValue: $jobname } },
          { key: "ci.job.id",   value: { stringValue: $jobid } } ] })
| { resourceLogs: [ {
      resource: { attributes: [
        { key: "service.name", value: { stringValue: $service } } ] },
      scopeLogs: [ { scope: { name: "github-actions-exporter" }, logRecords: . } ] } ] }
'

# --- one child span + correlated logs per completed job ---
jq -r '.jobs[] | select(.started_at != null and .completed_at != null)
        | [.name, (.conclusion // .status), .started_at, .completed_at, (.id|tostring)] | @tsv' jobs.json |
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

  # Ship this job's logs, correlated to its span. Best-effort: a failure here
  # must not abort the remaining jobs' export.
  set +e
  if gh_api "${API}/repos/${GITHUB_REPOSITORY}/actions/jobs/${id}/logs" -o job.log; then
    default_nanos=$(jq -rn --arg ts "${finished}" \
      '($ts[0:19] + "Z" | fromdateiso8601 | tostring) + "000000000"')
    if [ -s job.log ]; then
      tail -n "${MAX_LINES}" job.log | jq -R -s \
        --arg trace "${TRACE_ID}" --arg span "${SPAN_ID}" \
        --arg jobname "${name}" --arg jobid "${id}" \
        --arg service "${SERVICE_NAME}" --arg defaultNanos "${default_nanos}" \
        "${LOGS_JQ}" > logs-payload.json
      if curl -sS --fail -K "${HDR_CONF}" \
           -H "Content-Type: application/json" \
           -X POST "${BASE}/v1/logs" --data @logs-payload.json >/dev/null; then
        echo "    logs sent ($(wc -l < job.log | tr -d ' ') lines, capped ${MAX_LINES})"
      else
        echo "    WARNING: log export failed for job ${id}"
      fi
    fi
  else
    echo "    WARNING: could not fetch logs for job ${id}"
  fi
  set -e
done
echo "Trace + log export complete."
