#!/usr/bin/env bash
# Publish Cognito User Pool utilization for a managed cell.
# Intended for EventBridge Scheduler / cron in the vendor regional account.
#
# Usage:
#   CELL_ID=us-east-1-a COGNITO_USER_POOLS_QUOTA=1000 ./publish-cognito-pool-utilization.sh
#
# Metric: Majesta One/Managed CognitoUserPoolUtilizationPercent {CellId, Region}

set -euo pipefail

REGION="${AWS_REGION:-${AWS_DEFAULT_REGION:-us-east-1}}"
CELL_ID="${CELL_ID:?CELL_ID is required}"
QUOTA="${COGNITO_USER_POOLS_QUOTA:-1000}"

if ! [[ "$QUOTA" =~ ^[0-9]+$ ]] || [[ "$QUOTA" -le 0 ]]; then
  echo "COGNITO_USER_POOLS_QUOTA must be a positive integer" >&2
  exit 1
fi

count=0
token=""
while :; do
  if [[ -n "$token" ]]; then
    resp="$(aws cognito-idp list-user-pools --max-results 60 --region "$REGION" --next-token "$token" --output json)"
  else
    resp="$(aws cognito-idp list-user-pools --max-results 60 --region "$REGION" --output json)"
  fi
  batch="$(echo "$resp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(len(d.get("UserPools") or []))')"
  count=$((count + batch))
  token="$(echo "$resp" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d.get("NextToken") or "")')"
  [[ -z "$token" ]] && break
done

pct="$(python3 -c "print(round(($count / float($QUOTA)) * 100.0, 2))")"

aws cloudwatch put-metric-data \
  --region "$REGION" \
  --namespace Majesta One/Managed \
  --metric-data "[
    {
      \"MetricName\": \"CognitoUserPoolUtilizationPercent\",
      \"Dimensions\": [
        {\"Name\": \"CellId\", \"Value\": \"$CELL_ID\"},
        {\"Name\": \"Region\", \"Value\": \"$REGION\"}
      ],
      \"Value\": $pct,
      \"Unit\": \"Percent\"
    },
    {
      \"MetricName\": \"CognitoUserPoolCount\",
      \"Dimensions\": [
        {\"Name\": \"CellId\", \"Value\": \"$CELL_ID\"},
        {\"Name\": \"Region\", \"Value\": \"$REGION\"}
      ],
      \"Value\": $count,
      \"Unit\": \"Count\"
    }
  ]"

echo "cell=$CELL_ID region=$REGION pools=$count quota=$QUOTA utilization=${pct}%"

# Exit non-zero at block threshold so cron/CI can gate signups.
python3 -c "import sys; sys.exit(0 if $pct < 85 else 2)"
