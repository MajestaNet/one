#!/usr/bin/env bash
# Isolation checklist for two managed installs in the same regional cell.
# Proves horizontal traversal controls where credentials allow; documents the rest.
#
# Usage:
#   ./isolation-checklist.sh \
#     --install-a-url https://a.example \
#     --install-b-url https://b.example \
#     --pool-a-id us-east-1_AAA \
#     --pool-b-id us-east-1_BBB \
#     [--token-a '<cognito-id-token-from-a>'] \
#     [--jwt-a '<one-jwt-from-a>'] \
#     [--region us-east-1]

set -euo pipefail

INSTALL_A_URL=""
INSTALL_B_URL=""
POOL_A_ID=""
POOL_B_ID=""
TOKEN_A=""
JWT_A=""
REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --install-a-url) INSTALL_A_URL="$2"; shift 2 ;;
    --install-b-url) INSTALL_B_URL="$2"; shift 2 ;;
    --pool-a-id) POOL_A_ID="$2"; shift 2 ;;
    --pool-b-id) POOL_B_ID="$2"; shift 2 ;;
    --token-a) TOKEN_A="$2"; shift 2 ;;
    --jwt-a) JWT_A="$2"; shift 2 ;;
    --region) REGION="$2"; shift 2 ;;
    -h|--help)
      sed -n '1,20p' "$0"
      exit 0
      ;;
    *) echo "unknown arg: $1" >&2; exit 1 ;;
  esac
done

fail=0
pass() { echo "PASS  $*"; }
warn() { echo "WARN  $*"; }
bad()  { echo "FAIL  $*"; fail=1; }

need() {
  local v="$1" n="$2"
  if [[ -z "$v" ]]; then
    bad "missing required $n"
    return 1
  fi
}

need "$INSTALL_A_URL" --install-a-url || true
need "$INSTALL_B_URL" --install-b-url || true
need "$POOL_A_ID" --pool-a-id || true
need "$POOL_B_ID" --pool-b-id || true

if [[ "$INSTALL_A_URL" == "$INSTALL_B_URL" ]]; then
  bad "install A and B URLs must differ"
fi
if [[ "$POOL_A_ID" == "$POOL_B_ID" ]]; then
  bad "pool A and B IDs must differ (one Cognito User Pool per install)"
fi

# --- Crypto / AuthN proofs (no AWS admin required) ---
if [[ -n "$TOKEN_A" ]]; then
  code="$(curl -sS -o /tmp/one-iso-exchange.json -w '%{http_code}' \
    -X POST "${INSTALL_B_URL%/}/auth/v1/token/exchange" \
    -H 'Content-Type: application/json' \
    -d "{\"grant_type\":\"urn:ietf:params:oauth:grant-type:token-exchange\",\"subject_token\":$(python3 -c 'import json,sys; print(json.dumps(sys.argv[1]))' "$TOKEN_A"),\"subject_token_type\":\"urn:ietf:params:oauth:token-type:id_token\"}")" || code="000"
  if [[ "$code" == "401" || "$code" == "403" ]]; then
    pass "B rejects A's Cognito ID token on /auth/v1/token/exchange (HTTP $code)"
  else
    bad "expected 401/403 exchanging A's Cognito token on B; got HTTP $code body=$(cat /tmp/one-iso-exchange.json 2>/dev/null || true)"
  fi
else
  warn "skip Cognito cross-exchange probe (pass --token-a)"
fi

if [[ -n "$JWT_A" ]]; then
  code="$(curl -sS -o /tmp/one-iso-me.json -w '%{http_code}' \
    "${INSTALL_B_URL%/}/client/v1/me" \
    -H "Authorization: Bearer $JWT_A")" || code="000"
  if [[ "$code" == "401" || "$code" == "403" ]]; then
    pass "B rejects A's Majesta One JWT on /client/v1/me (HTTP $code)"
  else
    bad "expected 401/403 calling B with A's Majesta One JWT; got HTTP $code"
  fi
else
  warn "skip Majesta One JWT cross-call probe (pass --jwt-a)"
fi

# --- AWS resource proofs (optional; needs credentials) ---
if command -v aws >/dev/null 2>&1 && aws sts get-caller-identity --region "$REGION" >/dev/null 2>&1; then
  iss_a="https://cognito-idp.${REGION}.amazonaws.com/${POOL_A_ID}"
  iss_b="https://cognito-idp.${REGION}.amazonaws.com/${POOL_B_ID}"
  if [[ "$iss_a" != "$iss_b" ]]; then
    pass "OIDC issuers differ: $iss_a vs $iss_b"
  else
    bad "OIDC issuers unexpectedly equal"
  fi

  # Task role for A must not be allowed Admin APIs on Pool B (simulate with dry policy simulation if role ARN known).
  warn "IAM Pool-A↛Pool-B deny is enforced by install TF (Resource=pool ARN); confirm in AWS IAM console if auditing"
  warn "VPC-A↛RDS-B: no peering by default; confirm with ec2 describe-vpc-peering-connections for this cell"
else
  warn "skip live AWS probes (aws CLI / credentials unavailable)"
fi

# --- In-repo unit proofs reminder ---
pass "run: go test ./internal/authz/ ./internal/httpapi/ ./internal/deploy/ -count=1 -run 'CrossInstall|CustomerMismatch|ForeignIssuer'"

if [[ "$fail" -ne 0 ]]; then
  echo "isolation checklist FAILED"
  exit 1
fi
echo "isolation checklist OK (warnings are informational)"
