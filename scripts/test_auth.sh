#!/usr/bin/env bash
# test_auth.sh — smoke-test the Auth middleware layer against a running server.
# Requires the server to be listening on localhost:8080.
#
# Usage:
#   ./scripts/test_auth.sh

set -euo pipefail

BASE="http://localhost:8080/api/v1"
PASS=0
FAIL=0

check() {
  local desc="$1"
  local expected="$2"
  local actual="$3"

  if [[ "$actual" == "$expected" ]]; then
    echo "  PASS  $desc (HTTP $actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL  $desc — expected HTTP $expected, got $actual"
    FAIL=$((FAIL + 1))
  fi
}

echo "=== Auth middleware smoke tests ==="
echo

# ── dev-admin GET /sites → 200 ────────────────────────────────────────────────
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -H "Authorization: Bearer dev-admin" \
  "$BASE/sites")
check "dev-admin  GET  /api/v1/sites  → 200" "200" "$STATUS"

# ── dev-visitor DELETE /sites/<uuid> → 403 ────────────────────────────────────
FAKE_ID="00000000-0000-0000-0000-000000000001"
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X DELETE \
  -H "Authorization: Bearer dev-visitor" \
  "$BASE/sites/$FAKE_ID")
check "dev-visitor DELETE /api/v1/sites/:id → 403" "403" "$STATUS"

# ── dev-workcrew POST /sites → 403 ────────────────────────────────────────────
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  -X POST \
  -H "Authorization: Bearer dev-workcrew" \
  -H "Content-Type: application/json" \
  -d '{}' \
  "$BASE/sites")
check "dev-workcrew POST /api/v1/sites    → 403" "403" "$STATUS"

# ── no token GET /sites → 401 ─────────────────────────────────────────────────
STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
  "$BASE/sites")
check "no token   GET  /api/v1/sites  → 401" "401" "$STATUS"

echo
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]]
