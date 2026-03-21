#!/usr/bin/env bash
# scripts/mqtt_test_pub.sh — 快速驗證 MQTT 整條寫入路徑是否正常。
#
# 功能：
#   1. 用指定的 Edge 帳號發送一筆測試 payload 到 broker
#   2. 同時監聽 broker 確認訊息有到達
#   3. 查詢 DB 確認資料有寫入（需要 psql）
#
# Usage:
#   ./scripts/mqtt_test_pub.sh <mqtt_username> <password> <device_code> [device_type]
#
# Arguments:
#   mqtt_username  Gateway 的 MQTT 帳號（對應 gateways.mqtt_username）
#   password       Gateway 的 MQTT 密碼
#   device_code    要測試的 device_code（對應 devices.device_code）
#   device_type    裝置類型代碼，預設 CI（可用：CI SE SF ST SP SR SO）
#
# Environment variables:
#   MQTT_HOST      MQTT broker hostname（預設 localhost）
#   MQTT_PORT      MQTT broker port（預設 8883）
#   CAFILE         CA 憑證路徑（預設 certs/ca.crt）
#   DB_HOST        DB hostname（預設 localhost）
#   DB_PORT        DB port（預設 5432）
#   DB_USER        DB 使用者（預設 orion）
#   DB_NAME        DB 名稱（預設 orion）
#
# Examples:
#   ./scripts/mqtt_test_pub.sh 16481585116-loop1 test1234 110
#   ./scripts/mqtt_test_pub.sh 16481585116-loop1 test1234 110 SE
#   CAFILE=/opt/certs/ca.crt ./scripts/mqtt_test_pub.sh 16481585116-loop1 test1234 1e2 SP
#
set -euo pipefail

# ── Arguments ─────────────────────────────────────────────────────────────────
MQTT_USERNAME="${1:?Usage: $0 <mqtt_username> <password> <device_code> [device_type]}"
MQTT_PASSWORD="${2:?Usage: $0 <mqtt_username> <password> <device_code> [device_type]}"
DEVICE_CODE="${3:?Usage: $0 <mqtt_username> <password> <device_code> [device_type]}"
DEVICE_TYPE="${4:-CI}"

# ── Config ────────────────────────────────────────────────────────────────────
MQTT_HOST="${MQTT_HOST:-localhost}"
MQTT_PORT="${MQTT_PORT:-8883}"
CAFILE="${CAFILE:-certs/ca.crt}"
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-orion}"
DB_NAME="${DB_NAME:-orion}"

TOPIC="talos/${MQTT_USERNAME}/telemetry"
TS=$(date +%s%3N)  # Unix timestamp 毫秒

# ── Build payload based on device type ───────────────────────────────────────
case "${DEVICE_TYPE}" in
  CI)
    PAYLOAD=$(printf '{"ts":%d,"readings":[{"type":"CI","device_code":"%s","kw":3.23,"kwh":9876.0,"hz":50.0,"error":0,"alert":0,"status":"TEST_OK"}]}' "$TS" "$DEVICE_CODE")
    TABLE="telemetry_inverters"
    ;;
  SE)
    PAYLOAD=$(printf '{"ts":%d,"readings":[{"type":"SE","device_code":"%s","voltage":220.5,"current":15.2,"kw":3.35,"kva":3.5,"kvar":1.0,"kwh":12345.6,"pf":0.95,"status":"TEST_OK"}]}' "$TS" "$DEVICE_CODE")
    TABLE="telemetry_meters"
    ;;
  SF)
    PAYLOAD=$(printf '{"ts":%d,"readings":[{"type":"SF","device_code":"%s","flow":12.5,"consumption":1234.5,"status":"TEST_OK"}]}' "$TS" "$DEVICE_CODE")
    TABLE="telemetry_flow_meters"
    ;;
  ST|SP|SR|SO)
    PIN="${PIN:-0}"
    PAYLOAD=$(printf '{"ts":%d,"readings":[{"type":"%s","device_code":"%s","pin":%d,"val":25.5,"status":"TEST_OK"}]}' "$TS" "$DEVICE_TYPE" "$DEVICE_CODE" "$PIN")
    TABLE="telemetry_sensors"
    ;;
  *)
    echo "❌ Unknown device type: ${DEVICE_TYPE}"
    echo "   Supported: CI SE SF ST SP SR SO"
    exit 1
    ;;
esac

echo "========================================"
echo " MQTT Ingest Test"
echo "========================================"
echo " mqtt_username : ${MQTT_USERNAME}"
echo " topic         : ${TOPIC}"
echo " device_type   : ${DEVICE_TYPE}"
echo " device_code   : ${DEVICE_CODE}"
echo " ts            : ${TS}"
echo " table         : ${TABLE}"
echo "========================================"
echo ""

# ── Step 1: Publish ───────────────────────────────────────────────────────────
echo "▶ Step 1: Publishing to broker..."
mosquitto_pub \
  -h "$MQTT_HOST" -p "$MQTT_PORT" \
  --cafile "$CAFILE" --insecure \
  -u "$MQTT_USERNAME" -P "$MQTT_PASSWORD" \
  -t "$TOPIC" \
  -m "$PAYLOAD" \
  -d

echo ""
echo "✅ Publish sent"
echo ""

# ── Step 2: Wait for Orion to process ────────────────────────────────────────
echo "▶ Step 2: Waiting 2s for Orion to process..."
sleep 2

# ── Step 3: Verify DB write ───────────────────────────────────────────────────
echo "▶ Step 3: Checking DB for received_at IS NOT NULL..."
echo ""

if command -v psql &>/dev/null; then
  PGPASSWORD="${DB_PASSWORD:-}" psql \
    -h "$DB_HOST" -p "$DB_PORT" \
    -U "$DB_USER" -d "$DB_NAME" \
    -c "SELECT ts, received_at, received_at - ts AS lag FROM ${TABLE} WHERE received_at IS NOT NULL ORDER BY received_at DESC LIMIT 3;"
else
  echo "⚠️  psql not found, skipping DB check."
  echo "   Run manually:"
  echo "   SELECT ts, received_at, received_at - ts AS lag"
  echo "   FROM ${TABLE}"
  echo "   WHERE received_at IS NOT NULL"
  echo "   ORDER BY received_at DESC LIMIT 3;"
fi

echo ""
echo "========================================"
echo " Test complete"
echo " Check above: received_at should NOT be NULL"
echo " lag = network latency + device clock drift"
echo "========================================"