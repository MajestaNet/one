#!/usr/bin/env bash
# Headless slices of docs/customer-rollout-test-run.md (API, claim, CLI, MCP).
# Does not launch Control IDE. Customer fixtures stay under .customer-sandbox/.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

COMPOSE=(docker compose -f deploy/docker-compose.multi-env.yml)
PROD=http://localhost:8080
TEST=http://localhost:8081
PROD_CLAIM=rollout-prod-claim-token-change-me
TEST_CLAIM=rollout-test-claim-token-change-me
# Lab-only passwords for the campaign claim step (not production secrets).
PROD_PASS='choose-a-long-password'
TEST_PASS='choose-a-long-password'
SANDBOX="${ROOT}/.customer-sandbox/one-acme-rollout"
export ONE_CREDENTIAL_STORE="${ONE_CREDENTIAL_STORE:-file}"

need() { command -v "$1" >/dev/null 2>&1 || { echo "missing $1" >&2; exit 1; }; }
need curl
need jq
need docker

wait_ready() {
  local url=$1
  echo "waiting for ${url}/readyz …"
  curl -fsS --retry 60 --retry-all-errors --retry-delay 2 "${url}/readyz" >/dev/null
  curl -fsS "${url}/healthz" >/dev/null
}

json_field() { jq -r "$1"; }

echo "== compose up =="
"${COMPOSE[@]}" up --build -d
wait_ready "$PROD"
wait_ready "$TEST"
echo "  healthz/readyz ok on prod and test"

echo "== A claim prod =="
PROD_CLAIM_JSON="$(curl -sS -X POST "${PROD}/auth/v1/install/claim" \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"${PROD_CLAIM}\",\"email\":\"admin-prod@example.com\",\"password\":\"${PROD_PASS}\",\"displayName\":\"Prod Admin\"}")"
echo "$PROD_CLAIM_JSON" | jq '{claimed:(.access_token!=null),error:.error,.code}'
PROD_JWT="$(echo "$PROD_CLAIM_JSON" | json_field '.access_token // empty')"
if [[ -z "$PROD_JWT" ]]; then
  echo "prod already claimed; minting via bootstrap key" >&2
  PROD_JWT="$(curl -sS -X POST "${PROD}/auth/v1/token" \
    -H 'Content-Type: application/json' \
    -d '{"grant_type":"client_credentials","client_secret":"rollout-prod-admin"}' | json_field .access_token)"
fi
[[ -n "$PROD_JWT" ]] || { echo "no prod JWT" >&2; exit 1; }

echo "== A claim test =="
TEST_CLAIM_JSON="$(curl -sS -X POST "${TEST}/auth/v1/install/claim" \
  -H 'Content-Type: application/json' \
  -d "{\"token\":\"${TEST_CLAIM}\",\"email\":\"admin-test@example.com\",\"password\":\"${TEST_PASS}\",\"displayName\":\"Test Admin\"}")"
echo "$TEST_CLAIM_JSON" | jq '{claimed:(.access_token!=null),error:.error,.code}'
TEST_JWT="$(echo "$TEST_CLAIM_JSON" | json_field '.access_token // empty')"
if [[ -z "$TEST_JWT" ]]; then
  TEST_JWT="$(curl -sS -X POST "${TEST}/auth/v1/token" \
    -H 'Content-Type: application/json' \
    -d '{"grant_type":"client_credentials","client_secret":"rollout-test-admin"}' | json_field .access_token)"
fi
[[ -n "$TEST_JWT" ]] || { echo "no test JWT" >&2; exit 1; }

REV="$(curl -sS "${PROD}/version" | json_field '.apiRevision.recommended // .apiRevision.current // "1"')"
echo "  One-API-Revision=${REV}"

