#!/usr/bin/env bash
# mqtt_add_edge.sh — Register a new Talos Edge device with Mosquitto Dynamic Security.
#
# This script is the Dynamic Security counterpart to mqtt_setup.sh.
# It creates a Mosquitto dynsec client and adds it to the "edge-devices" group,
# which already carries the "edge" role with the correct ACLs.
#
# The old approach (passwordfile + aclfile) conflicts with the Dynamic Security
# Plugin — only one auth backend should be active at a time.
#
# Usage:
#   ./mqtt_add_edge.sh <edge_id> <password> [--hup|--restart]
#
# Example:
#   ./mqtt_add_edge.sh site01-edge01 s3cr3tPass
#   ./mqtt_add_edge.sh site01-edge01 s3cr3tPass --hup
#
set -euo pipefail

# ── Config ───────────────────────────────────────────────────────────────────
MOSQ_CONTAINER="mosquitto"
CAFILE="/mosquitto/certs/ca.crt"
HOST="localhost"
PORT="8883"

# Admin credentials — read from env so they are never hardcoded in the script.
ADMIN_USER="${MQTT_ADMIN_USER:-admin}"
ADMIN_PASS="${MQTT_ADMIN_PASS:?MQTT_ADMIN_PASS env var is required}"

MODE="restart"

# ── Argument parsing ─────────────────────────────────────────────────────────
usage() {
  echo ""
  echo "Usage: $0 <edge_id> <password> [--hup|--restart]"
  echo ""
  echo "Environment variables:"
  echo "  MQTT_ADMIN_USER  Mosquitto admin username (default: admin)"
  echo "  MQTT_ADMIN_PASS  Mosquitto admin password (required)"
  echo ""
  echo "Examples:"
  echo "  MQTT_ADMIN_PASS=secret $0 site01-edge01 edgePass"
  echo "  MQTT_ADMIN_PASS=secret $0 site01-edge01 edgePass --hup"
  echo ""
  exit 1
}

[[ $# -lt 2 ]] && usage

EDGE_ID="$1"
EDGE_PASS="$2"
shift 2

[[ "${1:-}" == "--hup" ]]     && MODE="hup"
[[ "${1:-}" == "--restart" ]] && MODE="restart"

# ── Validation ───────────────────────────────────────────────────────────────
if [[ ! "$EDGE_ID" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "❌ Invalid edge_id — only A-Z a-z 0-9 . _ - are allowed"
  exit 2
fi

# ── Shared dynsec command prefix ─────────────────────────────────────────────
CMD() {
  docker exec "$MOSQ_CONTAINER" mosquitto_ctrl \
    --cafile "$CAFILE" --insecure \
    -h "$HOST" -p "$PORT" \
    -u "$ADMIN_USER" -P "$ADMIN_PASS" \
    dynsec "$@"
}

echo "=============================="
echo " Registering Edge device"
echo " edge_id : $EDGE_ID"
echo " mode    : $MODE"
echo "=============================="

# ── Create client ────────────────────────────────────────────────────────────
# If the client already exists dynsec returns an error; we treat that as
# a no-op so the script is safe to re-run (idempotent).
if CMD createClient "$EDGE_ID" -p "$EDGE_PASS"; then
  echo "✅ Client created: $EDGE_ID"
else
  echo "ℹ️  Client already exists, updating password..."
  CMD setClientPassword "$EDGE_ID" "$EDGE_PASS"
fi

# ── Add to edge-devices group (inherits the "edge" role + ACLs) ──────────────
if CMD addGroupClient edge-devices "$EDGE_ID"; then
  echo "✅ Added $EDGE_ID to group: edge-devices"
else
  echo "ℹ️  $EDGE_ID is already a member of edge-devices"
fi

# ── Reload broker ────────────────────────────────────────────────────────────
# Dynamic Security changes take effect immediately without a reload,
# but a restart/HUP may be needed for other config changes.
if [[ "$MODE" == "hup" ]]; then
  echo "🔄 Reloading broker config (SIGHUP)"
  docker kill -s HUP "$MOSQ_CONTAINER"
elif [[ "$MODE" == "restart" ]]; then
  echo "🔄 Restarting broker"
  docker restart "$MOSQ_CONTAINER" > /dev/null
fi

echo ""
echo "🎉 Done — $EDGE_ID is ready to connect"
echo ""
echo "Allowed publish topics:"
echo "  talos/$EDGE_ID/telemetry"
echo "  talos/$EDGE_ID/status"
echo "  talos/$EDGE_ID/event"
echo "  talos/$EDGE_ID/response"
echo ""
echo "Allowed subscribe topics:"
echo "  talos/$EDGE_ID/command"
echo "  talos/$EDGE_ID/config"
echo "  talos/$EDGE_ID/ota"
echo "  orion/broadcast/#"
echo ""