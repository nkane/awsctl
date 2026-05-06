#!/usr/bin/env bash
# Seeds LocalStack with a few demo Lambda functions + DynamoDB tables so the
# TUI has something to render.
#
# Usage: ./scripts/seed-localstack.sh
set -euo pipefail

ENDPOINT="${AWSCTL_ENDPOINT_URL:-http://localhost:4566}"
export AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-test}"
export AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-test}"
export AWS_REGION="${AWS_REGION:-us-east-1}"

aws() { command aws --endpoint-url "$ENDPOINT" "$@"; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# --- Lambda fixtures ---------------------------------------------------------

cat >"$tmp/handler.py" <<'PY'
def handler(event, context):
    return {"echo": event}
PY
(cd "$tmp" && zip -q fn.zip handler.py)

create_fn() {
  local name="$1" desc="$2"
  if aws lambda get-function --function-name "$name" >/dev/null 2>&1; then
    echo "fn $name exists, skipping"
    return
  fi
  aws lambda create-function \
    --function-name "$name" \
    --runtime python3.12 \
    --handler handler.handler \
    --role arn:aws:iam::000000000000:role/lambda-role \
    --zip-file "fileb://$tmp/fn.zip" \
    --timeout 10 \
    --memory-size 128 \
    --description "$desc" \
    >/dev/null
  echo "fn $name created"
}

create_fn demo-hello       "demo: hello world"
create_fn demo-orders-api  "demo: orders API"
create_fn demo-image-proc  "demo: image processor"

# --- DynamoDB fixtures -------------------------------------------------------

create_table() {
  local name="$1"
  if aws dynamodb describe-table --table-name "$name" >/dev/null 2>&1; then
    echo "table $name exists, skipping"
    return
  fi
  aws dynamodb create-table \
    --table-name "$name" \
    --billing-mode PAY_PER_REQUEST \
    --attribute-definitions AttributeName=pk,AttributeType=S AttributeName=sk,AttributeType=S \
    --key-schema AttributeName=pk,KeyType=HASH AttributeName=sk,KeyType=RANGE \
    >/dev/null
  echo "table $name created"
}

create_table demo-users
create_table demo-orders

aws dynamodb put-item --table-name demo-users \
  --item '{"pk":{"S":"user#1"},"sk":{"S":"profile"},"name":{"S":"alice"}}' >/dev/null
aws dynamodb put-item --table-name demo-users \
  --item '{"pk":{"S":"user#2"},"sk":{"S":"profile"},"name":{"S":"bob"}}' >/dev/null

echo "done."