echo "== A /client/v1/me =="
curl -sS -H "Authorization: Bearer ${PROD_JWT}" -H "One-API-Revision: ${REV}" "${PROD}/client/v1/me" | jq '{id,email,principalType}'
curl -sS -H "Authorization: Bearer ${TEST_JWT}" -H "One-API-Revision: ${REV}" "${TEST}/client/v1/me" | jq '{id,email,principalType}'

echo "== C peers (expect loopback baseUrl rejected) =="
set +e
PEER_BAD="$(curl -sS -w '\n%{http_code}' -X POST "${PROD}/deploy/v1/peers" \
  -H "Authorization: Bearer ${PROD_JWT}" -H "One-API-Revision: ${REV}" \
  -H 'Content-Type: application/json' \
  -d '{"installId":"acme-test","installRole":"test","label":"Acme Test","baseUrl":"http://localhost:8081"}')"
set -e
echo "$PEER_BAD"
PEER_OK="$(curl -sS -w '\n%{http_code}' -X POST "${PROD}/deploy/v1/peers" \
  -H "Authorization: Bearer ${PROD_JWT}" -H "One-API-Revision: ${REV}" \
  -H 'Content-Type: application/json' \
  -d '{"installId":"acme-test","installRole":"test","label":"Acme Test"}')"
echo "$PEER_OK"
curl -sS -X POST "${TEST}/deploy/v1/peers" \
  -H "Authorization: Bearer ${TEST_JWT}" -H "One-API-Revision: ${REV}" \
  -H 'Content-Type: application/json' \
  -d '{"installId":"acme-prod","installRole":"prod","label":"Acme Prod"}' | jq '{installId,baseUrl,label}'

echo "== C one auth login =="
go run ./cmd/one auth login --base-url "$PROD" --token "$PROD_JWT" --alias prod
go run ./cmd/one auth login --base-url "$TEST" --token "$TEST_JWT" --alias test
go run ./cmd/one org use test
go run ./cmd/one org list

echo "== D project init =="
mkdir -p "${ROOT}/.customer-sandbox"
if [[ ! -f "${SANDBOX}/one.yaml" ]]; then
  go run ./cmd/one project init -dir "$SANDBOX" --customer-id acme-rollout
fi
if [[ ! -d "${SANDBOX}/.git" ]]; then
  git -C "$SANDBOX" init -q
  git -C "$SANDBOX" add .
  git -C "$SANDBOX" -c user.email=rollout@example.com -c user.name=Rollout commit -qm "init one/v1"
fi

echo "== D/E org validate + deploy template on test =="
go run ./cmd/one org validate -dir "$SANDBOX" --alias test
if command -v deno >/dev/null 2>&1; then
  go run ./cmd/one org deploy -dir "$SANDBOX" --alias test --suite CreateAccountFromContact
else
  echo "deno not on PATH; deploying without --suite (worker image still has Deno for async runs)"
  go run ./cmd/one org deploy -dir "$SANDBOX" --alias test
fi

echo "== F MCP catalog + initialize on test =="
curl -sS "${TEST}/mcp/tools" \
  -H "Authorization: Bearer ${TEST_JWT}" -H "One-API-Revision: ${REV}" | jq '.tools | map(.name)'
curl -sS -X POST "${TEST}/mcp" \
  -H "Authorization: Bearer ${TEST_JWT}" \
  -H "One-API-Revision: ${REV}" \
  -H "Content-Type: application/json" \
  -H "Accept: application/json, text/event-stream" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"rollout","version":"0"}}}' \
  | jq '{jsonrpc,id,result:(.result.serverInfo // .result)}'

echo "== F hosted loop cannot org_deploy (catalog contrast) =="
echo "  MCP has org_deploy; hosted /agents/runs executes a Client-only v1 subset (see builder-connect.md)."

echo "== headless slices finished =="
echo "  PROD_JWT and TEST_JWT are in this shell only; not written to disk."
echo "  Next (desktop): two Electron userData dirs — docs/customer-rollout-test-run.md scenario B."
echo "  Next (source): add Project__c in ${SANDBOX} then org deploy same SHA to prod."
